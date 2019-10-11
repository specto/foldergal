import asyncio

CONFIG = {}

def configure(config):
    global CONFIG
    CONFIG['FOLDER_ROOT'] = config['FOLDER_ROOT']
    CONFIG['RESCAN_SECONDS'] = config['RESCAN_SECONDS']


async def refresh():
    if not CONFIG:
        raise UnboundLocalError("Call foldergal.configure")
    while True:
        print ('Refreshing')
        await asyncio.sleep(CONFIG['RESCAN_SECONDS'])
