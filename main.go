package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"foldergal/embedded"
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

//go:generate go run embedded/embed.go

func fileExists(filename string) bool {
	if file, err := os.Stat(filename); os.IsNotExist(err) || file.IsDir() {
		return false
	}
	return true
}

var (
	logger          *log.Logger
	Config          configuration
	CacheFolder     string
	CacheFs         afero.Fs
	RootFs          afero.Fs
	PublicUrl       string
	cacheFolderName = "_foldergal_cache"
	UrlPrefix       = ""
	BuildVersion    = "dev"
	BuildTimestamp  = "now"
	BuildTime       time.Time
	startTime       time.Time
)

type configuration struct {
	Host              string
	Port              int
	Home              string
	Root              string
	Prefix            string
	TlsCrt            string
	TlsKey            string
	Http2             bool
	CacheExpiresAfter jsonDuration
	NotifyAfter       jsonDuration
	DiscordWebhook    string
	PublicHost        string
	Quiet             bool
	Ffmpeg            string
}

func (c *configuration) FromFile(configFile string) (err error) {
	if !fileExists(configFile) {
		return errors.New("missing " + configFile)
	}
	var file *os.File
	/* #nosec */
	if file, err = os.Open(configFile); err != nil {
		return
	}
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&c)
	return
}

type jsonDuration time.Duration

func (d jsonDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *jsonDuration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case float64:
		*d = jsonDuration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = jsonDuration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

func fail404(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNotFound)
	page := templates.ErrorPage{
		Page: templates.Page{
			Title:        "404 not found",
			Prefix:       UrlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		Message: r.URL.String(),
	}
	_ = templates.Tpl.ExecuteTemplate(w, "error", &page)
}

func fail500(w http.ResponseWriter, err error, _ *http.Request) {
	logger.Print(err)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusInternalServerError)
	page := templates.ErrorPage{
		Page: templates.Page{
			Title:        "500 internal server error",
			Prefix:       UrlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		Message: "see the logs for error details",
	}
	_ = templates.Tpl.ExecuteTemplate(w, "error", &page)
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

// Serve image previews of media files
func previewHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, UrlPrefix)
	ext := filepath.Ext(fullPath)
	mimeType := mime.TypeByExtension(ext)
	var f media
	// All thumbnails are jpeg... most of the time
	thumbPath := sanitizePath(fullPath) + ".jpg"
	contentType := "image/jpeg"

	if strings.HasPrefix(mimeType, "image/svg") {
		contentType = mimeType
		thumbPath = fullPath // SVG are their own thumbnails
		f = &svgFile{mediaFile{fullPath: fullPath, thumbPath: thumbPath}}
	} else if strings.HasPrefix(mimeType, "image/") {
		f = &imageFile{mediaFile{fullPath: fullPath, thumbPath: thumbPath}}
	} else if strings.HasPrefix(mimeType, "audio/") {
		contentType = "image/svg+xml"
		f = &audioFile{mediaFile{fullPath: fullPath}}
	} else if strings.HasPrefix(mimeType, "video/") {
		contentType = "image/svg+xml"
		f = &videoFile{mediaFile{fullPath: fullPath, thumbPath: thumbPath}}
	} else if strings.HasPrefix(mimeType, "application/pdf") {
		contentType = "image/svg+xml"
		f = &pdfFile{mediaFile{fullPath: fullPath}}
	} else {
		renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
		return
	}
	if !f.fileExists() {
		renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
		return
	}
	if f.thumbExpired() {
		err := f.thumbGenerate()
		if err != nil {
			logger.Print(err)
			renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
			return
		}
	}
	thumb := f.thumb()
	if thumb == nil || *thumb == nil {
		renderEmbeddedFile("res/broken.svg", "image/svg+xml", w, r)
		return
	}
	if !strings.HasSuffix(f.media().thumbInfo.Name(), ".jpg") {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, f.media().thumbPath, f.media().thumbInfo.ModTime(), *thumb)
}

