Web Gallery from Folders
===

A web server that displays folders of images.

Features
---

* Cache file listing
* Periodic file listing refresh
* Use file system owners as authors
* Read and use image metadata
* Provide search and filtering

TODO
---

+ [x] Generate thumbnails and display them
+ [x] Cache thumbnails to disk in app folder
+ [x] Fix thumbnail rotation according to exif tag
+ [ ] Provide RSS/atom feed
+ [ ] Notify discord (webhook?, direct to bot?)
+ [ ] Show author, dates for each item
+ [ ] Attempt to redirect to 'archive' folder for non-existing items
+ [ ] Security: don't go above configured folder
+ [ ] Choose appropriate license
+ [ ] Expose filter by author
+ [ ] Expose sort by date, name

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
