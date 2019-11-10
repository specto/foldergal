import foldergal
import os
import argparse
from signal import signal, SIGINT
import logging
import asyncio
import uvloop
import requests
import json
from urllib.parse import quote
from sanic import Sanic, response
from sanic.log import logger
from sanic.exceptions import NotFound
from jinja2 import Environment, PackageLoader, select_autoescape

# Initialize framework for our app and load config from file
app = Sanic(__name__, strict_slashes=False)
app.config.from_pyfile(
    os.path.join(os.path.dirname(__file__),
                 "../foldergal.cfg"))

# Setup template engine
jinja_env = Environment(
    loader=PackageLoader('www', 'templates'),
    autoescape=select_autoescape(['html'])
)

# Have static files served from folder
app.static(app.config['WWW_PREFIX'] + '/static', './src/static')
app.static(app.config['WWW_PREFIX'] + '/favicon.ico', './src/static/favicon.ico', name='favicon')

# Have thumbnails served from folder
app.static(app.config['WWW_PREFIX'] + '/thumbs', app.config['FOLDER_CACHE'], name='thumbs')


def render(template, *args, **kwargs):
    """ Template render helper """
    template = jinja_env.get_template(template)
    return template.render(
        *args, url_for=app.url_for, version=app.config['VERSION'], **kwargs)


@app.route(app.config['WWW_PREFIX'] + "/rss")
@app.route(app.config['WWW_PREFIX'] + "/atom")
async def rss(_):
    return response.text(json.dumps(
        await foldergal.get_file_list(3),
        sort_keys=True, indent=2, default=str))


# These routes must be the last and in this order
@app.route(app.config['WWW_PREFIX'] + "/<path:path>")
@app.route(app.config['WWW_PREFIX'] + "/")
async def index(req, path=''):
    path = './' + path
    order_by = req.args.get('order_by')
    desc = req.args.get('desc', '0') in ['1', 'true', 'yes']
    try:
        items = await foldergal.get_folder_items(path, order_by, desc)
    except NotADirectoryError:
        # the path leads to a file
        return await response.file_stream(await foldergal.get_file(path))
    # we are looking at a folder
    return response.html(render(
        'list.html',
        show_authors=True if app.config.get('AUTHORS', {}) else False,
        items=items,
        parent=await foldergal.get_parent(path),
        title=await foldergal.get_current(path),
        crumbs=await foldergal.get_breadcrumbs(path),
        order_by=order_by,
    ))

if app.config['WWW_PREFIX'] and app.config['WWW_PREFIX'] != '':
    @app.route('/')
    async def gohome(_):
        return response.redirect(app.config['WWW_PREFIX'])


@app.listener("before_server_stop")
async def on_shutdown(*_):
    logger.info('Shutting down...')
    logging.shutdown()


# Display some error message when things break
async def server_error_handler(_, exception):
    if isinstance(exception, NotFound):
        return response.html(render('error.html', title="Not found",
                                    message=exception), status=404)
    # It might be serious
    logger.error(exception, exc_info=app.config['DEBUG'])
    return response.html(
        render('error.html',
               heading="An error has occurred",
               message="Check the logs for clues how to fix it."),
        status=500
    )

app.error_handler.add(Exception, server_error_handler)


async def refresh():
    while True:
        discord_url = app.config.get('DISCORD_WEBHOOK')
        updates = await foldergal.scan_for_updates()
        if updates and discord_url:
            try:
                file = await foldergal.get_file(updates)
                if not file.is_dir():
                    file = await foldergal.get_parent(file)
                url = app.url_for('index',
                                  path=quote(foldergal.normalize_path(file)),
                                  _external=True)
                req = requests.post(
                    discord_url,
                    json={
                        'content': 'New item posted at: ' + url,
                        'username': 'gallery',
                    }
                )
                req.raise_for_status()
            except Exception as err:
                logger.error(err, exc_info=app.config['DEBUG'])

        await asyncio.sleep(app.config['RESCAN_SECONDS'])


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description='Show a folder on the web')
    parser.add_argument('folder_root', nargs='?',
        default='',
        help="folder to make visible over http")
    args = parser.parse_args()
    if args.folder_root:
        app.config['FOLDER_ROOT'] = args.folder_root

    logger.info(f'Starting @ {app.config.get("SERVER_NAME", "UNSPECIFIED SERVER")} '
                f'v{app.config["VERSION"]}')
    asyncio.set_event_loop(uvloop.new_event_loop())
    serv_coro = app.create_server(
        host=app.config["HOST"],
        port=app.config["PORT"],
        debug=app.config["DEBUG"],
        access_log=app.config["DEBUG"],
        return_asyncio_server=True,
    )
    loop = asyncio.get_event_loop()
    serv_task = asyncio.ensure_future(serv_coro, loop=loop)
    signal(SIGINT, lambda s, f: loop.stop())
    server = loop.run_until_complete(serv_task)
    server.after_start()

    # Initialize our core module and start periodic refresh
    foldergal.configure(app.config)
    refresh_task = loop.create_task(refresh())

    try:
        loop.run_forever()
    except KeyboardInterrupt as e:
        loop.stop()
    finally:
        server.before_stop()

        # Stop the periodic refresh
        refresh_task.cancel()

        # Wait for server to close
        close_task = server.close()
        loop.run_until_complete(close_task)

        # Complete all tasks on the loop
        for connection in server.connections:
            connection.close_if_idle()
        server.after_stop()
