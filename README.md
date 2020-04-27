Web Gallery from Folders
===

A web server that displays folders of images, video and audio.

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

### Configuration

#### Config File

`foldergal --config /path/to/config.json`

See config.default.json for details. Parameter names are case insensitive.

All settings in the config file override the env and cli parameters!

#### Environment Variables
```
export FOLDERGAL_HOST=localhost
export FOLDERGAL_PORT=8080
export FOLDERGAL_HOME=/path/to/settings/
export FOLDERGAL_ROOT=/files/to/serve/
export FOLDERGAL_PREFIX=gallery
export FOLDERGAL_CRT=/tls/server.crt
export FOLDERGAL_KEY=/tls/server.key
export FOLDERGAL_HTTP2=true
export FOLDERGAL_CACHE_EXPIRES_AFTER=0
export FOLDERGAL_NOTIFY_AFTER=30s
export FOLDERGAL_DISCORD_WEBHOOK=https://discordapp.com/api/webhooks/xxxxx
export FOLDERGAL_PUBLIC_HOST=example.com
export FOLDERGAL_QUIET=true
```

#### Parameters

Run `./foldergal --help` for full info on command line parameters. 
They will override the env variables.

#### Home folder structure

* home (defaults to current folder)
  * `foldergal.log`
  * `_foldergal_cache/` (created automatically)
  
You can put config.json and tls certificates there but you will still have 
to point the executable to them.

Limitations and known issues
---

Privacy warning: all media is served as is, to the byte. 
This means that no EXIF or other metadata is removed from your files.

Http2 works only with TLS, which works only if you provide certificate files.

You should clean the "home" folder manually after using the application.
The thumnail cache and the log file remain on your disk.

In the rare case that you shared a folder from one volume (windows drive) and
later shared a folder with the same name but from different volume
you might see some erroneous thumbnails.

Developer notes
---

When building use these flags to set the build time and version:
```
go build -ldflags="-X 'main.BuildTimestamp=TIME' -X 'main.BuildVersion=VERSION'"
```
Where TIME must be an RFC3339 string.

Package fsnotify requires `go get -u golang.org/x/sys/...`

Setting up service on Freebsd
---

1. copy somewehere the executable e.g. `/usr/local/bin/foldergal`
1. create a user to run the service e.g. `foldergaluser`
1. create the folder `/var/run/foldergal` and make the service user it's owner
1. create a file `/usr/local/etc/rc.d/foldergal` make sure it is executable
1. put there the contents of `rc-example-freebsd.sh` and edit
1. set the service user, correct paths, port, public name
1. add the line `foldergal_enable="yes"` in /etc/rc.conf
1. `service foldergal start`
1. you can check the logs in `FOLDERGAL_HOME` foldergal.log

Important: in order for file type detection to work you will need to have 
`/etc/mime-types` which freebsd does not have. 
One solution is to install apache and symlink to it's mime.types.
Ugly, yes, but foldergal will not see your videos otherwise.

Service on linux
--

Create `/etc/systemd/system/foldergal.service` similar to 
`systemd-example.service`. Change paths, user and don't forget to 
edit your `config.json`.

After `sudo systemctl daemon-reload` you can `sudo service foldergal start`.

To enable auto-start: `sudo systemctl enable foldergal`

TODO
---

* [x] Enable support for http2 (with tls) and fall back to http1
* [x] Embed favicon and serve it
* [x] Use and show app version
* [x] Notifications for new files via web e.g. discord webhook.
* [x] Show number of items in folder
* [x] Flag to turn off in-memory cache
* [x] Call ffmpeg binary to generate video thumbnail
* [ ] RSS feed
* [ ] In-browser notifications for new uploads

