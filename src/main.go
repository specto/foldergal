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
	"path/filepath"
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
	logger       *log.Logger
	root         string
	prefix       string
	rootFs       afero.Fs
	cacheFs      afero.Fs
	urlPrefix    = "/"
	BuildVersion = "dev"
	BuildTime    = "now"
)

func fail500(w http.ResponseWriter, err error, _ *http.Request) {
	logger.Print(err)
	http.Error(w, "500 internal server error", http.StatusInternalServerError)
}

// Serve image previews of media files
func previewHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	ext := filepath.Ext(fullPath)
	contentType := mime.TypeByExtension(ext)
	var f media
	// All thumbnails are jpeg, except when they are not...
	thumbPath := strings.TrimSuffix(fullPath, filepath.Ext(fullPath)) + ".jpg"

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
		embeddedFileHandler(w, r, brokenImage, "")
		return
	}
	if !f.fileExists() {
		embeddedFileHandler(w, r, brokenImage, "")
		return
	}
	if f.thumbExpired() {
		err := f.thumbGenerate()
		if err != nil {
			logger.Print(err)
			embeddedFileHandler(w, r, brokenImage, "")
			return
		}
	}
	thumb := f.thumb()
	if thumb == nil || *thumb == nil {
		embeddedFileHandler(w, r, brokenImage, "")
		return
	}
	thP := f.media().thumbPath
	thT := f.media().thumbInfo.ModTime()
	http.ServeContent(w, r, thP, thT, *thumb)
}

func splitUrlToBreadCrumbs(pageUrl *url.URL) (crumbs []templates.BreadCrumb) {
	deepcrumb := "/" + prefix + "/"
	crumbs = append(crumbs, templates.BreadCrumb{Url:deepcrumb, Title:"#:\\"})
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


// Prepare list of files
func listHandler(w http.ResponseWriter, r *http.Request) {
	var (
		parentUrl string
		title     string
		err       error
		contents  []os.FileInfo
	)
	fullPath := filepath.Join(root, r.URL.Path)
	contents, err = afero.ReadDir(rootFs, fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	if fullPath != root {
		title = filepath.Base(r.URL.Path)
		parentUrl = filepath.Join(urlPrefix, r.URL.Path, "..")
	}
	children := make([]templates.ListItem, 0, len(contents))
	for _, child := range contents {
		if containsDotFile(child.Name()) {
			continue
		}
		if !child.IsDir() && !validMediaByExtension(child.Name()) {
			continue
		}
		childPath, _ := filepath.Rel(root, filepath.Join(fullPath, child.Name()))
		childPath = url.PathEscape(childPath)
		thumb := "/go?folder"
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
			Prefix:       urlPrefix,
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
	fullPath := filepath.Join(root, r.URL.Path)
	thumbPath := strings.TrimSuffix(fullPath, filepath.Ext(fullPath)) + ".jpg"
	var err error
	m := mediaFile{
		fullPath:  fullPath,
		thumbPath: thumbPath,
	}
	m.fileInfo, err = rootFs.Stat(fullPath)
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
	fullPath := filepath.Join(root, r.URL.Path)
	//logger.Printf("URL: %v", r.URL)
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
		}
		embeddedFileHandler(w, r, embeddedFile, contentType)
		return
	}

	stat, err := rootFs.Stat(fullPath)
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
	defaultCacheMinutes, _ := strconv.Atoi(getEnvWithDefault("FOLDERGAL_CACHE_MINUTES", "720"))

	// Command line arguments (they override env)
	host := *flag.String("host", defaultHost, "host address to bind to")
	port := *flag.Int("port", defaultPort, "port to run at")
	home := *flag.String("home", defaultHome, "home folder")
	root = *flag.String("root", defaultRoot, "root folder to serve files from")
	prefix = *flag.String("prefix", defaultPrefix, "path prefix as in http://localhost/PREFIX/other/stuff")
	tlsCrt := *flag.String("crt", defaultCrt, "certificate file for TLS")
	tlsKey := *flag.String("key", defaultKey, "key file for TLS")
	useHttp2 := *flag.Bool("http2", defaultHttp2, "enable HTTP/2 (only with TLS)")
	cacheMinutesInt := *flag.Int("cache-minutes", defaultCacheMinutes, "minutes to keep cached resources in memory")
	cacheMinutes := time.Duration(cacheMinutesInt) * time.Minute
	flag.Parse()

	// Check keys to enable TLS
	useTls := false
	if tlsCrt == "" {
		tlsCrt = filepath.Join(home, "tls/server.crt")
	}
	if tlsKey == "" {
		tlsKey = filepath.Join(home, "tls/server.key")
	}
	if fileExists(tlsCrt) && fileExists(tlsKey) {
		useTls = true
	}

	// Set up log file
	logFile, err := os.OpenFile(filepath.Join(home, "foldergal.log"),
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Print("Error: Log file cannot be created in home directory.")
		log.Fatal(err)
	}
	defer logFile.Close()
	logger = log.New(logFile, "foldergal: ", log.Lshortfile|log.LstdFlags)
	//logger.Printf("Env is: %v", os.Environ())
	logger.Printf("Home folder is: %s", home)
	logger.Printf("Root folder is: %s", root)

	// Set up caching folder
	logger.Printf("Setting cache timeout to: %d minutes", cacheMinutesInt)
	cacheFolder := filepath.Join(home, "cache")
	err = os.MkdirAll(cacheFolder, 0750)
	if err != nil {
		log.Fatal(err)
	} else {
		base := afero.NewBasePathFs(afero.NewOsFs(), cacheFolder)
		layer := afero.NewMemMapFs()
		cacheFs = afero.NewCacheOnReadFs(base, layer, cacheMinutes)
	}

	// Set root media folder
	//rootFs := filteredFileSystem{http.Dir(root)}
	base := afero.NewOsFs()
	layer := afero.NewMemMapFs()
	rootFs = afero.NewCacheOnReadFs(base, layer, cacheMinutes)
	//httpFs = afero.NewHttpFs(rootFs)
	//srvFs := filteredFileSystem{afs.Dir(root)}

	// Routing
	httpmux := http.NewServeMux()
	if prefix != "" {
		prefixPath := fmt.Sprintf("/%s/", prefix)
		urlPrefix = prefixPath
		logger.Printf("Using prefix: %s", prefixPath)
		httpmux.Handle(prefixPath, http.StripPrefix(prefixPath, http.HandlerFunc(httpHandler)))
	}
	bind := fmt.Sprintf("%s:%d", host, port)
	httpmux.Handle("/", http.HandlerFunc(httpHandler))

	// Server start sequence
	if port == 0 {
		log.Fatalf("Error: misconfigured port %d", port)
	}
	var srvErr error
	if useTls {
		tlsConfig := &tls.Config{}

		// Use separate certificate pool to avoid warnings with self-signed certs
		caCertPool := x509.NewCertPool()
		pem, _ := ioutil.ReadFile(tlsCrt)
		caCertPool.AppendCertsFromPEM(pem)
		tlsConfig.RootCAs = caCertPool

		if useHttp2 {
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
		logger.Printf("Using certificate: %s and key: %s", tlsCrt, tlsKey)
		logger.Printf("Running at https://%v", bind)
		srvErr = srv.ListenAndServeTLS(tlsCrt, tlsKey)
	} else {
		logger.Printf("Running at http://%v", bind)
		srvErr = http.ListenAndServe(bind, httpmux)
	}
	defer log.Fatal(srvErr)
}
