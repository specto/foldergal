import os
import logging
import foldergal
from sanic import Sanic, response
from sanic.log import logger

from jinja2 import Environment, PackageLoader, select_autoescape

# Initialize framework for our app and load config from file
app = Sanic(__name__)
app.config.from_pyfile(
    os.path.join(os.path.dirname(__file__),
    "../foldergal.cfg")
)

# Setup template engine
jinja_env = Environment(
    loader=PackageLoader('www', 'templates'),
    autoescape=select_autoescape(['html'])
)

# Have static files served from folder
app.static('/static', './src/static')

# Add our core module
foldergal.configure(app.config)
app.add_task(foldergal.refresh())

def render(template, **kwargs):
    ''' Jinja render helper '''
    template = jinja_env.get_template(template)
    return template.render(url_for=app.url_for, **kwargs)

@app.route("/")
async def index(req):
    return response.html(render('list.html', message="Welcome"))


if __name__ == "__main__":
    logger.info(f'Starting server v{app.config["VERSION"]}')
    app.run(
        host=app.config["HOST"],
        port=app.config["PORT"],
        debug=app.config["DEBUG"],
        access_log=app.config["DEBUG"],
        workers=app.config["WORKERS"],
    )
