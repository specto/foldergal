import logging
import os
import foldergal
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
app.static('/gallery/static', './src/static', name='static')
# app.static('/static', './src/static', name='static')
app.static('/gallery/favicon.ico', './src/static/favicon.ico', name='favicon')
# app.static('/favicon.ico', './src/static/favicon.ico', name='favicon')

# Have thumbnails served from folder
app.static('/gallery/thumbs', app.config['FOLDER_CACHE'], name='thumbnails')

# Initialize our core module and start periodic refresh
foldergal.configure(app.config)
app.add_task(foldergal.refresh())


def render(template, **kwargs):
    """ Template render helper """
    template = jinja_env.get_template(template)
    return template.render(
        url_for=app.url_for,
        www_prefix=app.config['WWW_PREFIX'],
        **kwargs
    )


@app.route("/gallery/rss")
@app.route("/gallery/atom")
async def rss(_):
    return response.text('waaaaa')

# This must be the last route
@app.route("/gallery")
@app.route("/gallery/<path:path>")
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
async def on_shutdown(app, loop):
    print('shutting down...')
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
    app.run(
        host=app.config["HOST"],
        port=app.config["PORT"],
        debug=app.config["DEBUG"],
        access_log=app.config["DEBUG"],
        workers=app.config["WORKERS"],
        auto_reload=False,
        log_config=logging.basicConfig(
            filename=app.config["FOLDER_LOG"] + '/foldergal.log',
            level=logging.DEBUG if app.config['DEBUG'] else logging.INFO
        ),
    )
