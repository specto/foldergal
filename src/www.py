from flask import Flask, escape, request
from flask import send_from_directory, render_template
from pathlib import Path
from logging.config import dictConfig
from flask.logging import default_handler


dictConfig({
    'version': 1,
    'formatters': {'default': {
        'format': '[%(asctime)s] %(levelname)s in %(module)s: %(message)s',
    }},
    'handlers': {'wsgi': {
        'class': 'logging.StreamHandler',
        'stream': 'ext://flask.logging.wsgi_errors_stream',
        'formatter': 'default'
    }},
    'root': {
        'level': 'INFO',
        'handlers': ['wsgi']
    }
})

app = Flask(__name__)
app.logger.removeHandler(default_handler)

@app.route('/')
def hello():
    name = request.args.get("name", "World")
    return render_template("list.html", message=f'Hello, {escape(name)}!');

app.logger.info('Server started')
