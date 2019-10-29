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

1. Run with internal server:
   `python src/www.py &> log/foldergal.log`
1. [optional] Put behind nginx
   ```
    location /gallery {
      proxy_pass http://127.0.0.1:5000/gallery;
      proxy_set_header Host $host;
    }
   ```
1. Test...
