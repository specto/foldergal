package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"foldergal/config"
	"foldergal/gallery"
	"foldergal/storage"
	"foldergal/templates"
	"github.com/spf13/afero"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

//go:generate go run storage/embed.go

func fileExists(filename string) bool {
	if file, err := os.Stat(filename); os.IsNotExist(err) || file.IsDir() {
		return false
	}
	return true
}

var (
	logger          *log.Logger
	cacheFolderName = "_foldergal_cache"
	BuildVersion    = "dev"
	BuildTimestamp  = "now"
	BuildTime       time.Time
	startTime       time.Time
	urlPrefix       string
	rssFreshness    = 2 * 168 * time.Hour
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
		Message: r.URL.String(),
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

var dangerousPathSymbols = regexp.MustCompile("[:]")

func sanitizePath(path string) (sanitized string) {
	if vol := filepath.VolumeName(path); vol != "" {
		sanitized = strings.TrimPrefix(path, vol)
		sanitized = strings.TrimPrefix(sanitized, "\\")
	} else {
		sanitized = path
	}
	dangerousPathSymbols.ReplaceAllString(sanitized, "_")
	return
}

// Serve image previews of Media files
func previewHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	ext := filepath.Ext(fullPath)
	mimeType := mime.TypeByExtension(ext)
	var f gallery.Media
	// All thumbnails are jpeg... most of the time
	thumbPath := sanitizePath(fullPath) + ".jpg"
	contentType := "image/jpeg"

	if strings.HasPrefix(mimeType, "image/svg") {
		contentType = mimeType
		thumbPath = fullPath // SVG are their own thumbnails
		f = &gallery.SvgFile{MediaFile: gallery.MediaFile{
			FullPath: fullPath, ThumbPath: thumbPath}}
	} else if strings.HasPrefix(mimeType, "image/") {
		f = &gallery.ImageFile{MediaFile: gallery.MediaFile{
			FullPath: fullPath, ThumbPath: thumbPath}}
	} else if strings.HasPrefix(mimeType, "audio/") {
		contentType = "image/svg+xml"
		f = &gallery.AudioFile{MediaFile: gallery.MediaFile{FullPath: fullPath}}
	} else if strings.HasPrefix(mimeType, "video/") {
		contentType = "image/svg+xml"
		f = &gallery.VideoFile{MediaFile: gallery.MediaFile{
			FullPath: fullPath, ThumbPath: thumbPath}}
	} else if strings.HasPrefix(mimeType, "application/pdf") {
		contentType = "image/svg+xml"
		f = &gallery.PdfFile{MediaFile: gallery.MediaFile{FullPath: fullPath}}
	} else {
		renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
		return
	}
	if !f.FileExists() {
		renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
		return
	}
	if f.ThumbExpired() {
		err := f.ThumbGenerate()
		if err != nil {
			logger.Print(err)
			renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
			return
		}
	}
	thumb := f.Thumb()
	if thumb == nil || *thumb == nil {
		renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
		return
	}
	if !strings.HasSuffix(f.Media().ThumbInfo.Name(), ".jpg") {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, f.Media().ThumbPath, f.Media().ThumbInfo.ModTime(), *thumb)
}

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

func folderSize(startPath string) (totalSize int64) {
	err := filepath.Walk(startPath,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			totalSize += info.Size()
			return nil
		})
	if err != nil {
		logger.Print(err)
	}
	return
}

