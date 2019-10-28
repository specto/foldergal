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

Deployment
---

Running behind nginx
```
    location /gallery {
      proxy_pass http://127.0.0.1:5000/gallery;
      proxy_set_header Host $host;
    }
```
