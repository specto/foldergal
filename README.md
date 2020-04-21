Web Gallery from Folders
===

A web server that displays folders of images.

Features
---

* Cache file listing
* Periodic file listing refresh
* Use file system owners as authors
* Read and use image metadata
* Provide search and filering


Running
---

Use environment variables if you like:
```
export FOLDERGAL_PORT=8080
export FOLDERGAL_HOST=localhost
export FOLDERGAL_HOME=/path/to/settings/
export FOLDERGAL_ROOT=/files/to/serve/
export FOLDERGAL_PREFIX=gallery
export FOLDERGAL_CRT=/tls/server.crt
export FOLDERGAL_KEY=/tls/server.key
export FOLDERGAL_HTTP2=true
```

Also, command line parameters are available.
Run `./foldergal --help` for more info.

Home folder (defaults to the executable location) structure:
```
 |- foldergal.log
 |+ cache
    |-... generated thumbnails
 |+ tls
    |- server.crt [optional]
    |- server.key [optional]
```

TODO
---

* [x] Enable support for http2 (with tls) and fall back to http1
* [ ] Embed favicon and serve it

