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
+ [x] Provide RSS/atom feed
+ [ ] Attempt to redirect to 'archive' folder for non-existing items
+ [ ] Expose filter by author
+ [ ] Expose sort by date, name
+ [ ] Provide apple-touch-icon.png
+ [ ] Use temporary folder for thumbnail cache
+ [ ] Display counter of images in current folder


Deployment
---

### Prerequisites

* python 3.6+
* pip

1. `pip install -r requirements.txt` (maybe in virtualenv)
1. Create a file `foldergal.local.cfg` and set those options that you need
  (overriding foldergal.cfg):
   - set target `FOLDER_ROOT` to the directory you will be serving files from
   - set `FOLDER_CACHE` to the directory where you want to store 
     the generated thumbnails, default is `cache` in the project
   - set `WWW_PREFIX` if you need to run it in subfolder, 
     e.g. for http://localhost/gallery/ use `WWW_PREFIX='gallery'`
   - `SERVER_NAME` is important when you use `DISCORD_WEBHOOK`
   - `AUTHORS` is used to map user ids from the file system to names and 
     then to show the user names. Don't set it if you don't need authors.
   - `TARGET_EXT` are the file extensions you want to be displayed. 
     Every file with different extension is ignored. Valid extensions are 
     the image file formats supported by `pillow`.
1. Run with internal server (maybe in virtualenv):
   `python src/www.py`
1. [optional] Put behind nginx
   ```
    location /www-prefix/ {
      proxy_pass http://127.0.0.1:5000/www-prefix/;
      proxy_set_header Host $host;
    }
   ```
1. Test...

Run as service on FreeBSD
---

1. Create a user to run the service.
1. Copy `rc.foldergal.sh` to `/usr/local/etc/rc.d/foldergal`
1. Important: edit the script to match your virtualenv and project directories 
   and also the new user. 
1. Then add `foldergal_enable="yes"` in `/etc/rc.conf`
1. You now have `service foldergal start`.

