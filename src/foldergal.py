from datetime import datetime
from typing import NamedTuple, Sequence, Mapping, Union, Iterator

from sanic.exceptions import ServerError
from sanic.log import logger
from pathlib import Path
from PIL import Image, ExifTags
from natsort import natsorted

CONFIG = {}
FILES = {}
AUTHORS = {}


def configure(config):
    global CONFIG, AUTHORS
    CONFIG['FOLDER_ROOT'] = config['FOLDER_ROOT']
    CONFIG['RESCAN_SECONDS'] = config['RESCAN_SECONDS']
    CONFIG['TARGET_EXT'] = config['TARGET_EXT']
    CONFIG['FOLDER_CACHE'] = config['FOLDER_CACHE']
    AUTHORS = config.get('AUTHORS', {})


def normalize_path(p):
    return './' + p.relative_to(CONFIG['FOLDER_ROOT']).as_posix()


class FolderItem(NamedTuple):
    id: str = ''
    name: str = ''
    type: str = ''
    author: dict = {}
    thumb: str = ''
    parent: str = ''
    full_path: str = ''
    cdate: datetime = datetime.now()
    mdate: datetime = datetime.now()

    @property
    def path(self) -> str:
        return Path(self.parent).joinpath(self.name)

    def __repr__(self) -> str:
        return f'<{self.type.upper()} {self.parent}/{self.name}>'


def get_file_meta(fid: str, file: Path) -> FolderItem:
    stat = file.stat()
    return FolderItem(
        id=fid,
        type='image',
        name=file.name,
        full_path=file.resolve().as_posix(),
        author=AUTHORS.get(stat.st_uid, {'uid': stat.st_uid}),
        cdate=datetime.fromtimestamp(stat.st_ctime),
        mdate=datetime.fromtimestamp(stat.st_mtime),
    )


async def scan(folder: Path) -> dict:
    contents = {}
    for child in folder.iterdir():
        path_key = normalize_path(child)
        if child.is_dir():
            sub_contents = await scan(child)
            if sub_contents:
                contents[path_key] = sub_contents
        elif child.suffix.lower() in CONFIG['TARGET_EXT']:
            contents[path_key] = get_file_meta(path_key, child)
    return contents


def find_first_new(dict_a: dict, dict_b: dict) -> str:
    for k in dict_a.keys():
        if k not in dict_b:
            return k
        elif isinstance(dict_a[k], dict) and isinstance(dict_b[k], dict):
            subdiff = find_first_new(dict_a[k], dict_b[k])
            if subdiff:
                return subdiff


async def scan_for_updates() -> str:
    global FILES
    if not CONFIG:
        raise ServerError("Call foldergal.configure")
    logger.debug('Scanning for updated files...')
    try:
        new_files = await scan(Path(CONFIG['FOLDER_ROOT']))
        diff = find_first_new(new_files, FILES)
    except Exception as e:
        logger.error(e)
        return ''
    result = diff if FILES and diff else ''
    FILES = new_files
    return result


THUMB_SIZE = (512, 512)


def path_to_id(path: Union[Path, str]) -> str:
    return str(path).replace('/', '_')


def generate_thumb(path, mtime) -> str:
    if not CONFIG:
        raise ServerError("Call foldergal.configure")
    filename = path_to_id(path)
    cache_path = Path(CONFIG['FOLDER_CACHE']).resolve()
    thumb_file = cache_path.joinpath(filename).resolve()
    try:  # Security check
        thumb_file.relative_to(cache_path)
    except ValueError as e:
        logger.error(e)
        return 'broken.svg'
    # Check for fresh thumb
    if not thumb_file.exists() or thumb_file.stat().st_mtime < mtime:
        try:
            logger.debug(f'generating thumb for {filename}')
            im = Image.open(path)
            im.thumbnail(THUMB_SIZE, resample=Image.BICUBIC)
            exif = {ExifTags.TAGS.get(i, i): tag for i, tag in im.getexif().items()}
            orientation = exif.get('Orientation', 1)
            if orientation and orientation > 1:
                # Orientation happens to map directly to pillow transpose 'method' values
                # 1 - 000 NORMAL: -
                # 2 - 001 FLIP_HORIZONTAL: flip()
                # 3 - 010 ROTATE_180: rotate(180)
                # 4 - 011 FLIP_VERTICAL: rotate(180) flip()
                # 5 - 100 TRANSPOSE: rotate(90) flip()
                # 6 - 101 ROTATE_90: rotate(90)
                # 7 - 110 TRANSVERSE: rotate(-90) flip()
                # 8 - 111 ROTATE_270: rotate(-90)
                # also see: https://sirv.com/help/resources/rotate-photos-to-be-upright/
                im = im.transpose(method=orientation - 1)
            im.save(thumb_file)
        except Exception as e:
            logger.error(e)
            return 'broken.svg'
    return filename


