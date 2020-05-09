package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
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
	"strconv"
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
	config          gallery.Config
	cacheFolder     string
	cacheFolderName = "_foldergal_cache"
	BuildVersion    = "dev"
	BuildTimestamp  = "now"
	BuildTime       time.Time
	startTime       time.Time
	urlPrefix       string
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

func sanitizePath(path string) string {
	var sanitized string
	if vol := filepath.VolumeName(path); vol != "" {
		sanitized = strings.TrimPrefix(path, vol)
		sanitized = strings.TrimPrefix(sanitized, "\\")
	} else {
		sanitized = path
	}
	dangerousPathSymbols.ReplaceAllString(sanitized, "_")
	return sanitized
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
	bToMb := func(b uint64) uint64 {
		return b / 1024 / 1024
	}
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	_, _ = fmt.Fprintf(w, "Root:        %v\n", config.Root)
	_, _ = fmt.Fprintf(w, "Root size:   %v MiB\n", bToMb(uint64(folderSize(config.Root))))
	_, _ = fmt.Fprintf(w, "Cache:       %v\n", cacheFolder)
	_, _ = fmt.Fprintf(w, "Cache size:  %v MiB\n", bToMb(uint64(folderSize(cacheFolder))))
	_, _ = fmt.Fprintf(w, "FolderWatch: %v\n", gallery.WatchedFolders)
	_, _ = fmt.Fprintf(w, "\n")
	_, _ = fmt.Fprintf(w, "Alloc:       %v MiB\n", bToMb(m.Alloc))
	_, _ = fmt.Fprintf(w, "TotalAlloc:  %v MiB\n", bToMb(m.TotalAlloc))
	_, _ = fmt.Fprintf(w, "Sys:         %v MiB\n", bToMb(m.Sys))
	_, _ = fmt.Fprintf(w, "NumGC:       %v\n", m.NumGC)
	_, _ = fmt.Fprintf(w, "Goroutines:  %v\n", runtime.NumGoroutine())
	//_, _ = fmt.Fprintf(w, "goVersion:   %v\n", runtime.Version())
	_, _ = fmt.Fprintf(w, "SvcUptime:   %v\n", time.Since(startTime))
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
	} else if config.PublicHost != "" {
		title = config.PublicHost
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
		childPath = filepath.ToSlash(childPath)
		thumb := urlPrefix + "/static?folder"
		class := "folder"
		if !child.IsDir() {
			thumb = filepath.Join(folderPath, child.Name()+"?thumb")
			class = mediaClass
		}
		children = append(children, templates.ListItem{
			ModTime: child.ModTime(),
			Url:     childPath,
			Name:    child.Name(),
			Thumb:   thumb,
			Class:   class,
			W:       config.ThumbWidth,
			H:       config.ThumbHeight,
		})
	}
	if sortBy == "date" {
		sort.Slice(children, func(i, j int) bool {
			return children[i].ModTime.After(children[j].ModTime)
		})
	} else { // Sort by name
		sort.Slice(children, func(i, j int) bool {
			return strings.ToLower(children[i].Name) < strings.ToLower(children[j].Name)
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

// A secondary router.
//
// Since we are mapping URLs to File system resources we cannot use any names
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

	config.ThumbHeight = 200
	config.ThumbWidth = 200

	// Environment variables
	if config.Host = os.Getenv("FOLDERGAL_HOST"); config.Host == "" {
		config.Host = "localhost"
	}
	if config.Port, _ = strconv.Atoi(os.Getenv("FOLDERGAL_HOST")); config.Port == 0 {
		config.Port = 8080
	}
	if config.Home = os.Getenv("FOLDERGAL_HOME"); config.Home == "" {
		config.Home = execFolder
	}
	if config.Root = os.Getenv("FOLDERGAL_ROOT"); config.Root == "" {
		config.Root = execFolder
	}
	config.Prefix = os.Getenv("FOLDERGAL_PREFIX")
	config.TlsCrt = os.Getenv("FOLDERGAL_CRT")
	config.TlsKey = os.Getenv("FOLDERGAL_KEY")
	config.Http2, _ = strconv.ParseBool(os.Getenv("FOLDERGAL_HTTP2"))
	if envValue := os.Getenv("FOLDERGAL_CACHE_EXPIRES_AFTER"); envValue != "" {
		envCacheExpires, _ := time.ParseDuration(envValue)
		config.CacheExpiresAfter = gallery.JsonDuration(envCacheExpires)
	}
	if envValue := os.Getenv("FOLDERGAL_NOTIFY_AFTER"); envValue != "" {
		envNotifyAfter, _ := time.ParseDuration(envValue)
		config.NotifyAfter = gallery.JsonDuration(envNotifyAfter)
	} else {
		config.NotifyAfter = gallery.JsonDuration(30 * time.Second)
	}
	config.DiscordWebhook = os.Getenv("FOLDERGAL_DISCORD_WEBHOOK")
	config.PublicHost = os.Getenv("FOLDERGAL_PUBLIC_HOST")
	config.Quiet, _ = strconv.ParseBool(os.Getenv("FOLDERGAL_QUIET"))

	// Command line arguments (they override env)
	flag.StringVar(&config.Host, "host", config.Host, "host address to bind to")
	flag.IntVar(&config.Port, "port", config.Port, "port to run at")
	flag.StringVar(&config.Home, "home", config.Home, "home folder e.g. to keep thumbnails")
	flag.StringVar(&config.Root, "root", config.Root, "root folder to serve files from")
	flag.StringVar(&config.Prefix, "prefix", config.Prefix,
		"path prefix as in http://localhost/PREFIX/other/stuff")
	flag.StringVar(&config.TlsCrt, "crt", config.TlsCrt, "certificate File for TLS")
	flag.StringVar(&config.TlsKey, "key", config.TlsKey, "key file for TLS")
	flag.BoolVar(&config.Http2, "http2", config.Http2, "enable HTTP/2 (only with TLS)")
	flag.BoolVar(&config.Quiet, "quiet", config.Quiet, "don't print to console")
	flag.DurationVar((*time.Duration)(&config.CacheExpiresAfter), "cache-expires-after",
		time.Duration(config.CacheExpiresAfter),
		"duration to keep cached resources in memory")
	flag.DurationVar((*time.Duration)(&config.NotifyAfter), "notify-after",
		time.Duration(config.NotifyAfter),
		"duration to delay notifications and combine them in one")
	flag.StringVar(&config.DiscordWebhook, "discord", config.DiscordWebhook,
		"webhook URL to receive notifications when new media appears")
	flag.StringVar(&config.PublicHost, "pub-host", config.PublicHost,
		"the public name for the machine")

	// The following order is important
	storage.Intialize()
	templates.Initialize()
}

func main() {
	configFile := flag.String("config", "",
		"json file to get all the parameters from")
	showVersion := flag.Bool("version", false, "show program version and build time")

	flag.Parse()

	if *showVersion {
		fmt.Printf("foldergal %v, built on %v\n", BuildVersion, BuildTime.In(time.Local))
		os.Exit(0)
	}

	// Config file variables override all others
	if err := config.FromJson(*configFile); *configFile != "" && err != nil {
		log.Fatalf("error: invalid config %v", err)
	}
	config.Home, _ = filepath.Abs(config.Home)
	config.Root, _ = filepath.Abs(config.Root)

	// Set up log file
	logFile := filepath.Join(config.Home, "foldergal.log")
	logging, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644) // #nosec Permit everybody to read the log file
	if err != nil {
		log.Print("Error: Log File cannot be created.")
		log.Fatal(err)
	}
	defer func() { _ = logging.Close() }()
	logger = log.New(logging, "foldergal: ", log.Lshortfile|log.LstdFlags)
	if !config.Quiet {
		log.Printf("Logging to %s", logFile)
	}

	// Set root media folder
	if exists, err := os.Stat(config.Root); os.IsNotExist(err) || !exists.IsDir() {
		log.Fatalf("Root folder does not exist: %v", config.Root)
	}
	logger.Printf("Root folder is: %s", config.Root)
	if !config.Quiet {
		log.Printf("Serving files from: %v", config.Root)
	}
	if config.CacheExpiresAfter == 0 {
		storage.Root = afero.NewReadOnlyFs(afero.NewBasePathFs(afero.NewOsFs(), config.Root))
	} else {
		storage.Root = afero.NewCacheOnReadFs(
			afero.NewReadOnlyFs(afero.NewBasePathFs(afero.NewOsFs(), config.Root)),
			afero.NewMemMapFs(),
			time.Duration(config.CacheExpiresAfter))
	}

	//stat, _ := storage.Internal.Stat("asdf.svg")
	//fmt.Printf("%v\n", stat.Size())

	// Set up caching folder
	cacheFolder = filepath.Join(config.Home, cacheFolderName)
	err = os.MkdirAll(cacheFolder, 0750)
	if err != nil {
		log.Fatal(err)
	}
	if !config.Quiet {
		log.Printf("Cache folder is: %s\n", cacheFolder)
	}
	logger.Printf("Cache folder is: %s", cacheFolder)
	if config.CacheExpiresAfter == 0 {
		storage.Cache = afero.NewBasePathFs(afero.NewOsFs(), cacheFolder)
	} else {
		storage.Cache = afero.NewCacheOnReadFs(
			afero.NewBasePathFs(afero.NewOsFs(), cacheFolder),
			afero.NewMemMapFs(),
			time.Duration(config.CacheExpiresAfter))
		logger.Printf("Cache in-memory expiration after %v", time.Duration(config.CacheExpiresAfter))
	}

	gallery.Initialize(&config, logger)

	// Routing
	httpmux := http.NewServeMux()
	if config.Prefix != "" {
		urlPrefix = fmt.Sprintf("/%s", strings.Trim(config.Prefix, "/"))
		httpmux.Handle(urlPrefix, http.StripPrefix(urlPrefix, http.HandlerFunc(httpHandler)))
	}
	httpmux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderEmbeddedFile("res/favicon.ico", "", w, r)
	}))
	httpmux.Handle("/", http.HandlerFunc(httpHandler))
	bind := fmt.Sprintf("%s:%d", config.Host, config.Port)

	if config.Ffmpeg == "" {
		if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
			config.Ffmpeg = ffmpegPath
			logger.Printf("Found ffmpeg at: %v", ffmpegPath)
		}
	}

	// Server start sequence
	useTls := false
	if fileExists(config.TlsCrt) && fileExists(config.TlsKey) {
		useTls = true
		logger.Printf("Using certificate: %s and key: %s", config.TlsCrt, config.TlsKey)
	}
	if config.DiscordWebhook != "" { // Start filesystem watcher
		go gallery.StartFsWatcher()
	}
	var srvErr error
	defer func() {
		if srvErr != nil {
			log.Fatal(srvErr)
		}
	}()
	if config.PublicHost != "" {
		config.PublicUrl = strings.Trim(config.PublicHost, "/") + urlPrefix + "/"
	} else {
		config.PublicUrl = bind + urlPrefix + "/"
	}
	if useTls { // Prepare the TLS
		tlsConfig := &tls.Config{}

		// Use separate certificate pool to avoid warnings with self-signed certs
		caCertPool := x509.NewCertPool()
		pem, _ := ioutil.ReadFile(config.TlsCrt)
		caCertPool.AppendCertsFromPEM(pem)
		tlsConfig.RootCAs = caCertPool

		// Optional http2
		if config.Http2 {
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
		config.PublicUrl = "https://" + config.PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, config.PublicUrl)
		if !config.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, config.PublicUrl)
		}
		srvErr = srv.ListenAndServeTLS(config.TlsCrt, config.TlsKey)
	} else { // Normal start
		config.PublicUrl = "http://" + config.PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, config.PublicUrl)
		if !config.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, config.PublicUrl)
		}
		srvErr = http.ListenAndServe(bind, httpmux)
	}
}
