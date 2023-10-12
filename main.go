package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"foldergal/config"
	"foldergal/gallery"
	"foldergal/storage"
	"foldergal/templates"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/fvbommel/sortorder"
	"github.com/spf13/afero"
)

// Verifies if file exists and is not a folder
func fileExists(filename string) bool {
	if file, err := os.Stat(filename); os.IsNotExist(err) || file.IsDir() {
		return false
	}
	return true
}

const (
	QUERY_OVERLAY = "overlay"
)

var (
	logger           *log.Logger
	cacheFolderName  = "_foldergal_cache"
	BuildVersion     = "dev"
	BuildTimestamp   = "now"
	BuildTime        time.Time
	startTime        time.Time
	urlPrefix        string
	rssFreshness     = 2 * 168 * time.Hour // Two weeks
	rssNotFreshCount = 20                  // entries to show in RSS if not fresh
	faultyDate, _    = time.Parse("2006-01-02", "0001-01-02")
)

func fail404(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNotFound)
	page := templates.ErrorPage{
		Page: templates.Page{
			Title:        "404 not found",
			Prefix:       urlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		Message: r.URL.Path,
	}
	_ = templates.Html.ExecuteTemplate(w, "error", &page)
}

func fail500(w http.ResponseWriter, err error, _ *http.Request) {
	logger.Print(err)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusInternalServerError)
	page := templates.ErrorPage{
		Page: templates.Page{
			Title:        "500 internal server error",
			Prefix:       urlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		Message: "see the logs for error details",
	}
	_ = templates.Html.ExecuteTemplate(w, "error", &page)
}

// Get a subpath to a path
func sanitizePath(p string) string {
	base := "."
	if filepath.IsLocal(p) {
		return filepath.Clean(p)
	}
	result := filepath.Join(base, filepath.Clean(filepath.Base(p)))
	if result == ".." {
		return "."
	}
	return result
}

// Route for image previews of media files
func previewHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err         error
		f           gallery.Media
		contentType = "image/jpeg"
		fullPath    = strings.TrimPrefix(r.URL.Path, urlPrefix)
		// All thumbnails are jpegs... most of the time
		thumbPath = sanitizePath(fullPath) + ".jpg"
		mediaType = mime.TypeByExtension(filepath.Ext(fullPath))
	)
	switch {
	case strings.HasPrefix(mediaType, "image/svg"):
		contentType = mediaType
		f, err = gallery.NewSvg(fullPath)
	case strings.HasPrefix(mediaType, "image/"):
		f, err = gallery.NewImage(fullPath, thumbPath)
	case strings.HasPrefix(mediaType, "audio/"):
		contentType = "image/svg+xml"
		f, err = gallery.NewAudio(fullPath, thumbPath)
	case strings.HasPrefix(mediaType, "video/"):
		contentType = "image/svg+xml"
		f, err = gallery.NewVideo(fullPath, thumbPath)
	case strings.HasPrefix(mediaType, "application/pdf"):
		contentType = "image/svg+xml"
		f, err = gallery.NewPdf(fullPath, thumbPath)
	default: // Unrecognized mime type
		w.WriteHeader(http.StatusNotFound)
		renderEmbeddedFile("res/broken.svg", w, r)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		renderEmbeddedFile("res/broken.svg", w, r)
		return
	}
	err = gallery.GenerateThumb(f)
	if err != nil {
		fail500(w, err, r)
		return
	}
	thumb, err := f.Thumb()
	if err != nil {
		if errors.Is(err, gallery.ErrThumbNotPossible) {
			switch {
			case strings.HasPrefix(mediaType, "audio/"):
				renderEmbeddedFile("res/audio.svg", w, r)
				return
			case strings.HasPrefix(mediaType, "video/"):
				renderEmbeddedFile("res/video.svg", w, r)
				return
			}
		}
		logger.Print(err)
		w.WriteHeader(http.StatusNotFound)
		renderEmbeddedFile("res/broken.svg", w, r)
		return
	}
	if !strings.HasSuffix(f.ThumbName(), ".jpg") {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, f.ThumbPath(), f.ThumbModTime(), thumb)
}

