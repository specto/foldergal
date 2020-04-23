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


Usage
---

TLDR;
1. Run in terminal something like
   `./foldergal-binary --root /path/to/serve/files/from --home /tmp/foldergal`
2. Visit http://localhost:8080

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
export FOLDERGAL_CACHE_MINUTES=60
```

Http2 works only with TLS, which works only if you provide certificate files.

Run `./foldergal --help` for full info on command line parameters.

Home folder structure:
```
home (defaults to current folder)
 ╿
 ├─ foldergal.log
 ├╼ cache (created automatically)
 │  └─ ...generated thumbnails
 └╼ tls [optional]
    ├─ server.crt [optional]
    └─ server.key [optional]
```

TODO
---

* [x] Enable support for http2 (with tls) and fall back to http1
* [x] Embed favicon and serve it
* [x] Use and show app version
* [ ] Notifications with discord hook
* [ ] Call ffmpeg binary to generate video thumbnail

