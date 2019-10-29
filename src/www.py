import foldergal
import os
from signal import signal, SIGINT
import logging
import asyncio
import uvloop
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
app.static('/static', './src/static')
app.static('/favicon.ico', './src/static/favicon.ico', name='favicon')

# Have thumbnails served from folder
app.static('/thumbs', app.config['FOLDER_CACHE'], name='thumbs')


def prefixed_url_for(*args, **kwargs):
    url = app.url_for(*args, **kwargs)
    prefix = app.config['WWW_PREFIX']
    if prefix and url.startswith('/'):
        return prefix + url
    return url


def render(template, *args, **kwargs):
    """ Template render helper """

    template = jinja_env.get_template(template)
    return template.render(*args, url_for=prefixed_url_for, **kwargs)


@app.route("/rss")
@app.route("/atom")
async def rss(_):
    return response.text('waaaaa')


# These must be the last routes in this order
@app.route("/<path:path>")
@app.route("/")
async def index(req, path=''):
    path = './' + path
    order_by = req.args.get('order_by')
    desc = req.args.get('desc', '0') in ['1', 'true', 'yes']
    try:
        items = await foldergal.get_folder_items(path, order_by, desc)
    except ValueError:
        # this is path to a file
        return await response.file_stream(await foldergal.get_file(path))
    # we are looking at a folder
    return response.html(render(
        'list.html',
        items=items,
        parent=await foldergal.get_parent(path),
        heading=path,
        crumbs=foldergal.get_breadcrumbs(path),
        order_by=order_by
    ))


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
    logger.error(exception)
    return response.html(
        render('error.html',
               heading="An error has occurred",
               message="Check the logs for clues how to fix it."),
        status=500
    )

app.error_handler.add(Exception, server_error_handler)

if __name__ == "__main__":
    logger.info(f'Starting server v{app.config["VERSION"]}')
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
    refresh_task = loop.create_task(foldergal.refresh())

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