// Splits a url "path" to separate tokens
func splitUrlToBreadCrumbs(pageUrl *url.URL) (crumbs []templates.BreadCrumb) {
	deepcrumb := urlPrefix + "/"
	currentUrl := strings.TrimPrefix(pageUrl.Path, urlPrefix)
	crumbs = append(crumbs, templates.BreadCrumb{Url: deepcrumb, Title: "#:\\"})
	for _, name := range strings.Split(currentUrl, "/") {
		if name == "" {
			continue
		}
		crumbs = append(crumbs,
			templates.BreadCrumb{Url: deepcrumb + name, Title: name})
		deepcrumb += name + "/"
	}
	return
}

// Counts recursively all valid media files in startPath
func fileCount(startPath string) (totalCount int64) {
	err := filepath.WalkDir(startPath,
		func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && gallery.IsValidMedia(path) {
				totalCount += 1
			}
			return nil
		})
	if err != nil {
		logger.Print(err)
	}
	return
}

// Retrieves the byte size of all files in startPath
func folderSize(startPath string) (totalSize int64) {
	err := filepath.WalkDir(startPath,
		func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				totalSize += info.Size()
			}
			return nil
		})
	if err != nil {
		logger.Print(err)
	}
	return
}

// Route for status report
func statusHandler(w http.ResponseWriter, _ *http.Request) {
	bToMb := func(b uint64) string {
		return fmt.Sprintf("%v MiB", b/1024/1024)
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	rowData := [][2]string{
		{"Total Files:", fmt.Sprintf("%v", uint64(fileCount(config.Global.Root)))},
		{"Media Folder Size:", bToMb(uint64(folderSize(config.Global.Root)))},
		{"Thumbnail Folder Size:", bToMb(uint64(folderSize(config.Global.Cache)))},
		{"Folders Watched:", fmt.Sprint(gallery.WatchedFolders)},
		{"Public Url:", config.Global.PublicUrl},
		{"Prefix:", config.Global.Prefix},
		{"-", ""},
		{"Alloc Memory:", bToMb(m.Alloc)},
		{"Sys Memory:", bToMb(m.Sys)},
		{"Goroutines:", fmt.Sprint(runtime.NumGoroutine())},
		{"-", ""},
		{"App Version:", BuildVersion},
		{"App Build Date:", BuildTimestamp},
		{"Service Uptime:", time.Since(startTime).String()},
	}

	page := templates.TwoColTable{
		Page: templates.Page{
			Title:        "System Status",
			Prefix:       urlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		Rows: rowData,
	}
	_ = templates.Html.ExecuteTemplate(w, "table", &page)
}

// Route for lists of files
func listHandler(w http.ResponseWriter, r *http.Request, opts config.CookieSettings, isOverlay bool) {
	if gallery.ContainsDotFile(r.URL.Path) {
		fail404(w, r)
		return
	}
	var (
		parentUrl  string
		title      string
		err        error
		contents   []os.FileInfo
		folderPath string
	)
	folderPath = strings.TrimPrefix(r.URL.Path, urlPrefix)
	if isOverlay {
		folderPath = filepath.Dir(folderPath)
	}
	fs, err := storage.Root.Open(folderPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	defer fs.Close()
	contents, err = fs.Readdir(-1)
	if err != nil {
		fail500(w, err, r)
		return
	}

	folderInfo, _ := fs.Stat()
	if folderPath != "/" && folderPath != "" {
		title = filepath.Base(r.URL.Path)
		parentUrl = path.Join(urlPrefix, folderPath, "..")
	} else if config.Global.PublicHost != "" {
		title = config.Global.PublicHost
	}

	children := make([]templates.ListItem, 0, len(contents))
	for _, child := range contents {
		if gallery.ContainsDotFile(child.Name()) {
			continue
		}
		mediaClass := gallery.GetMediaClass(child.Name())
		if !child.IsDir() && mediaClass == "" {
			continue
		}
		childPath := filepath.Join(urlPrefix, folderPath, child.Name())
		childPath = gallery.EscapePath(filepath.ToSlash(childPath))
		thumb := urlPrefix + "/?static/ui.svg#iconFolder"
		class := "folder"
		if !child.IsDir() {
			thumb = gallery.EscapePath(filepath.Join(urlPrefix, folderPath, child.Name())) + "?thumb"
			class = mediaClass
			if config.Global.Ffmpeg == "" {
				class += " nothumb"
			}
		}
		if child.ModTime().Before(faultyDate) {
			// TODO: find the reason for afero bad dates and remove this fix
			logger.Printf("Invalid date detected for %s", childPath)
			child, _ = storage.Root.Stat(filepath.Join(folderPath, child.Name()))
		}
		children = append(children, templates.ListItem{
			Id:      gallery.EscapePath(child.Name()),
			ModTime: child.ModTime(),
			Url:     childPath,
			Name:    child.Name(),
			Thumb:   thumb,
			Class:   class,
			W:       config.Global.ThumbWidth,
			H:       config.Global.ThumbHeight,
		})
	}
	if opts.Sort == "date" {
		sort.Slice(children, func(i, j int) bool {
			if opts.Order {
				j, i = i, j
			}
			return children[i].ModTime.Before(children[j].ModTime)
		})
	} else { // Sort by name
		sort.Slice(children, func(i, j int) bool {
			if opts.Order {
				j, i = i, j
			}
			return sortorder.NaturalLess(
				strings.ToLower(children[i].Name),
				strings.ToLower(children[j].Name))
		})
	}
	pUrl, _ := url.Parse(folderPath)
	crumbs := splitUrlToBreadCrumbs(pUrl)
	w.Header().Set("Date", folderInfo.ModTime().UTC().Format(http.TimeFormat))
	itemCount := ""
	if folderPath != "/" && folderPath != "" && len(children) > 0 {
		itemCount = fmt.Sprintf("%v ", len(children))
	}
	err = templates.Html.ExecuteTemplate(w, "layout", &templates.List{
		Page: templates.Page{
			Title:        title,
			Prefix:       urlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
			ShowOverlay:  isOverlay,
		},
		BreadCrumbs: crumbs,
		ItemCount:   itemCount,
		SortedBy:    opts.Sort,
		IsReversed:  opts.Order,
		DisplayMode: opts.Show,
		ParentUrl:   parentUrl,
		Items:       children,
	})
	if err != nil {
		fail500(w, err, r)
		return
	}
}

// Route to serve actual files
func fileHandler(w http.ResponseWriter, r *http.Request) {
	if gallery.ContainsDotFile(r.URL.Path) {
		fail404(w, r)
		return
	}
	fullPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	media, err := gallery.NewMedia(fullPath)
	if err != nil {
		if errors.Is(err, gallery.ErrNotValid) {
			// Hide invalid media as non-existing
			fail404(w, r)
			return
		}
		fail500(w, err, r)
		return
	}
	contents, err := media.File()
	if err != nil {
		if errors.Is(err, gallery.ErrFileNotFound) {
			fail404(w, r)
			return
		}
		fail500(w, err, r)
		return
	}

	http.ServeContent(w, r, fullPath, media.FileModTime(), contents)
}

// Delivers file contents for static resources
func renderEmbeddedFile(resFile string, w http.ResponseWriter, r *http.Request) {
	f, err := storage.InternalHttp.Open(resFile)
	if err != nil {
		fail404(w, r)
		return
	}
	defer f.Close()
	var name string
	if q, _ := parseQuery(r.URL.RawQuery); q.Has("static") {
		name = q.Get("static")
	} else {
		name = filepath.Base(resFile)
	}
	http.ServeContent(w, r, name, BuildTime, f)
}

// Route for RSS/Atom
func rssHandler(t string, w http.ResponseWriter, r *http.Request) {
	loc, _ := time.LoadLocation("UTC")

	typeRss := "atom"
	var formatTime func(time.Time) string
	if t == "rss" {
		typeRss = "rss"
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		formatTime = func(t time.Time) string {
			return t.In(loc).Format(http.TimeFormat)

		}
	} else {
		w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
		formatTime = func(t time.Time) string {
			return t.In(loc).Format(time.RFC3339)
		}
	}

	pathToUrl := func(p string) string {
		return gallery.EscapePath(config.Global.PublicUrl +
			strings.TrimPrefix(p, config.Global.Root+"/"))
	}

	var rssItems []templates.RssItem
	err := filepath.WalkDir(config.Global.Root,
		func(walkPath string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && !gallery.ContainsDotFile(walkPath) &&
				gallery.IsValidMedia(walkPath) {
				if info, err := entry.Info(); err == nil {
					urlStr := pathToUrl(walkPath)
					rssItems = append(rssItems, templates.RssItem{
						Type:  gallery.GetMediaClass(walkPath),
						Title: filepath.Base(walkPath),
						Url:   urlStr,
						Thumb: urlStr + "?thumb",
						Id:    urlStr,
						Mdate: info.ModTime(),
						Date:  formatTime(info.ModTime()),
					})
				}
				return nil
			}
			return nil
		})
	if err != nil {
		logger.Print(err)
	}

	sort.Slice(rssItems, func(i, j int) bool {
		return rssItems[i].Mdate.After(rssItems[j].Mdate)
	})

	// Filter latest entries
	var latestItems []templates.RssItem
	freshPeriod := time.Now().Add(-rssFreshness) // negative duration to subtract
	for _, entry := range rssItems {
		if entry.Mdate.After(freshPeriod) || len(latestItems) < rssNotFreshCount {
			latestItems = append(latestItems, entry)
		}
	}

	lastDate := time.Now()
	if len(latestItems) > 0 {
		lastDate = latestItems[0].Mdate
	}
	lastDateStr := formatTime(lastDate)
	w.Header().Set("Last-modified", lastDateStr)

	rss := templates.RssPage{
		FeedUrl:   config.Global.PublicUrl + "feed?" + typeRss,
		SiteTitle: config.Global.PublicHost,
		SiteUrl:   config.Global.PublicUrl,
		LastDate:  lastDateStr,
		Items:     latestItems,
	}
	if err := templates.Rss.ExecuteTemplate(w, typeRss, &rss); err != nil {
		fail500(w, err, r)
	}
}

// Parses a string like key1/value1/key2/value2 to a map.
func parseQuery(q string) (m url.Values, err error) {
	m = make(url.Values)
	if strings.HasPrefix(q, ";") || strings.HasSuffix(q, ";") {
		err = fmt.Errorf("invalid semicolon separator in query")
	}
	parts := strings.Split(q, "/")
	length := len(parts)
	for i := 0; i < length; i += 2 {
		if strings.Contains(parts[i], ";") {
			err = fmt.Errorf("invalid semicolon separator in query")
			i -= 1
			continue
		}
		if i+1 >= length {
			break
		}
		if strings.Contains(parts[i+1], ";") {
			err = fmt.Errorf("invalid semicolon separator in query")
			i += 1
			continue
		}
		key, err1 := url.QueryUnescape(parts[i])
		if err1 != nil {
			err = err1
			continue
		}
		val, err1 := url.QueryUnescape(parts[i+1])
		if err1 != nil {
			err = err1
			continue
		}
		m.Add(key, val)
	}
	return m, err
}

// A secondary router.
//
// Since we are mapping URLs to filesystem resources we cannot use any names
// for our internal resources.
//
// Three types of content are served:
//   - internal resource (image, css, etc.)
//   - html to show folder lists
//   - media file (thumbnail or larger file)
func HttpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	q, _ := parseQuery(r.URL.RawQuery)
	cookieName := "settings"

	opts := config.DefaultCookieSettings()
	if settingsCookie, nocookie := r.Cookie(cookieName); nocookie == nil {
		_ = opts.Unmarshal(settingsCookie.Value)
	}

// 	mustSaveSettings := false

	// All these can be set simultaneously in the query string
	reqOrder := q.Get("order")
	if reqOrder != "" {
		if reqOrder == "desc" {
			opts.Order = true
		} else {
			opts.Order = false
		}
	}
// 	if _, ok := q["asc"]; ok {
// 		opts.Order = false
// 		mustSaveSettings = true
// 	}
// 	if _, ok := q["desc"]; ok {
// 		opts.Order = true
// 		mustSaveSettings = true
// 	}
	if reqSort := q.Get("sort"); reqSort != "" {
		opts.Sort = reqSort
		// Default order for date sorting must be descending
		if reqSort == "date" && reqOrder == "" {
			opts.Order = true
		}
	}
// 	if _, ok := q["by-date"]; ok {
// 		opts.Sort = "date"
// 		mustSaveSettings = true
// 	}
// 	if _, ok := q["by-name"]; ok {
// 		if !mustSaveSettings { // Default order for name is ascending
// 			opts.Order = false
// 		}
// 		opts.Sort = "name"
// 		mustSaveSettings = true
// 	}
	if reqDisplay := q.Get("display"); reqDisplay != "" {
		opts.Show = reqDisplay
	}
// 	if _, ok := q["show-inline"]; ok {
// 		opts.Show = "inline"
// 		mustSaveSettings = true
// 	}
// 	if _, ok := q["show-files"]; ok {
// 		opts.Show = "files"
// 		mustSaveSettings = true
// 	}

// 	if mustSaveSettings {
// 		cookieData, err := opts.Marshal()
// 		if err == nil {
// 			cookiePath := urlPrefix
// 			if cookiePath == "" {
// 				cookiePath = "/"
// 			}
// 			http.SetCookie(w, &http.Cookie{Name: cookieName,
// 				Value: cookieData, MaxAge: 3e6, Path: cookiePath})
// 		} else {
// 			log.Printf("Error creating cookie: %s", err)
// 		}
// 	}

	// We use query string parameters for internal resources. Isn't that novel!
	if q.Has("status") {
		statusHandler(w, r)
		return
	} else if q.Has("thumb") {
		previewHandler(w, r)
		return
	} else if q.Has("broken") { // Keep this separate from static, just in case...
		renderEmbeddedFile("res/broken.svg", w, r)
		return
	} else if q.Has("static") {
		staticResource := q.Get("static")
		renderEmbeddedFile("res/"+staticResource, w, r)
		return
	} else if q.Has("rss") {
		rssHandler("rss", w, r)
		return
	} else if q.Has("atom") {
		rssHandler("atom", w, r)
		return
	} else if q.Has("error") {
		fail404(w, r)
		return
	}

	stat, err := storage.Root.Stat(fullPath)
	if err != nil { // Non-existing resource was requested
		fail404(w, r)
		return
	}
	if stat.IsDir() || opts.Show == QUERY_OVERLAY {
		// Prepare and render folder contents
		listHandler(w, r, opts, opts.Show == QUERY_OVERLAY)
	} else { // This is a media file and we should serve it in all it's glory
		fileHandler(w, r)
	}
}

func realInit() {
	startTime = time.Now()
	var errTime error
	BuildTime, errTime = time.Parse(time.RFC3339, BuildTimestamp)
	if errTime != nil {
		BuildTime = time.Now()
	}

	// Get current execution folder
	execFolder, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Environment variables
	config.Global.LoadEnv(execFolder)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\tNotice: most command line arguments\n")
		fmt.Fprintf(os.Stderr, "\tare avalable as environment variables, \n")
		fmt.Fprintf(os.Stderr, "\te.g. %sPORT=8080\n\n", config.EnvPrefix)

		flag.PrintDefaults()
	}

	// Command line arguments, they override env
	flag.StringVar(&config.Global.Host, "host", config.Global.Host, "host address to bind to")
	flag.IntVar(&config.Global.Port, "port", config.Global.Port, "port to run at")
	flag.StringVar(&config.Global.Home, "home", config.Global.Home, "home folder e.g. to keep thumbnails")
	flag.StringVar(&config.Global.Root, "root", config.Global.Root, "root folder to serve files from")
	flag.StringVar(&config.Global.Prefix, "prefix", config.Global.Prefix,
		"path prefix as in http://localhost/PREFIX/other/stuff")
	flag.StringVar(&config.Global.TlsCrt, "crt", config.Global.TlsCrt, "certificate File for TLS")
	flag.StringVar(&config.Global.TlsKey, "key", config.Global.TlsKey, "key file for TLS")
	flag.BoolVar(&config.Global.Http2, "http2", config.Global.Http2, "enable HTTP/2 (only with TLS)")
	flag.BoolVar(&config.Global.Quiet, "quiet", config.Global.Quiet, "don't print to console")
	flag.DurationVar((*time.Duration)(&config.Global.CacheExpiresAfter),
		"cache-expires-after", time.Duration(config.Global.CacheExpiresAfter),
		"duration to keep cached resources in memory")
	flag.DurationVar((*time.Duration)(&config.Global.NotifyAfter),
		"notify-after", time.Duration(config.Global.NotifyAfter),
		"duration to delay notifications and combine them in one")
	flag.StringVar(&config.Global.DiscordWebhook, "discord-webhook", config.Global.DiscordWebhook,
		"webhook URL to receive notifications when new media appears")
	flag.StringVar(&config.Global.DiscordName, "discord-name", config.Global.DiscordName,
		"name to appear on sent notifications")
	flag.StringVar(&config.Global.PublicHost, "pub-host", config.Global.PublicHost,
		"the public name for the machine")
	flag.IntVar(&config.Global.ThumbWidth, "thumb-width", config.Global.ThumbWidth, "width for thumbnails")
	flag.IntVar(&config.Global.ThumbHeight, "thumb-height", config.Global.ThumbHeight, "height for thumbnails")
	flag.StringVar(&config.Global.ConfigFile, "config", config.Global.ConfigFile,
		"json file to get all the parameters from")

	showVersion := flag.Bool("version", false, "show program version and build time")

	// ??? Move below to main

	flag.Parse()

	if *showVersion {
		fmt.Printf("foldergal %v, built on %v\n", BuildVersion, BuildTime.In(time.Local))
		os.Exit(0)
	}

	// Configuration file variables override all others
	if config.Global.ConfigFile != "" {
		if err := config.Global.FromJson(config.Global.ConfigFile); err != nil {
			log.Fatalf("error: invalid config %v", err)
		}
	}
	config.Global.Home, _ = filepath.Abs(config.Global.Home)
	config.Global.Root, _ = filepath.Abs(config.Global.Root)

	config.Global.TimeLocation, err = time.LoadLocation(config.Global.TimeZone)
	if err != nil {
		log.Fatal(err)
	}

	// Set up log file
	logFile := filepath.Join(config.Global.Home, "foldergal.log")
	logging, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o0644) // #nosec Permit everybody to read the log file
	if err != nil {
		log.Print("Error: Log File cannot be created.")
		log.Fatal(err)
	}
	config.Global.Log = log.New(logging, "foldergal: ", log.Lshortfile|log.LstdFlags)
	logger = config.Global.Log
	if !config.Global.Quiet {
		log.Printf("Logging to %s", logFile)
	}

	// Set root media folder
	if exists, err := os.Stat(config.Global.Root); os.IsNotExist(err) || !exists.IsDir() {
		log.Fatalf("Root folder does not exist: %v", config.Global.Root)
	}
	logger.Printf("Root folder is: %s", config.Global.Root)
	if !config.Global.Quiet {
		log.Printf("Serving files from: %v", config.Global.Root)
	}
	baseRoot := afero.NewReadOnlyFs(
		afero.NewBasePathFs(afero.NewOsFs(), config.Global.Root))
	if config.Global.CacheExpiresAfter == 0 {
		storage.Root = baseRoot
	} else {
		storage.Root = afero.NewCacheOnReadFs(
			baseRoot,
			afero.NewMemMapFs(),
			time.Duration(config.Global.CacheExpiresAfter))
	}

	// Set up caching folder
	config.Global.Cache = filepath.Join(config.Global.Home, cacheFolderName)
	err = os.MkdirAll(config.Global.Cache, 0o0750)
	if err != nil {
		log.Fatal(err)
	}
	if !config.Global.Quiet {
		log.Printf("Cache folder is: %s\n", config.Global.Cache)
	}
	logger.Printf("Cache folder is: %s", config.Global.Cache)
	if config.Global.CacheExpiresAfter == 0 {
		storage.Cache = afero.NewBasePathFs(afero.NewOsFs(), config.Global.Cache)
	} else {
		storage.Cache = afero.NewCacheOnReadFs(
			afero.NewBasePathFs(afero.NewOsFs(), config.Global.Cache),
			afero.NewMemMapFs(),
			time.Duration(config.Global.CacheExpiresAfter))
		logger.Printf("Cache in-memory expiration after %v",
			time.Duration(config.Global.CacheExpiresAfter))
	}
}

