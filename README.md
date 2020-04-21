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
export FOLDERGAL_HOME=/path/to/store/stuff
```

Also command line parameters are available.
Run `./foldergal --help` for more info.
