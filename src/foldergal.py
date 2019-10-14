import asyncio
from sanic.exceptions import ServerError
from sanic.log import logger
from pathlib import Path

CONFIG = {}
files = {}

def configure(config):
    global CONFIG
    CONFIG['FOLDER_ROOT'] = config['FOLDER_ROOT']
    CONFIG['RESCAN_SECONDS'] = config['RESCAN_SECONDS']
    CONFIG['TARGET_EXT'] = config['TARGET_EXT']

def normalize_path(p):
    return './' + p.relative_to(CONFIG['FOLDER_ROOT']).as_posix()


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


async def get_file_by_id(id):
    if not CONFIG:
        raise ServerError("Call foldergal.configure")


async def get_folder_contents(target=None, sub_items = None):
    if target == './':
        return 'folder', files.keys()
    current_folder = sub_items if sub_items else files
    for name, item in current_folder.items():
        if name == target:
            if isinstance(item, dict):
                return 'folder', item
            return 'image', name
        elif isinstance(item, dict):
            sub_item = await get_folder_contents(target, item)
            if sub_item:
                return sub_item
