import asyncio
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
app.static('/static', './src/static')

# Initialize our core module and start periodic refresh
foldergal.configure(app.config)
app.add_task(foldergal.refresh())


def render(template, **kwargs):
    """ Template render helper """
    template = jinja_env.get_template(template)
    return template.render(url_for=app.url_for, **kwargs)


@app.route('/test')
async def view_slash(req):
    return response.html(render('view.html', body="test wazaaaaa"))


@app.route("/get/<file>")
async def get(req, file):
    file_data = await foldergal.get_file_by_id(file)
    if file_data:
        return await response.file_stream(file_data)
    return response.html(
        render('error.html', message=f'File "{file}" was not found'),
        status=404
    )

# This must be the last route
@app.route("/")
@app.route("/<path:path>")
async def index(req, path=''):
    path = './' + path
    logger.debug(path)
    try:
        ftype, content = (await foldergal.get_folder_contents(path))
    except Exception as e:
        logger.debug(e)
        return response.html(
            render('error.html', message=f'"{path}" was not found'),
            status=404
        )
    if ftype == 'image':
        # this is path to file
        return await response.file_stream(app.config['FOLDER_ROOT'] + content)
    return response.html(render('list.html', folders=[c[1:] for c in content]))


# Display some error message when things break
async def server_error_handler(request, exception):
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
    )