func splitUrlToBreadCrumbs(pageUrl *url.URL) (crumbs []templates.BreadCrumb) {
	deepcrumb := UrlPrefix + "/"
	currentUrl := strings.TrimPrefix(pageUrl.Path, UrlPrefix)
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
	_, _ = fmt.Fprintf(w, "Root:        %v\n", Config.Root)
	_, _ = fmt.Fprintf(w, "Root size:   %v MiB\n", bToMb(uint64(folderSize(Config.Root))))
	_, _ = fmt.Fprintf(w, "Cache:       %v\n", CacheFolder)
	_, _ = fmt.Fprintf(w, "Cache size:  %v MiB\n", bToMb(uint64(folderSize(CacheFolder))))
	_, _ = fmt.Fprintf(w, "FolderWatch: %v\n", watchedFolders)
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
	if containsDotFile(r.URL.Path) {
		fail404(w, r)
		return
	}
	var (
		parentUrl string
		title     string
		err       error
		contents  []os.FileInfo
	)
	folderPath := strings.TrimPrefix(r.URL.Path, UrlPrefix)
	contents, err = afero.ReadDir(RootFs, folderPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	folderInfo, _ := RootFs.Stat(folderPath)
	if folderPath != "/" && folderPath != "" {
		title = filepath.Base(r.URL.Path)
		parentUrl = path.Join(r.URL.Path, "..")
	} else if Config.PublicHost != "" {
		title = Config.PublicHost
	}
	children := make([]templates.ListItem, 0, len(contents))
	for _, child := range contents {
		if containsDotFile(child.Name()) {
			continue
		}
		mediaClass := getMediaClass(child.Name())
		if !child.IsDir() && mediaClass == "" {
			continue
		}
		childPath := filepath.Join(UrlPrefix, folderPath, child.Name())
		childPath = filepath.ToSlash(childPath)
		thumb := UrlPrefix + "/static?folder"
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
			W:       ThumbWidth,
			H:       ThumbHeight,
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
			Prefix:       UrlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		BreadCrumbs: crumbs,
		ItemCount:   itemCount,
		SortedBy:    sortBy,
		ParentUrl:   parentUrl,
		Items:       children,
	}
	err = templates.Tpl.ExecuteTemplate(w, "layout", &list)
	if err != nil {
		fail500(w, err, r)
		return
	}
}

// Serve actual files
func fileHandler(w http.ResponseWriter, r *http.Request) {
	if containsDotFile(r.URL.Path) {
		fail404(w, r)
		return
	}
	fullPath := strings.TrimPrefix(r.URL.Path, UrlPrefix)
	thumbPath := strings.TrimSuffix(fullPath, filepath.Ext(fullPath)) + ".jpg"
	var err error
	m := mediaFile{
		fullPath:  fullPath,
		thumbPath: thumbPath,
	}
	m.fileInfo, err = RootFs.Stat(fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	if m.fileInfo.IsDir() || !validMedia(fullPath) {
		fail404(w, r)
		return
	}
	contents := m.file()
	if contents == nil {
		fail404(w, r)
		return
	}
	http.ServeContent(w, r, fullPath, m.fileInfo.ModTime(), *contents)
}

func renderEmbeddedFile(resFile string, contentType string,
	w http.ResponseWriter, r *http.Request) {
	f, err := embedded.Fs.Open(resFile)
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
// Since we are mapping URLs to file system resources we cannot use any names
// for our internal resources.
//
// Three types of content are served:
//    * internal resource (image, css, etc.)
//    * html to show folder lists
//    * media file (thumbnail or larger file)
func httpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, UrlPrefix)
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

	stat, err := RootFs.Stat(fullPath)
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
	if Config.Host = os.Getenv("FOLDERGAL_HOST"); Config.Host == "" {
		Config.Host = "localhost"
	}
	if Config.Port, _ = strconv.Atoi(os.Getenv("FOLDERGAL_HOST")); Config.Port == 0 {
		Config.Port = 8080
	}
	if Config.Home = os.Getenv("FOLDERGAL_HOME"); Config.Home == "" {
		Config.Home = execFolder
	}
	if Config.Root = os.Getenv("FOLDERGAL_ROOT"); Config.Root == "" {
		Config.Root = execFolder
	}
	Config.Prefix = os.Getenv("FOLDERGAL_PREFIX")
	Config.TlsCrt = os.Getenv("FOLDERGAL_CRT")
	Config.TlsKey = os.Getenv("FOLDERGAL_KEY")
	Config.Http2, _ = strconv.ParseBool(os.Getenv("FOLDERGAL_HTTP2"))
	if envValue := os.Getenv("FOLDERGAL_CACHE_EXPIRES_AFTER"); envValue != "" {
		envCacheExpires, _ := time.ParseDuration(envValue)
		Config.CacheExpiresAfter = jsonDuration(envCacheExpires)
	}
	if envValue := os.Getenv("FOLDERGAL_NOTIFY_AFTER"); envValue != "" {
		envNotifyAfter, _ := time.ParseDuration(envValue)
		Config.NotifyAfter = jsonDuration(envNotifyAfter)
	} else {
		Config.NotifyAfter = jsonDuration(30 * time.Second)
	}
	Config.DiscordWebhook = os.Getenv("FOLDERGAL_DISCORD_WEBHOOK")
	Config.PublicHost = os.Getenv("FOLDERGAL_PUBLIC_HOST")
	Config.Quiet, _ = strconv.ParseBool(os.Getenv("FOLDERGAL_QUIET"))

	// Command line arguments (they override env)
	flag.StringVar(&Config.Host, "host", Config.Host, "host address to bind to")
	flag.IntVar(&Config.Port, "port", Config.Port, "port to run at")
	flag.StringVar(&Config.Home, "home", Config.Home, "home folder e.g. to keep thumbnails")
	flag.StringVar(&Config.Root, "root", Config.Root, "root folder to serve files from")
	flag.StringVar(&Config.Prefix, "prefix", Config.Prefix,
		"path prefix as in http://localhost/PREFIX/other/stuff")
	flag.StringVar(&Config.TlsCrt, "crt", Config.TlsCrt, "certificate file for TLS")
	flag.StringVar(&Config.TlsKey, "key", Config.TlsKey, "key file for TLS")
	flag.BoolVar(&Config.Http2, "http2", Config.Http2, "enable HTTP/2 (only with TLS)")
	flag.BoolVar(&Config.Quiet, "quiet", Config.Quiet, "don't print to console")
	flag.DurationVar((*time.Duration)(&Config.CacheExpiresAfter), "cache-expires-after",
		time.Duration(Config.CacheExpiresAfter),
		"duration to keep cached resources in memory")
	flag.DurationVar((*time.Duration)(&Config.NotifyAfter), "notify-after",
		time.Duration(Config.NotifyAfter),
		"duration to delay notifications and combine them in one")
	flag.StringVar(&Config.DiscordWebhook, "discord", Config.DiscordWebhook,
		"webhook URL to receive notifications when new media appears")
	flag.StringVar(&Config.PublicHost, "pub-host", Config.PublicHost,
		"the public name for the machine")

	// The following order is important
	embedded.Intialize()
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
	if err := Config.FromFile(*configFile); *configFile != "" && err != nil {
		log.Fatalf("error: invalid config %v", err)
	}
	Config.Home, _ = filepath.Abs(Config.Home)
	Config.Root, _ = filepath.Abs(Config.Root)

	// Set up log file
	logFile := filepath.Join(Config.Home, "foldergal.log")
	logging, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0644) // #nosec Permit everybody to read the log file
	if err != nil {
		log.Print("Error: Log file cannot be created.")
		log.Fatal(err)
	}
	defer func() { _ = logging.Close() }()
	logger = log.New(logging, "foldergal: ", log.Lshortfile|log.LstdFlags)
	if !Config.Quiet {
		log.Printf("Logging to %s", logFile)
	}

	// Set root media folder
	if exists, err := os.Stat(Config.Root); os.IsNotExist(err) || !exists.IsDir() {
		log.Fatalf("Root folder does not exist: %v", Config.Root)
	}
	logger.Printf("Root folder is: %s", Config.Root)
	if !Config.Quiet {
		log.Printf("Serving files from: %v", Config.Root)
	}
	if Config.CacheExpiresAfter == 0 {
		RootFs = afero.NewReadOnlyFs(afero.NewBasePathFs(afero.NewOsFs(), Config.Root))
	} else {
		base := afero.NewReadOnlyFs(afero.NewBasePathFs(afero.NewOsFs(), Config.Root))
		layer := afero.NewMemMapFs()
		RootFs = afero.NewCacheOnReadFs(base, layer, time.Duration(Config.CacheExpiresAfter))
	}

	//stat, _ := embedded.Fs.Stat("asdf.svg")
	//fmt.Printf("%v\n", stat.Size())

	// Set up caching folder
	CacheFolder = filepath.Join(Config.Home, cacheFolderName)
	err = os.MkdirAll(CacheFolder, 0750)
	if err != nil {
		log.Fatal(err)
	}
	if !Config.Quiet {
		log.Printf("Cache folder is: %s\n", CacheFolder)
	}
	logger.Printf("Cache folder is: %s", CacheFolder)
	if Config.CacheExpiresAfter == 0 {
		CacheFs = afero.NewBasePathFs(afero.NewOsFs(), CacheFolder)
	} else {
		base := afero.NewBasePathFs(afero.NewOsFs(), CacheFolder)
		layer := afero.NewMemMapFs()
		CacheFs = afero.NewCacheOnReadFs(base, layer, time.Duration(Config.CacheExpiresAfter))
		logger.Printf("Cache in-memory expiration after %v", time.Duration(Config.CacheExpiresAfter))
	}

	// Routing
	httpmux := http.NewServeMux()
	if Config.Prefix != "" {
		UrlPrefix = fmt.Sprintf("/%s", strings.Trim(Config.Prefix, "/"))
		httpmux.Handle(UrlPrefix, http.StripPrefix(UrlPrefix, http.HandlerFunc(httpHandler)))
	}
	httpmux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renderEmbeddedFile("res/favicon.ico", "", w, r)
	}))
	httpmux.Handle("/", http.HandlerFunc(httpHandler))
	bind := fmt.Sprintf("%s:%d", Config.Host, Config.Port)

	if Config.Ffmpeg == "" {
		if ffmpegPath, err := exec.LookPath("ffmpeg"); err == nil {
			Config.Ffmpeg = ffmpegPath
			logger.Printf("Found ffmpeg at: %v", ffmpegPath)
		}
	}

	// Server start sequence
	useTls := false
	if fileExists(Config.TlsCrt) && fileExists(Config.TlsKey) {
		useTls = true
		logger.Printf("Using certificate: %s and key: %s", Config.TlsCrt, Config.TlsKey)
	}
	go startFsWatcher(Config.Root) // Start filesystem watcher
	var srvErr error
	defer func() {
		if srvErr != nil {
			log.Fatal(srvErr)
		}
	}()
	if Config.PublicHost != "" {
		PublicUrl = strings.Trim(Config.PublicHost, "/") + UrlPrefix + "/"
	} else {
		PublicUrl = bind + UrlPrefix + "/"
	}
	if useTls { // Prepare the TLS
		tlsConfig := &tls.Config{}

		// Use separate certificate pool to avoid warnings with self-signed certs
		caCertPool := x509.NewCertPool()
		pem, _ := ioutil.ReadFile(Config.TlsCrt)
		caCertPool.AppendCertsFromPEM(pem)
		tlsConfig.RootCAs = caCertPool

		// Optional http2
		if Config.Http2 {
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
		PublicUrl = "https://" + PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, PublicUrl)
		if !Config.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, PublicUrl)
		}
		srvErr = srv.ListenAndServeTLS(Config.TlsCrt, Config.TlsKey)
	} else { // Normal start
		PublicUrl = "http://" + PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, PublicUrl)
		if !Config.Quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, PublicUrl)
		}
		srvErr = http.ListenAndServe(bind, httpmux)
	}
}