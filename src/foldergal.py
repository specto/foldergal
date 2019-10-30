import asyncio
from datetime import datetime
from typing import NamedTuple, Sequence

from sanic.exceptions import ServerError
from sanic.log import logger
from pathlib import Path
from PIL import Image, ExifTags
from natsort import natsorted

CONFIG = {}
files = {}
authors = {
    1001: {'name': 'vivok', 'uid': 1001},
    1002: {'name': 'tihman', 'uid': 1002},
    1003: {'name': 'pes', 'uid': 1003},
}

def configure(config):
    global CONFIG
    CONFIG['FOLDER_ROOT'] = config['FOLDER_ROOT']
    CONFIG['RESCAN_SECONDS'] = config['RESCAN_SECONDS']
    CONFIG['TARGET_EXT'] = config['TARGET_EXT']
    CONFIG['FOLDER_CACHE'] = config['FOLDER_CACHE']


def normalize_path(p):
    return './' + p.relative_to(CONFIG['FOLDER_ROOT']).as_posix()


class FolderItem(NamedTuple):
    name: str = ''
    type: str = ''
    author: str = ''
    thumb: str = ''
    parent: str = ''
    cdate: datetime = datetime.now()
    mdate: datetime = datetime.now()

    @property
    def path(self) -> str:
        return Path(self.parent).joinpath(self.name)

    def __repr__(self) -> str:
        return f'<{self.type.upper()} {self.parent}/{self.name}>'


async def scan(folder):
    contents = {}
    root = Path(folder)
    if not root.exists():
        logger.error(f'Folder not found "{root}"')
        return contents

    for child in root.iterdir():
        if child.is_dir():
            sub_contents = await scan(child)
            if sub_contents:
                contents[normalize_path(child)] = sub_contents
        elif child.suffix.lower() in CONFIG['TARGET_EXT']:
            contents[normalize_path(child)] = child.name
    return contents


async def refresh():
    global files
    if not CONFIG:
        raise ServerError("Call foldergal.configure")
    while True:
        logger.debug('Refreshing...')
        files = await scan(CONFIG['FOLDER_ROOT'])
        await asyncio.sleep(CONFIG['RESCAN_SECONDS'])


THUMB_SIZE = (512, 512)


def path_to_id(path):
    return str(path).replace('/', '_')


def generate_thumb(path, mtime):
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
            exif = {ExifTags.TAGS.get(id, id): tag for id, tag in im.getexif().items()}
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
                author=authors.get(stat.st_uid, {'uid': stat.st_uid}),
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


def get_breadcrumbs(path):
    current = Path(path)
    if current.parent == current.root:
        return None
    return [('/', '/')] + [(str(p[0]) + '/' + p[1], p[1])
                           for p in zip(reversed(current.parent.parents),
                                        current.parent.parts)]


async def get_folder_tree(target=None, sub_items=None):
    if target == './':
        return 'folder', files.keys()
    current_folder = sub_items if sub_items else files
    for name, item in current_folder.items():
        if target and name == target:
            if isinstance(item, dict):
                return 'folder', item, {}
            return 'image', name, {}
        elif isinstance(item, dict):
            deep_item = await get_folder_tree(target, item)
            if deep_item:
                return deep_item
