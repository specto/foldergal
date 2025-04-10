Web Gallery from a Folder
===

A web server that displays a gallery of folders with images, video and audio.


Features
---

* __Browse media files__ - serves only media files from one folder 
  (images, audio, video and PDFs)
* __Local and Public__ - suitable for quick running to show something in the 
 local network as well as for long time running and public internet exposure
* __No editing__, no uploading - everything is read only
* __Usable without JavaScript__ - all javascript is optional
* __Portable__ - single executable file; available for 
  all major systems
* __Thumbnail generation__ - from all images
  (requires ffmpeg installed for audio and video files);
  in a temporary folder by default
* __Simple look__ - with light & dark theme support based on browser preferences
* __Content sorting__ - by file date or name
* __Shortcuts for navigation__ - next/previous with keyboard 
  and touch swipe (when the client has JavaScript enabled)
* __RSS/atom feed__
* __Discord web-hook__ - can notify for new uploads
* __Cache in memory__ - can be enabled to mitigate extensive reading from disk 
  when peaks in traffic happen
* __TLS and HTTP/2__ - can handle secure connections directly; 
  does not manage certificates for you

![](screenshot.png "Foldergal Screenshot")


Usage
---

1. Run `foldergal /path/to/images`
2. Visit http://localhost:8080

To run it publicly:
`foldergal --host 0.0.0.0 --port 80 /path/to/images`

### Configuration, Parameters and Environment variables

Run `./foldergal --help` for full info on command line parameters. 

Every parameter can be set with a corresponding ENV var.
For full list of the environment variables see [.env.defaults](.env.defaults).

The parameters will override the env variables.

You can also use a config file with all desired parameters.  
`./foldergal --config /path/to/config.json`

See [config.default.json](config.default.json) for details.
Parameter names in the config file are case insensitive.

All settings in the config file override the ENV and CLI parameters.

### Thumbnail Cache and Log

* foldergal.log
* foldergal_cache/...

By default the app creates a temporary folder to keep all 
generated files and a log file.
They are removed on exit.

If you provide a "home" folder, it is used for cache and log file storage
and it is not removed on exit.

### Folder metadata

Metadata can be read from files named `_foldergal.yaml` in any folder 
and displayed on list pages.

Supported fields are:
```yaml
description: Something about the images in the folder
copyright: text
```

Limitations and Known Issues
---

Privacy notice: all media files are served verbatim. 
This means that EXIF or other metadata is not removed from your files.

HTTP/2 works only with TLS, which works only if you provide certificate files.

In the rare case that you had set a "home" folder and used a "root" folder on 
a mounted volume (e.g. windows drive for network share) 
and later mounted a different volume which has files with the same names
you might see some erroneous thumbnails.

If you see an error stating "too many open files" you should set a larger 
file descriptor limit. Like running `ulimit -n 100000`.

Password protection is not implemented. Basic http authentication can be 
used with a proxy server to protect separate folders as well the whole gallery.
It should be decently secure when used over HTTPS.


Developer notes
---

`go run ./cmd/foldergal/main.go`

Use the [Makefile](Makefile).


Setting up as a service on FreeBSD
---

1. Copy the executable somewhere e.g. `/usr/local/bin/foldergal`
1. Create a user to run the service e.g. `foldergaluser`
1. Create the folder `/var/run/foldergal` and make the service user it's owner
1. Put there the contents of [example-rc-freebsd.sh](example-rc-freebsd.sh) in 
   `/usr/local/etc/rc.d/foldergal` and customize parameters
   (service user, correct HOME and ROOT paths, port, public name, etc.)
1. Make it executable `chmod u+x /usr/local/etc/rc.d/foldergal`
1. `sysrc foldergal_enable="YES"`
1. `service foldergal start`


Service on Debian
--

1. Copy the executable somewhere e.g. `/usr/local/bin/foldergal`
1. Create `/etc/systemd/system/foldergal.service` similar to 
   [example-systemd.service](example-systemd.service). 
   Configure paths, user and parameters.
1. `systemctl daemon-reload`
1. `service foldergal start`
1. Enable auto-start `systemctl enable foldergal`


TODO List
---

* [x] Generate audio file thumbnails 
* [x] Use touch events (swipe left and right) for overlay media navigation
* [x] Loading indicator for overlay image next and prev
* [x] Hide loading background for media
* [x] Add dark theme support
* [x] Rewrite embedded files using the `go:embed` directive available in go 1.16
* [x] Fix consistency of sort flags
* [x] Combine SVG icons in a single file and `<use>` sprites 
* [x] Introduce folder metadata files
* [x] Make sure thumbnail generation uses correct resizing  
  <https://zuru.tech/blog/the-dangers-behind-image-resizing>
* [ ] Generate pdf thumbnails (imagemagick?)
* [ ] Improve tests: closer to 100% test coverage; fuzz tests  
  <https://blog.fuzzbuzz.io/go-fuzzing-basics>
* [ ] (maybe) Dynamic folder icons generated from the full folder path
* [ ] Fix mysterious date bug 0001-01-01 on freebsd
* [ ] Implement Adaptive Video (Multi Bitrate HLS)  
  Serve pre-generated bundles of video files. 
  Provide JS fallback for browsers not supporting HLS.
  - <https://github.com/bluenviron/gohlslib>
  - <https://medium.com/@peer5/creating-a-production-ready-multi-bitrate-hls-vod-stream-dff1e2f1612c>
  - fallback script <https://github.com/video-dev/hls.js>
* [ ] (maybe) Use webauthn for authentication  
  <https://github.com/go-webauthn/webauthn>
  