async def get_folder_items(path, order_by='name', desc=True) -> Sequence[FolderItem]:
    root_path = Path(CONFIG['FOLDER_ROOT']).resolve()
    folder = root_path.joinpath(path).resolve()
    if not folder.exists():
        raise LookupError(f'{path} not found')
    if not folder.is_dir():
        raise ValueError(f'{path} is not a folder')
    try:  # Security check
        folder.relative_to(root_path)
    except ValueError as e:
        logger.error(e)
        return []
    result = []
    for child in folder.iterdir():
        stat = child.stat()
        if child.is_dir():
            result.append(FolderItem(
                type='folder',
                name=child.name,
                parent=path,
                cdate=datetime.fromtimestamp(stat.st_ctime),
                mdate=datetime.fromtimestamp(stat.st_mtime),
            ))
        elif child.suffix.lower() in CONFIG['TARGET_EXT']:
            thumb = generate_thumb(child, stat.st_mtime)
            result.append(FolderItem(
                type='image',
                name=child.name,
                parent=path,
                author=AUTHORS.get(stat.st_uid, {'uid': stat.st_uid}),
                cdate=datetime.fromtimestamp(stat.st_ctime),
                mdate=datetime.fromtimestamp(stat.st_mtime),
                thumb=thumb,
            ))

    if order_by == 'cdate':
        def sorter(i):
            return i.cdate
    elif order_by == 'mdate':
        def sorter(i):
            return i.mdate
    else:  # default sort by name
        def sorter(i):
            return i.name.lower()
    if desc:
        return list(reversed(natsorted(result, key=sorter)))
    return natsorted(result, key=sorter)


async def get_file(path):
    root_path = Path(CONFIG['FOLDER_ROOT']).resolve()
    file = root_path.joinpath(path).resolve()
    try:  # Security check
        file.relative_to(root_path)
    except ValueError as e:
        logger.error(e)
        return ''
    return file


async def get_parent(path='./'):
    return Path(path).parent if path != './' else ''


async def get_current(path='./'):
    return Path(path).name if path != './' else '#:\\>'


async def get_breadcrumbs(path=None):
    if not path or path == './':
        return []
    root_path = Path(CONFIG['FOLDER_ROOT']).resolve()
    location = root_path.joinpath(path).resolve()
    location = location.relative_to(root_path)
    return list(reversed([l for l in location.parents if l.name])) + [location]


async def get_file_tree() -> Sequence[Union[Mapping, Sequence]]:

    def _dict_keys_to_list(d: Mapping):
        return [_dict_keys_to_list(val)
                if isinstance(val, Mapping) else val._asdict()
                for key, val in d.items()]
    return _dict_keys_to_list(FILES)


async def get_file_list(limit=0) -> Sequence[Mapping]:
    rez = []

    def _flatten(seq):
        nonlocal rez
        for i in seq:
            print(len(rez))
            if isinstance(i, Mapping):
                rez += [i]
            else:
                rez += _flatten(i)

    _flatten(await get_file_tree())
    return list(reversed(sorted(rez, key=lambda o: o['cdate'])))


async def get_folder_tree(target=None, sub_items=None):
    if target == './':
        return 'folder', FILES.keys()
    current_folder = sub_items if sub_items else FILES
    for name, item in current_folder.items():
        if target and name == target:
            if isinstance(item, Mapping):
                return 'folder', item, {}
            return 'image', name, {}
        elif isinstance(item, Mapping):
            deep_item = await get_folder_tree(target, item)
            if deep_item:
                return deep_item
