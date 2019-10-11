import asyncio
import os
import foldergal
from sanic import Sanic, response
from sanic.log import logger

from jinja2 import Environment, PackageLoader, select_autoescape

# Initialize framework for our app and load config from file
app = Sanic(__name__)
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

# Initialize our core module and start periodic refresh
foldergal.configure(app.config)
refresh_task = foldergal.refresh()
app.add_task(refresh_task)


def render(template, **kwargs):
    """ Jinja render helper """
    template = jinja_env.get_template(template)
    return template.render(url_for=app.url_for, **kwargs)


@app.route("/")
async def index(req):
    return response.html(render('list.html', message="Welcome"))


@app.listener('before_server_stop')
async def notify_server_stopping(application, loop):
    logger.info(f'Stopping server v{application.config["VERSION"]}')


if __name__ == "__main__":
    logger.info(f'Starting server v{app.config["VERSION"]}')
    app.run(
        host=app.config["HOST"],
        port=app.config["PORT"],
        debug=app.config["DEBUG"],
        access_log=app.config["DEBUG"],
        workers=app.config["WORKERS"],
    )