func statusHandler(w http.ResponseWriter, _ *http.Request) {
	bToMb := func(b uint64) string {
		return fmt.Sprintf("%v MiB", b/1024/1024)
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	var rowData [][2]string
	rowData = append(rowData, [2]string{"Root:", config.Global.Root})
	rowData = append(rowData, [2]string{"Root Size:", bToMb(uint64(folderSize(config.Global.Root)))})
	rowData = append(rowData, [2]string{"Folders Watched:", fmt.Sprint(gallery.WatchedFolders)})
	rowData = append(rowData, [2]string{"Cache:", config.Global.Cache})
	rowData = append(rowData, [2]string{"Cache Size:", bToMb(uint64(folderSize(config.Global.Cache)))})
	rowData = append(rowData, [2]string{"Prefix:", config.Global.Prefix})
	rowData = append(rowData, [2]string{"Public Url:", config.Global.PublicUrl})
	rowData = append(rowData, [2]string{"-", ""})
	rowData = append(rowData, [2]string{"Alloc:", bToMb(m.Alloc)})
	rowData = append(rowData, [2]string{"TotalAlloc:", bToMb(m.TotalAlloc)})
	rowData = append(rowData, [2]string{"Sys:", bToMb(m.Sys)})
	rowData = append(rowData, [2]string{"NumGC:", fmt.Sprint(m.NumGC)})
	rowData = append(rowData, [2]string{"Goroutines:", fmt.Sprint(runtime.NumGoroutine())})
	rowData = append(rowData, [2]string{"-", ""})
	rowData = append(rowData, [2]string{"App Version:", BuildVersion})
	rowData = append(rowData, [2]string{"App Build Date:", BuildTimestamp})
	rowData = append(rowData, [2]string{"Service Uptime:", fmt.Sprint(time.Since(startTime))})

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

// Prepare list of files
func listHandler(w http.ResponseWriter, r *http.Request, sortBy string) {
	if gallery.ContainsDotFile(r.URL.Path) {
		fail404(w, r)
		return
	}
	var (
		parentUrl string
		title     string
		err       error
		contents  []os.FileInfo
	)
	folderPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	contents, err = afero.ReadDir(storage.Root, folderPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	folderInfo, _ := storage.Root.Stat(folderPath)
	if folderPath != "/" && folderPath != "" {
		title = filepath.Base(r.URL.Path)
		parentUrl = path.Join(r.URL.Path, "..")
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
		childPath := filepath.Join(urlPrefix, folderPath, gallery.EscapePath(child.Name()))
		childPath = filepath.ToSlash(childPath)
		thumb := urlPrefix + "/static?folder"
		class := "folder"
		if !child.IsDir() {
			thumbPath := filepath.Join(folderPath, gallery.EscapePath(child.Name()))
			thumb = thumbPath + "?thumb"
			class = mediaClass
		}
		children = append(children, templates.ListItem{
			ModTime: child.ModTime(),
			Url:     childPath,
			Name:    child.Name(),
			Thumb:   thumb,
			Class:   class,
			W:       config.Global.ThumbWidth,
			H:       config.Global.ThumbHeight,
		})
	}
	if sortBy == "date" {
		sort.Slice(children, func(i, j int) bool {
			return children[i].ModTime.After(children[j].ModTime)
		})
	} else { // Sort by name
		sort.Slice(children, func(i, j int) bool {
			return gallery.NaturalLess(
				strings.ToLower(children[i].Name),
				strings.ToLower(children[j].Name))
		})
	}
	crumbs := splitUrlToBreadCrumbs(r.URL)
	w.Header().Set("Date", folderInfo.ModTime().UTC().Format(http.TimeFormat))
	itemCount := ""
	if folderPath != "/" && folderPath != "" && len(children) > 0 {
		itemCount = fmt.Sprintf("%v ", len(children))
	}
	list := templates.List{
		Page: templates.Page{
			Title:        title,
			Prefix:       urlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		BreadCrumbs: crumbs,
		ItemCount:   itemCount,
		SortedBy:    sortBy,
		ParentUrl:   parentUrl,
		Items:       children,
	}
	err = templates.Html.ExecuteTemplate(w, "layout", &list)
	if err != nil {
		fail500(w, err, r)
		return
	}
}

// Serve actual files
func fileHandler(w http.ResponseWriter, r *http.Request) {
	if gallery.ContainsDotFile(r.URL.Path) {
		fail404(w, r)
		return
	}
	fullPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	thumbPath := strings.TrimSuffix(fullPath, filepath.Ext(fullPath)) + ".jpg"
	var err error
	m := gallery.MediaFile{
		FullPath:  fullPath,
		ThumbPath: thumbPath,
	}
	m.FileInfo, err = storage.Root.Stat(fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	if m.FileInfo.IsDir() || !gallery.IsValidMedia(fullPath) {
		fail404(w, r)
		return
	}
	contents := m.File()
	if contents == nil {
		fail404(w, r)
		return
	}

	http.ServeContent(w, r, fullPath, m.FileInfo.ModTime(), *contents)
}

func renderEmbeddedFile(resFile string, contentType string,
	w http.ResponseWriter, r *http.Request) {
	f, err := storage.Internal.Open(resFile)
	if err != nil {
		fail500(w, err, r)
		return
	}
	if contentType != "" {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, r.URL.Path, BuildTime, f)
}

func rssHandler(t string, w http.ResponseWriter, _ *http.Request) {
	loc, _ := time.LoadLocation("GMT")

	// Limit rss items only to the most fresh
	then := time.Now().Add(-rssFreshness) // negative duration to subtract
	isFresh := func(t time.Time) bool {
		return t.After(then)
	}
	typeRss := "atom"
	if t == "rss" {
		typeRss = "rss"
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
	}

	pathToUrl := func(p string) string {
		return config.Global.PublicUrl +
			strings.TrimPrefix(p, config.Global.Root+"/")
	}

	var rssItems []templates.RssItem
	err := filepath.Walk(config.Global.Root,
		func(walkPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && !gallery.ContainsDotFile(walkPath) &&
				gallery.IsValidMedia(walkPath) && isFresh(info.ModTime()) {
				url := pathToUrl(walkPath)
				rssItems = append(rssItems, templates.RssItem{
					Type:  gallery.GetMediaClass(walkPath),
					Title: url,
					Url:   url,
					Thumb: url + "?thumb",
					Id:    url,
					Mdate: info.ModTime(),
					Date:  info.ModTime().In(loc).Format(http.TimeFormat),
				})
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

	lastDate := time.Now()
	if len(rssItems) > 0 {
		lastDate = rssItems[0].Mdate
	}
	lastDateStr := lastDate.In(loc).Format(http.TimeFormat)
	w.Header().Set("Last-modified", lastDateStr)

	rss := templates.RssPage{
		FeedUrl:   config.Global.PublicUrl + "feed?" + typeRss,
		SiteTitle: config.Global.PublicUrl,
		SiteUrl:   config.Global.PublicUrl,
		LastDate:  lastDateStr,
		Items:     rssItems,
	}
	_ = templates.Rss.ExecuteTemplate(w, typeRss, &rss)
}

// A secondary router.
//
// Since we are mapping URLs to filesystem resources we cannot use any names
// for our internal resources.
//
// Three types of content are served:
//    * internal resource (image, css, etc.)
//    * html to show folder lists
//    * media file (thumbnail or larger file)
func httpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	q := r.URL.Query()
	// Retrieve sort order from cookie
	sortBy := "name"
	if sorted, nocookie := r.Cookie("sort"); nocookie == nil {
		sortBy = sorted.Value
	}
	// We use query string parameters for internal resources. Isn't that novel!
	if _, ok := q["status"]; ok {
		statusHandler(w, r)
		return
	} else if _, ok := q["thumb"]; ok {
		previewHandler(w, r)
		return
	} else if _, ok := q["by-date"]; ok {
		http.SetCookie(w, &http.Cookie{Name: "sort", Value: "date", MaxAge: 3e6})
		sortBy = "date"
	} else if _, ok := q["by-name"]; ok {
		http.SetCookie(w, &http.Cookie{Name: "sort", Value: "", MaxAge: -1})
		sortBy = "name"
	} else if _, ok := q["broken"]; ok {
		renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
		return
	} else if _, ok := q["up"]; ok {
		renderEmbeddedFile("res/up.svg", "image/svg+xml", w, r)
		return
	} else if _, ok := q["folder"]; ok {
		renderEmbeddedFile("res/folder.svg", "image/svg+xml", w, r)
		return
	} else if _, ok := q["favicon"]; ok {
		renderEmbeddedFile("res/favicon.ico", "", w, r)
		return
	} else if _, ok := q["css"]; ok {
		renderEmbeddedFile("res/style.css", "text/css", w, r)
		return
	} else if _, ok := q["rss"]; ok {
		rssHandler("rss", w, r)
		return
	} else if _, ok := q["atom"]; ok {
		rssHandler("atom", w, r)
		return
	} else if len(q) > 0 {
		fail404(w, r)
		return
	}

	stat, err := storage.Root.Stat(fullPath)
	if err != nil { // Non-existing resource was requested
		fail404(w, r)
		return
	}
	if stat.IsDir() { // Prepare and render folder contents
		listHandler(w, r, sortBy)
	} else { // This is a media file and we should serve it in all it's glory
		fileHandler(w, r)
	}
}

func init() {
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
	config.Global.Host = config.SfromEnv("HOST", "localhost")
	config.Global.Port = config.IfromEnv("PORT", 8080)
	config.Global.Home = config.SfromEnv("HOME", execFolder)
	config.Global.Root = config.SfromEnv("ROOT", execFolder)
	config.Global.Prefix = config.SfromEnv("PREFIX", "")
	config.Global.TlsCrt = config.SfromEnv("TLS_CRT", "")
	config.Global.TlsKey = config.SfromEnv("TLS_KEY", "")
	config.Global.Http2 = config.BfromEnv("HTTP2", false)
	config.Global.CacheExpiresAfter = config.DfromEnv("CACHE_EXPIRES_AFTER", 0)
	config.Global.NotifyAfter = config.DfromEnv("NOTIFY_AFTER", config.JsonDuration(30*time.Second))
	config.Global.DiscordWebhook = config.SfromEnv("DISCORD_WEBHOOK", "")
	config.Global.DiscordName = config.SfromEnv("DISCORD_NAME", "Gallery")
	config.Global.PublicHost = config.SfromEnv("PUBLIC_HOST", "")
	config.Global.Quiet = config.BfromEnv("QUIET", false)
	config.Global.ConfigFile = config.SfromEnv("CONFIG", "")
	config.Global.ThumbWidth = config.IfromEnv("THUMB_W", 400)
	config.Global.ThumbHeight = config.IfromEnv("THUMB_H", 400)

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
}

func main() {
	showVersion := flag.Bool("version", false, "show program version and build time")

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

	// Set up log file
	logFile := filepath.Join(config.Global.Home, "foldergal.log")
	logging, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644) // #nosec Permit everybody to read the log file
	if err != nil {
		log.Print("Error: Log File cannot be created.")
		log.Fatal(err)
	}
	defer func() { _ = logging.Close() }()
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
	if config.Global.CacheExpiresAfter == 0 {
		storage.Root = afero.NewReadOnlyFs(afero.NewBasePathFs(afero.NewOsFs(), config.Global.Root))
	} else {
		storage.Root = afero.NewCacheOnReadFs(
			afero.NewReadOnlyFs(afero.NewBasePathFs(afero.NewOsFs(), config.Global.Root)),
			afero.NewMemMapFs(),
			time.Duration(config.Global.CacheExpiresAfter))
	}

	// Set up caching folder
	config.Global.Cache = filepath.Join(config.Global.Home, cacheFolderName)
	err = os.MkdirAll(config.Global.Cache, 0750)
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
		logger.Printf("Cache in-memory expiration after %v", time.Duration(config.Global.CacheExpiresAfter))
	}

	// Routing
	httpmux := http.NewServeMux()
	if config.Global.Prefix != "" {
		urlPrefix = fmt.Sprintf("/%s", strings.Trim(config.Global.Prefix, "/"))
		httpmux.Handle(urlPrefix, http.StripPrefix(urlPrefix, http.HandlerFunc(httpHandler)))
	}
	httpmux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderEmbeddedFile("res/favicon.ico", "", w, r)
	}))
	httpmux.Handle("/", http.HandlerFunc(httpHandler))
	bind := fmt.Sprintf("%s:%d", config.Global.Host, config.Global.Port)

	if config.Global.Ffmpeg == "" {
		if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
			config.Global.Ffmpeg = ffmpegPath
			logger.Printf("Found ffmpeg at: %v", ffmpegPath)
		}
	}

	// Server start sequence
	useTls := false
	if fileExists(config.Global.TlsCrt) && fileExists(config.Global.TlsKey) {
		useTls = true
		logger.Printf("Using certificate: %s and key: %s", config.Global.TlsCrt, config.Global.TlsKey)
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
		tlsConfig := &tls.Config{}

		// Use separate certificate pool to avoid warnings with self-signed certs
		caCertPool := x509.NewCertPool()
		pem, _ := ioutil.ReadFile(config.Global.TlsCrt)
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
			Addr:      bind,
			Handler:   httpmux,
			TLSConfig: tlsConfig,
		}
		config.Global.PublicUrl = "https://" + config.Global.PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, config.Global.PublicUrl)
		if !config.Global.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, config.Global.PublicUrl)
		}
		srvErr = srv.ListenAndServeTLS(config.Global.TlsCrt, config.Global.TlsKey)
	} else { // Normal start
		config.Global.PublicUrl = "http://" + config.Global.PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, config.Global.PublicUrl)
		if !config.Global.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, config.Global.PublicUrl)
		}
		srvErr = http.ListenAndServe(bind, httpmux)
	}
}
