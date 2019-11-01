Web Gallery from Folders
===

A web server that displays folders of images.
Useful if you want to share for example a network folder 
that you put media files to.


Features
---

* Cache file listing
* Periodic file listing refresh
* Use file system owners as authors, optional
* Read and use image metadata
* Provide search and filtering
* Subfolder path navigation

TODO
---

+ [x] Generate thumbnails and display them
+ [x] Cache thumbnails to disk in app folder
+ [x] Fix thumbnail rotation according to exif tag
+ [x] Security: don't go above configured folder
+ [x] Name sort with natural digit order
+ [x] Notify discord (webhook!, direct to bot?)
+ [x] Show author, dates for each item
+ [ ] Provide RSS/atom feed
+ [ ] Attempt to redirect to 'archive' folder for non-existing items
+ [ ] Expose filter by author
+ [ ] Expose sort by date, name

Deployment
---

1. Check config params in `foldergal.cfg`
  * add www path prefix if needed
  * set target folder root
1. Run with internal server:
   `python src/www.py &> log/foldergal.log`
1. [optional] Put behind nginx
   ```
    location /www-prefix/ {
      proxy_pass http://127.0.0.1:5000/www-prefix/;
      proxy_set_header Host $host;
    }
   ```
1. Test...