func main() {
	realInit()

	// Routing
	httpmux := http.NewServeMux()
	if config.Global.Prefix != "" {
		urlPrefix = fmt.Sprintf("/%s", strings.Trim(config.Global.Prefix, "/"))
		httpmux.Handle(urlPrefix,
			http.StripPrefix(urlPrefix, http.HandlerFunc(HttpHandler)))
	}
	httpmux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderEmbeddedFile("res/favicon.ico", w, r)
	}))
	httpmux.Handle("/", http.HandlerFunc(HttpHandler))
	bind := fmt.Sprintf("%s:%d", config.Global.Host, config.Global.Port)

	ffmpegPath := config.Global.Ffmpeg
	if config.Global.Ffmpeg == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffmpegPath, err := exec.LookPath(ffmpegPath); err == nil {
		config.Global.Ffmpeg = ffmpegPath
		logger.Printf("Found ffmpeg at: %v", ffmpegPath)
	} else {
		config.Global.Ffmpeg = ""
	}

	// Server start sequence
	useTls := false
	if fileExists(config.Global.TlsCrt) && fileExists(config.Global.TlsKey) {
		useTls = true
		logger.Printf("Using certificate: %s and key: %s",
			config.Global.TlsCrt, config.Global.TlsKey)
	}
	if config.Global.DiscordWebhook != "" { // Start filesystem watcher
		go gallery.StartFsWatcher()
	}
	var srvErr error
	defer func() {
		if srvErr != nil {
			log.Fatal(srvErr)
		}
	}()
	if config.Global.PublicHost != "" {
		config.Global.PublicUrl = strings.Trim(config.Global.PublicHost, "/") + urlPrefix + "/"
	} else {
		config.Global.PublicUrl = bind + urlPrefix + "/"
	}

	if useTls { // Prepare the TLS
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// Use separate certificate pool to avoid warnings with self-signed certs
		caCertPool := x509.NewCertPool()
		pem, _ := os.ReadFile(config.Global.TlsCrt)
		caCertPool.AppendCertsFromPEM(pem)
		tlsConfig.RootCAs = caCertPool

		// Optional http2
		if config.Global.Http2 {
			logger.Print("Using HTTP/2")
			tlsConfig.NextProtos = []string{"h2"}
		} else {
			tlsConfig.NextProtos = []string{"http/1.1"}
		}
		srv := &http.Server{
			ReadHeaderTimeout: 30 * time.Second,
			Addr:              bind,
			Handler:           httpmux,
			TLSConfig:         tlsConfig,
		}
		config.Global.PublicUrl = "https://" + config.Global.PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, config.Global.PublicUrl)
		if !config.Global.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, config.Global.PublicUrl)
		}
		srvErr = srv.ListenAndServeTLS(config.Global.TlsCrt, config.Global.TlsKey)
	} else { // Normal start
		srv := &http.Server{
			ReadHeaderTimeout: 30 * time.Second,
			Addr:              bind,
			Handler:           httpmux,
		}
		config.Global.PublicUrl = "http://" + config.Global.PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, config.Global.PublicUrl)
		if !config.Global.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, config.Global.PublicUrl)
		}
		srvErr = srv.ListenAndServe()
	}
}
