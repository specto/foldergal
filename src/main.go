package main

import (
	"./templates"
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"github.com/spf13/afero"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

func getEnvWithDefault(key string, defaultValue string) string {
	if envValue := os.Getenv(key); envValue != "" {
		return envValue
	} else {
		return defaultValue
	}
}

var (
	logger          *log.Logger
	RootFolder      string
	cacheFolder     string
	RootFs          afero.Fs
	CacheFs         afero.Fs
	PublicUrl       string
	cacheFolderName = "_foldergal_cache"
	UrlPrefix       = "/"
	BuildVersion    = "dev"
	BuildTime       = "now"
	startTime       time.Time
)

func fail500(w http.ResponseWriter, err error, _ *http.Request) {
	logger.Print(err)
	http.Error(w, "500 internal server error", http.StatusInternalServerError)
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
	fullPath := filepath.Join(RootFolder, r.URL.Path)
	ext := filepath.Ext(fullPath)
	contentType := mime.TypeByExtension(ext)
	var f media
	// All thumbnails are jpeg, except when they are not...
	thumbPath := strings.TrimSuffix(sanitizePath(fullPath), filepath.Ext(fullPath)) + ".jpg"

	if strings.HasPrefix(contentType, "image/svg") {
		w.Header().Set("Content-Type", contentType)
		f = &svgFile{mediaFile{fullPath: fullPath, thumbPath: thumbPath}}
	} else if strings.HasPrefix(contentType, "image/") {
		f = &imageFile{mediaFile{fullPath: fullPath, thumbPath: thumbPath}}
	} else if strings.HasPrefix(contentType, "audio/") {
		w.Header().Set("Content-Type", "image/svg+xml")
		f = &audioFile{mediaFile{fullPath: fullPath}}
	} else if strings.HasPrefix(contentType, "video/") {
		w.Header().Set("Content-Type", "image/svg+xml")
		f = &videoFile{mediaFile{fullPath: fullPath}}
	} else if strings.HasPrefix(contentType, "application/pdf") {
		w.Header().Set("Content-Type", "image/svg+xml")
		f = &pdfFile{mediaFile{fullPath: fullPath}}
	} else {
		embeddedFileHandler(w, r, brokenImage, "image/svg+xml")
		return
	}
	if !f.fileExists() {
		embeddedFileHandler(w, r, brokenImage, "image/svg+xml")
		return
	}
	if f.thumbExpired() {
		err := f.thumbGenerate()
		if err != nil {
			logger.Print(err)
			embeddedFileHandler(w, r, brokenImage, "image/svg+xml")
			return
		}
	}
	thumb := f.thumb()
	if thumb == nil || *thumb == nil {
		embeddedFileHandler(w, r, brokenImage, "image/svg+xml")
		return
	}
	thP := f.media().thumbPath
	thT := f.media().thumbInfo.ModTime()
	http.ServeContent(w, r, thP, thT, *thumb)
}

func splitUrlToBreadCrumbs(pageUrl *url.URL) (crumbs []templates.BreadCrumb) {
	deepcrumb := UrlPrefix
	crumbs = append(crumbs, templates.BreadCrumb{Url: deepcrumb, Title: "#:\\"})
	enslavedPath, _ := url.PathUnescape(pageUrl.Path)
	for _, br := range strings.Split(enslavedPath, "/") {
		if br == "" {
			continue
		}
		crumbs = append(crumbs, templates.BreadCrumb{Url: deepcrumb + br, Title: br})
		deepcrumb += br + "/"
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
	_, _ = fmt.Fprintf(w, "Root:        %v\n", RootFolder)
	_, _ = fmt.Fprintf(w, "Root size:   %v MiB\n", bToMb(uint64(folderSize(RootFolder))))
	_, _ = fmt.Fprintf(w, "Cache:       %v\n", cacheFolder)
	_, _ = fmt.Fprintf(w, "Cache size:  %v MiB\n", bToMb(uint64(folderSize(cacheFolder))))
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
func listHandler(w http.ResponseWriter, r *http.Request) {
	var (
		parentUrl string
		title     string
		err       error
		contents  []os.FileInfo
	)
	fullPath := filepath.Join(RootFolder, r.URL.Path)
	contents, err = afero.ReadDir(RootFs, fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	if fullPath != RootFolder {
		title = filepath.Base(r.URL.Path)
		parentUrl = path.Join(UrlPrefix, r.URL.Path, "..")
	}
	children := make([]templates.ListItem, 0, len(contents))
	for _, child := range contents {
		if containsDotFile(child.Name()) {
			continue
		}
		if !child.IsDir() && !validMediaByExtension(child.Name()) {
			continue
		}
		childPath, _ := filepath.Rel(RootFolder, filepath.Join(fullPath, child.Name()))
		childPath = filepath.ToSlash(childPath)
		thumb := "go?folder"
		if !child.IsDir() {
			thumb = fmt.Sprintf("%s?thumb", childPath)
		}
		li := templates.ListItem{
			Url:   childPath,
			Name:  child.Name(),
			Thumb: thumb,
			W:     ThumbWidth,
			H:     ThumbHeight,
		}
		children = append(children, li)
	}
	crumbs := splitUrlToBreadCrumbs(r.URL)
	err = templates.ListTpl.ExecuteTemplate(w, "layout", &templates.List{
		Page: templates.Page{
			Title:        title,
			Prefix:       UrlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTime,
			BreadCrumbs:  crumbs,
		},
		ParentUrl: parentUrl,
		Items:     children,
	})
	if err != nil {
		fail500(w, err, r)
		return
	}
}

// Serve actual files
func fileHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(RootFolder, r.URL.Path)
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
	contents := m.file()
	if contents == nil {
		http.NotFound(w, r)
		return
	}
	http.ServeContent(w, r, fullPath, m.fileInfo.ModTime(), *contents)
}

func embeddedFileHandler(w http.ResponseWriter, r *http.Request, id embeddedFileId, forceContentType string) {
	if forceContentType != "" {
		w.Header().Set("Content-Type", forceContentType)
	}
	http.ServeContent(w, r, r.URL.Path, time.Now(), bytes.NewReader(embeddedFiles[id]))
}

// Elaborate router
func httpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(RootFolder, r.URL.Path)
	q := r.URL.Query()
	if _, ok := q["thumb"]; ok { // Thumbnails are marked with &thumb in the query string
		previewHandler(w, r)
		return
	} else if len(q) > 0 {
		var embeddedFile embeddedFileId
		contentType := "image/svg+xml"
		if _, ok := q["broken"]; ok {
			embeddedFile = brokenImage
		} else if _, ok := q["up"]; ok {
			embeddedFile = upImage
		} else if _, ok := q["folder"]; ok {
			embeddedFile = folderImage
		} else if _, ok := q["favicon"]; ok {
			embeddedFile = faviconImage
			contentType = "" // Expecting ServeContent to put the correct image/x-icon
		} else if _, ok := q["status"]; ok {
			statusHandler(w, r)
			return
		}
		embeddedFileHandler(w, r, embeddedFile, contentType)
		return
	}

	stat, err := RootFs.Stat(fullPath)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if stat.IsDir() { // Prepare and render folder contents
		listHandler(w, r)
	} else { // This is a media file and we should serve it in all it's glory
		fileHandler(w, r)
	}
}

func init() {
	startTime = time.Now()
}

func main() {
	// Get current execution folder
	execFolder, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	// Environment variables
	defaultHost := getEnvWithDefault("FOLDERGAL_HOST", "localhost")
	defaultPort, _ := strconv.Atoi(getEnvWithDefault("FOLDERGAL_PORT", "8080"))
	defaultHome := getEnvWithDefault("FOLDERGAL_HOME", execFolder)
	defaultRoot := getEnvWithDefault("FOLDERGAL_ROOT", execFolder)
	defaultPrefix := getEnvWithDefault("FOLDERGAL_PREFIX", "")
	defaultCrt := getEnvWithDefault("FOLDERGAL_CRT", "")
	defaultKey := getEnvWithDefault("FOLDERGAL_KEY", "")
	defaultHttp2, _ := strconv.ParseBool(getEnvWithDefault("FOLDERGAL_HTTP2", ""))
	defaultCacheExpires, _ := time.ParseDuration(getEnvWithDefault(
		"FOLDERGAL_CACHE_EXPIRES_AFTER", "6h"))
	defaultDiscordWebhook := getEnvWithDefault("FOLDERGAL_DISCORD_WEBHOOK", "")
	defaultPublicHost := getEnvWithDefault("FOLDERGAL_PUBLIC_HOST", "")
	defaultQuiet, _ := strconv.ParseBool(getEnvWithDefault("FOLDERGAL_QUIET", ""))

	// Command line arguments (they override env)
	host := flag.String("host", defaultHost, "host address to bind to")
	port := flag.Int("port", defaultPort, "port to run at")
	home := flag.String("home", defaultHome, "home folder e.g. to keep thumbnails")
	root := flag.String("root", defaultRoot, "root folder to serve files from")
	prefix := flag.String("prefix", defaultPrefix,
		"path prefix as in http://localhost/PREFIX/other/stuff")
	tlsCrt := flag.String("crt", defaultCrt, "certificate file for TLS")
	tlsKey := flag.String("key", defaultKey, "key file for TLS")
	useHttp2 := flag.Bool("http2", defaultHttp2, "enable HTTP/2 (only with TLS)")
	quiet := flag.Bool("quiet", defaultQuiet, "don't print to console")
	cacheExpires := flag.Duration("cache-expires-after",
		defaultCacheExpires,
		"duration to keep cached resources in memory")
	discordWebhook := flag.String("discord", defaultDiscordWebhook,
		"webhook URL to receive notifications when new media appears")
	publicHost := flag.String("pub-host", defaultPublicHost,
		"the public name for the machine")

	flag.Parse()

	////////////////////////////////////////////////////////////////////////////

	// Set up log file
	logFile := filepath.Join(*home, "foldergal.log")
	logging, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Print("Error: Log file cannot be created in home directory.")
		log.Fatal(err)
	}
	defer func() {
		_ = logging.Close()
	}()
	logger = log.New(logging, "foldergal: ", log.Lshortfile|log.LstdFlags)
	if !*quiet {
		log.Printf("Logging to %s", logFile)
	}

	// Set root media folder
	RootFolder = *root
	logger.Printf("Root folder is: %s", RootFolder)
	if !*quiet {
		log.Printf("Serving files from: %v", RootFolder)
	}
	base := afero.NewOsFs()
	layer := afero.NewMemMapFs()
	RootFs = afero.NewCacheOnReadFs(base, layer, *cacheExpires)

	// Set up caching folder
	cacheFolder = filepath.Join(*home, cacheFolderName)
	err = os.MkdirAll(cacheFolder, 0750)
	if err != nil {
		log.Fatal(err)
	} else {
		if !*quiet {
			log.Printf("Created cache folder: %s\n", cacheFolder)
		}
		logger.Printf("Cache folder is: %s", cacheFolder)
		base := afero.NewBasePathFs(afero.NewOsFs(), cacheFolder)
		layer := afero.NewMemMapFs()
		CacheFs = afero.NewCacheOnReadFs(base, layer, *cacheExpires)
	}
	logger.Printf("Cache in-memory expiration after %v", *cacheExpires)

	// Routing
	httpmux := http.NewServeMux()
	if *prefix != "" {
		UrlPrefix = fmt.Sprintf("/%s/", strings.Trim(*prefix, "/"))
		httpmux.Handle(UrlPrefix, http.StripPrefix(UrlPrefix, http.HandlerFunc(httpHandler)))
	}
	bind := fmt.Sprintf("%s:%d", *host, *port)
	httpmux.Handle("/", http.HandlerFunc(httpHandler))

	// Server start sequence
	if *port == 0 {
		log.Fatalf("Error: misconfigured port %d", port)
	}

	// Check keys to enable TLS
	useTls := false
	tlsCrtFile := *tlsCrt
	tlsKeyFile := *tlsKey
	if fileExists(tlsCrtFile) && fileExists(tlsKeyFile) {
		useTls = true
		logger.Printf("Using certificate: %s and key: %s", tlsCrtFile, tlsKeyFile)
	}

	// Fire filesystem change monitor
	go startFsWatcher(RootFolder, *discordWebhook)

	// Start the server
	var srvErr error
	defer func() {
		if srvErr != nil {
			log.Fatal(srvErr)
		}
	}()
	if *publicHost != "" {
		PublicUrl = strings.Trim(*publicHost, "/") + UrlPrefix
	} else {
		PublicUrl = bind + UrlPrefix
	}
	if useTls { // Prepare the TLS
		tlsConfig := &tls.Config{}

		// Use separate certificate pool to avoid warnings with self-signed certs
		caCertPool := x509.NewCertPool()
		pem, _ := ioutil.ReadFile(tlsCrtFile)
		caCertPool.AppendCertsFromPEM(pem)
		tlsConfig.RootCAs = caCertPool

		// Optional http2
		if *useHttp2 {
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
		if !*quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, PublicUrl)
		}
		srvErr = srv.ListenAndServeTLS(tlsCrtFile, tlsKeyFile)
	} else { // Normal start
		PublicUrl = "http://" + PublicUrl
		logger.Printf("Running v:%v at %v", BuildVersion, PublicUrl)
		if !*quiet {
			log.Printf("Running v:%v at %v\nPress ^C to stop...\n", BuildVersion, PublicUrl)
		}
		srvErr = http.ListenAndServe(bind, httpmux)
	}
}
