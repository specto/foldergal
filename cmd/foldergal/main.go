package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"specto.org/projects/foldergal/internal/config"
	"specto.org/projects/foldergal/internal/gallery"
	"specto.org/projects/foldergal/internal/storage"
	"specto.org/projects/foldergal/internal/templates"

	"github.com/fvbommel/sortorder"
	"github.com/spf13/afero"
)

type ctxKey string

const (
	reqSettings    ctxKey = "reqSettings"
	folderSettings ctxKey = "folderSettings"
)

type feed string

const (
	feedRss  feed = "rss"
	feedAtom feed = "atom"
)

var (
	BuildVersion   = "dev"
	BuildTimestamp = "now"
	BuildTime      time.Time
)

var (
	logger            *log.Logger
	cacheFolderName   = "foldergal_cache"
	startTime         time.Time
	urlPrefix         string
	feedFreshness     = 2 * 168 * time.Hour // Two weeks
	feedNotFreshCount = 20                  // entries to show in RSS if not fresh
	headerTimeout     = 3 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Verify if a file exists and is not a folder
func fileExists(filename string) bool {
	if file, err := os.Stat(filename); os.IsNotExist(err) || file.IsDir() {
		return false
	}
	return true
}

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

func infoF(format string, args ...any) {
	logger.Printf(format, args...)
	if !config.Global.Quiet {
		log.Printf(format, args...)
	}
}

func fail500(w http.ResponseWriter, err error, _ *http.Request) {
	logger.Print(fmt.Errorf("fail500 error: %w", err))
	w.WriteHeader(http.StatusInternalServerError)
}

// Cleans a path so it does not go up (..) nor does it start with root (/)
func sanitizePath(p string) string {
	clean := filepath.ToSlash(filepath.Clean(p))
	noSubdirs := strings.ReplaceAll(clean, "../", "")
	noRoot := strings.TrimPrefix(noSubdirs, "/")
	noUp := strings.TrimPrefix(noRoot, "..")
	return filepath.ToSlash(filepath.Clean(noUp))
}

// Route for image previews of media files
func previewHandler(w http.ResponseWriter, r *http.Request) {
	var (
		err         error
		file        gallery.Media
		contentType = "image/jpeg"
		fullPath    = strings.TrimPrefix(r.URL.Path, urlPrefix)
		// All thumbnails are jpegs... most of the time
		thumbPath = sanitizePath(fullPath) + ".jpg"
		mediaType = mime.TypeByExtension(filepath.Ext(fullPath))
	)
	switch {
	case strings.HasPrefix(mediaType, "image/svg"):
		contentType = mediaType
		file, err = gallery.NewSvg(fullPath)
	case strings.HasPrefix(mediaType, "image/"):
		file, err = gallery.NewImage(fullPath, thumbPath)
	case strings.HasPrefix(mediaType, "audio/"):
		contentType = "image/svg+xml"
		file, err = gallery.NewAudio(fullPath, thumbPath)
	case strings.HasPrefix(mediaType, "video/"):
		contentType = "image/svg+xml"
		file, err = gallery.NewVideo(fullPath, thumbPath)
	case strings.HasPrefix(mediaType, "application/pdf"):
		contentType = "image/svg+xml"
		file, err = gallery.NewPdf(fullPath, thumbPath)
	default: // Unrecognized mime type
		w.WriteHeader(http.StatusNotFound)
		staticHandler("res/broken.svg", w, r)
		return
	}
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		staticHandler("res/broken.svg", w, r)
		return
	}
	if err := gallery.GenerateThumb(file); err != nil {
		fail500(w, err, r)
		return
	}
	thumb, err := file.Thumb()
	if err != nil {
		if errors.Is(err, gallery.ErrThumbNotPossible) {
			switch {
			case strings.HasPrefix(mediaType, "audio/"):
				staticHandler("res/audio.svg", w, r)
				return
			case strings.HasPrefix(mediaType, "video/"):
				staticHandler("res/video.svg", w, r)
				return
			}
		}
		logger.Print(err)
		w.WriteHeader(http.StatusNotFound)
		staticHandler("res/broken.svg", w, r)
		return
	}
	defer thumb.Close()

	if !strings.HasSuffix(file.ThumbName(), ".jpg") {
		w.Header().Set("Content-Type", contentType)
	}
	http.ServeContent(w, r, file.ThumbPath(), file.ThumbModTime(), thumb)
}

// Splits a url "path" to separate tokens
func splitUrlToBreadCrumbs(pageUrl *url.URL, qs string) (crumbs []templates.BreadCrumb) {
	deepcrumb := urlPrefix + "/"
	currentUrl := strings.TrimPrefix(pageUrl.Path, urlPrefix)
	crumbs = append(crumbs, templates.BreadCrumb{Url: deepcrumb + qs, Title: "#:\\"})
	for name := range strings.SplitSeq(currentUrl, "/") {
		if name == "" {
			continue
		}
		crumbs = append(crumbs,
			templates.BreadCrumb{Url: deepcrumb + name + qs, Title: name})
		deepcrumb += name + "/"
	}
	return
}

// Counts recursively all valid media files in startPath
func mediaCount(startPath string) (totalCount int64) {
	_ = filepath.WalkDir(startPath,
		func(path string, entry os.DirEntry, err error) error {
			if !entry.IsDir() && gallery.IsValidMedia(path) {
				totalCount += 1
			}
			return nil
		})
	return
}

// Retrieves the byte size of media files in startPath
func folderMediaSize(startPath string) (totalSize int64) {
	_ = filepath.WalkDir(startPath,
		func(path string, entry os.DirEntry, err error) error {
			if !entry.IsDir() && gallery.IsValidMedia(path) {
				if info, err1 := entry.Info(); err1 == nil {
					totalSize += info.Size()
				}
			}
			return nil
		})
	return
}

// Route for status report
func statusHandler(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	fileCount := mediaCount(config.Global.Root)
	folderSize := folderMediaSize(config.Global.Root)
	thumbSize := folderMediaSize(config.Global.Cache)
	cacheExpires := time.Duration(config.Global.CacheExpiresAfter).String()
	if config.Global.CacheExpiresAfter == 0 {
		cacheExpires = "cache is disabled"
	}

	rowData := [][2]string{
		{"Total Media Files:", fmt.Sprint(fileCount)},
		{"Media Folder Size:", fmt.Sprintf("%v MiB", folderSize/1024/1024)},
		{"Thumbnail Folder Size:", fmt.Sprintf("%v MiB", thumbSize/1024/1024)},
		{"Folders Watched:", fmt.Sprint(gallery.WatchedFolders)},
		{"Public Url:", config.Global.PublicUrl},
		{"Prefix:", config.Global.Prefix},
		{"Cache Expires After:", cacheExpires},
		{"-", ""},
		{"Alloc Memory:", fmt.Sprintf("%v MiB", m.Alloc/1024/1024)},
		{"Sys Memory:", fmt.Sprintf("%v MiB", m.Sys/1024/1024)},
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
	if err := templates.Html.ExecuteTemplate(w, "table", &page); err != nil {
		fail500(w, err, r)
	}
}

// Route for lists of files
func listHandler(w http.ResponseWriter, r *http.Request) {
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
	opts := r.Context().Value(reqSettings).(config.RequestSettings)
	querystring := opts.QueryString()
	if parentUrl != "" { // parentUrl is empty when visiting the root folder
		parentUrl += querystring
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
		childPath = gallery.EscapePath(childPath)
		thumb := urlPrefix + "/?static/ui.svg#iconFolder"
		class := "folder"
		if !child.IsDir() {
			thumb = gallery.EscapePath(filepath.Join(urlPrefix, folderPath, child.Name())) + "?thumb"
			class = string(mediaClass)
			if config.Global.Ffmpeg == "" {
				class += " nothumb"
			}
		}
		children = append(children, templates.ListItem{
			Id:      gallery.EscapePath(child.Name()),
			ModTime: child.ModTime(),
			Url:     childPath + querystring,
			Name:    child.Name(),
			Thumb:   thumb,
			Class:   class,
			W:       config.Global.ThumbWidth,
			H:       config.Global.ThumbHeight,
		})
	}
	sort.Slice(children,
		itemSorter(children, opts.Sort, opts.Order == config.QueryOrderDesc))
	pUrl, _ := url.Parse(folderPath)
	crumbs := splitUrlToBreadCrumbs(pUrl, querystring)
	w.Header().Set("Date", folderInfo.ModTime().UTC().Format(http.TimeFormat))
	itemCount := ""
	if folderPath != "/" && folderPath != "" && len(children) > 0 {
		itemCount = fmt.Sprintf("%v ", len(children))
	}

	listTpl := templates.List{
		Page: templates.Page{
			Title:        title,
			Prefix:       urlPrefix,
			AppVersion:   BuildVersion,
			AppBuildTime: BuildTimestamp,
		},
		BreadCrumbs:    crumbs,
		ItemCount:      itemCount,
		IsSortedByName: opts.Sort == config.QuerySortName,
		IsReversed:     opts.Order == config.QueryOrderDesc,
		LinkOrderAsc:   opts.WithOrder(config.QueryOrderAsc).QueryFull(),
		LinkOrderDesc:  opts.WithOrder(config.QueryOrderDesc).QueryFull(),
		LinkSortName:   opts.WithSort(config.QuerySortName).QueryFull(),
		LinkSortDate:   opts.WithSort(config.QuerySortDate).QueryFull(),
		ParentUrl:      parentUrl,
		Items:          children,
		Copyright:      config.Global.Copyright,
	}

	metaCtx := r.Context().Value(folderSettings)
	if metaCtx != nil {
		meta := metaCtx.(config.FolderSettings)
		listTpl.Description = meta.Description
		listTpl.Copyright = meta.Copyright
	}

	err = templates.Html.ExecuteTemplate(w, "layout", &listTpl)
	if err != nil {
		fail500(w, err, r)
		return
	}
}

type LessFunc func(i, j int) bool

func reverse(less LessFunc) LessFunc {
	return func(i, j int) bool { return !less(i, j) }
}

func itemSorter(li []templates.ListItem, field config.QTypeSort, rev bool) LessFunc {
	var sorter LessFunc
	switch field {
	case config.QuerySortDate:
		sorter = func(i, j int) bool {
			return li[i].ModTime.Before(li[j].ModTime)
		}
	case config.QuerySortName:
		sorter = func(first, second int) bool {
			return sortorder.NaturalLess(
				strings.ToLower(li[first].Name),
				strings.ToLower(li[second].Name))
		}
	default:
		sorter = func(_, _ int) bool { return true }
	}
	if rev {
		return reverse(sorter)
	}
	return sorter
}

// Serve html containers for media
func viewHandler(w http.ResponseWriter, r *http.Request) {
	if gallery.ContainsDotFile(r.URL.Path) {
		fail404(w, r)
		return
	}

	fullPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	mediaType := gallery.GetMediaClass(filepath.Base(fullPath))
	var templateName string
	switch mediaType {
	case gallery.MediaImage:
		templateName = "view_img"
	case gallery.MediaAudio:
		templateName = "view_audio"
	case gallery.MediaVideo:
		templateName = "view_video"
	case gallery.MediaPdf:
		templateName = "view_pdf"
	default:
		fail500(w, errors.New("unkown media type"), r)
		return
	}

	// Get the parent folder
	parentUrl := path.Join(urlPrefix, fullPath, "..")
	folderPath := strings.TrimPrefix(parentUrl, urlPrefix)
	fs, err := storage.Root.Open(folderPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	defer fs.Close()
	contents, err := fs.Readdir(-1)
	if err != nil {
		fail500(w, err, r)
		return
	}
	opts := r.Context().Value(reqSettings).(config.RequestSettings)
	querystring := opts.QueryString()

	escCurrentMediaPath := gallery.EscapePath(filepath.Join(urlPrefix, fullPath))
	currentMediaPath, _ := url.PathUnescape(escCurrentMediaPath)

	totalItems := 0

	// Collect all children of parent folder
	children := make([]templates.ListItem, 0, len(contents))
	for _, child := range contents {
		if gallery.ContainsDotFile(child.Name()) {
			continue
		}
		// Look only for media items
		if mediaClass := gallery.GetMediaClass(child.Name()); child.IsDir() || (!child.IsDir() && mediaClass == "") {
			continue
		}
		childPath := filepath.Join(urlPrefix, folderPath, child.Name())
		childPath = gallery.EscapePath(childPath)

		// Get total count of items in parent folder
		totalItems += 1
		unescapedPath, _ := url.PathUnescape(childPath)
		children = append(children, templates.ListItem{
			ModTime: child.ModTime(),
			Url:     unescapedPath,
			Name:    child.Name(),
		})
	}

	sort.Slice(children,
		itemSorter(children, opts.Sort, opts.Order == config.QueryOrderDesc))

	// Get previous and next items according to the current sort order
	var lastChild, nextChild templates.ListItem
	for i, child := range children {
		if child.Url == currentMediaPath {
			if i == 0 {
				// No previous child if we are the first one
				lastChild = templates.ListItem{}
			} else {
				lastChild.Url += querystring
			}
			if totalItems > i+1 {
				nextChild = children[i+1]
				nextChild.Url += querystring
			}
			break
		}
		lastChild = child
	}

	var parentName string
	if parentUrl == "/" {
		parentName = "../"
	} else {
		parentName = "../" + filepath.Base(parentUrl)
	}
	err = templates.Html.ExecuteTemplate(w, templateName, &templates.ViewPage{
		Page: templates.Page{
			Title:      escCurrentMediaPath,
			Prefix:     urlPrefix,
			LinkPrev:   string(lastChild.Url),
			LinkNext:   string(nextChild.Url),
			ParentUrl:  parentUrl + querystring + "#" + filepath.Base(escCurrentMediaPath),
			ParentName: parentName,
		},
		MediaPath: fmt.Sprintf("%s?%s/%s",
			escCurrentMediaPath, config.QKeyDisplay, config.QueryDisplayFile),
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
	defer contents.Close()

	http.ServeContent(w, r, fullPath, media.FileModTime(), contents)
}

// Delivers file contents for static resources
func staticHandler(resFile string, w http.ResponseWriter, r *http.Request) {
	staticFile, err := storage.InternalHttp.Open(resFile)
	if err != nil {
		fail404(w, r)
		return
	}
	defer staticFile.Close()
	http.ServeContent(w, r, filepath.Base(resFile), BuildTime, staticFile)
}

// Route for RSS/Atom feed
func feedHandler(feedType feed, w http.ResponseWriter, r *http.Request) {
	loc, _ := time.LoadLocation("UTC")

	var formatTime func(time.Time) string
	if feedType == feedRss {
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

	var feedItems []templates.FeedItem
	err := filepath.WalkDir(config.Global.Root,
		func(walkPath string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() && !gallery.ContainsDotFile(walkPath) &&
				gallery.IsValidMedia(walkPath) {
				if info, err := entry.Info(); err == nil {
					urlStr := pathToUrl(walkPath)
					feedItems = append(feedItems, templates.FeedItem{
						Type:  string(gallery.GetMediaClass(walkPath)),
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

	sort.Slice(feedItems, func(i, j int) bool {
		return feedItems[i].Mdate.After(feedItems[j].Mdate)
	})

	// Filter latest entries
	var latestItems []templates.FeedItem
	freshPeriod := time.Now().Add(-feedFreshness) // negative duration to subtract
	for _, entry := range feedItems {
		if entry.Mdate.After(freshPeriod) || len(latestItems) < feedNotFreshCount {
			latestItems = append(latestItems, entry)
		}
	}

	lastDate := time.Now()
	if len(latestItems) > 0 {
		lastDate = latestItems[0].Mdate
	}
	lastDateStr := formatTime(lastDate)
	w.Header().Set("Last-modified", lastDateStr)

	feedTpl := templates.FeedPage{
		FeedUrl:   config.Global.PublicUrl + "feed?" + string(feedType),
		SiteTitle: config.Global.PublicHost,
		SiteUrl:   config.Global.PublicUrl,
		LastDate:  lastDateStr,
		Items:     latestItems,
	}
	err = templates.Feed.ExecuteTemplate(w, string(feedType), &feedTpl)
	if err != nil {
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
			// Allow empty keys if they are the last element
			key, err1 := url.QueryUnescape(parts[i])
			if err1 != nil {
				err = err1
				continue
			}
			m.Add(strings.ToLower(key), "")
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
		m.Add(strings.ToLower(key), strings.ToLower(val))
	}
	return m, err
}

// HttpHandler does the main routing of requests.
//
// Since we are mapping URLs to filesystem resources we cannot use any names
// for our internal resources.
//
// Types of content that are served:
//   - internal resource (image, css, etc.)
//   - list of folder items
//   - view of an item (html)
//   - preview image (thumbnail)
//   - direct media file
//   - info page about our running program
//   - RSS (or atom) feed
func HttpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, urlPrefix)
	q, _ := parseQuery(r.URL.RawQuery)

	// We use query string parameters for internal resources. Isn't that novel!
	switch {
	case q.Has("status"):
		statusHandler(w, r)
		return
	case q.Has("thumb"):
		previewHandler(w, r)
		return
	case q.Has("broken"): // Keep this separate from static, just in case...
		staticHandler("res/broken.svg", w, r)
		return
	case q.Has("static"):
		staticResource := q.Get("static")
		staticHandler("res/"+staticResource, w, r)
		return
	case q.Has("rss"):
		feedHandler(feedRss, w, r)
		return
	case q.Has("atom"):
		feedHandler(feedAtom, w, r)
		return
	case q.Has("error"):
		fail404(w, r)
		return
	}

	stat, err := storage.Root.Stat(fullPath)
	if err != nil { // Non-existing resource was requested
		fail404(w, r)
		return
	}
	switch {
	case stat.IsDir():
		listHandler(w, r)
	case q.Get(config.QKeyDisplay.String()) == string(config.QueryDisplayFile):
		// This is a media file and we should serve it in all it's glory
		fileHandler(w, r)
	default:
		viewHandler(w, r)
	}
}

func paramHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q, _ := parseQuery(r.URL.RawQuery)
		rSettings := config.RequestSettingsFromQuery(q)
		sortCtx := context.WithValue(r.Context(), reqSettings, rSettings)
		next.ServeHTTP(w, r.WithContext(sortCtx))
	})
}

func metadataHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		metafile := strings.TrimPrefix(r.URL.Path, urlPrefix)
		if !config.HasFolderSettings(metafile) {
			next.ServeHTTP(w, r)
			return
		}
		fs, err := config.ReadFolderSettings(metafile)
		if err != nil {
			logger.Printf("metadata error: %v\n", err)
			next.ServeHTTP(w, r)
			return
		}
		logger.Printf("metadata: %+v\n", fs)
		metaCtx := context.WithValue(r.Context(), folderSettings, fs)
		next.ServeHTTP(w, r.WithContext(metaCtx))
	})
}

func initGlobalsAndFlags() error {
	startTime = time.Now()
	var errTime error
	// NOTE: Embedded static files use the build time of the binary for timestamp
	BuildTime, errTime = time.Parse(time.RFC3339, BuildTimestamp)
	if errTime != nil {
		BuildTime = time.Now()
	}

	// Get current execution folder
	execFolder, err := os.Getwd()
	if err != nil {
		return err
	}

	// Environment variables
	config.Global.LoadEnv(execFolder)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [options] [FOLDER]\n", filepath.Base(os.Args[0]))
		fmt.Fprintf(os.Stderr, "\tFOLDER overrides \"--root\".\n\n")
		fmt.Printf("Options:\n")
		flag.PrintDefaults()
	}

	// NOTE: Command line arguments override env
	flag.StringVar(&config.Global.Host, "host", config.Global.Host,
		"host address to bind to")
	flag.IntVar(&config.Global.Port, "port", config.Global.Port,
		"port to run at")
	flag.StringVar(&config.Global.Home, "home", config.Global.Home,
		"folder used to keep thumbnails (by default a temporary folder is created and auto-removed on exit)")
	flag.StringVar(&config.Global.Root, "root", config.Global.Root,
		"root folder to serve files from")
	flag.StringVar(&config.Global.Prefix, "prefix", config.Global.Prefix,
		"path prefix as in http://localhost/PREFIX/other/stuff")
	flag.StringVar(&config.Global.TlsCrt, "tls-crt", config.Global.TlsCrt,
		"certificate file for TLS")
	flag.StringVar(&config.Global.TlsKey, "tls-key", config.Global.TlsKey,
		"key file for TLS")
	flag.BoolVar(&config.Global.Http2, "http2", config.Global.Http2,
		"enable HTTP/2 (only with TLS)")
	flag.BoolVar(&config.Global.Quiet, "quiet", config.Global.Quiet,
		"don't print to console")
	flag.DurationVar((*time.Duration)(&config.Global.CacheExpiresAfter),
		"cache-expires-after", time.Duration(config.Global.CacheExpiresAfter),
		"duration to keep cached resources in memory")
	flag.DurationVar((*time.Duration)(&config.Global.NotifyAfter),
		"notify-after", time.Duration(config.Global.NotifyAfter),
		"duration to delay notifications and combine them in one")
	flag.StringVar(&config.Global.DiscordWebhook,
		"discord-webhook", config.Global.DiscordWebhook,
		"webhook URL to receive notifications when new media appears")
	flag.StringVar(&config.Global.DiscordName,
		"discord-name", config.Global.DiscordName,
		"name to appear on sent notifications")
	flag.StringVar(&config.Global.PublicHost,
		"pub-host", config.Global.PublicHost,
		"the public name for the machine")
	flag.IntVar(&config.Global.ThumbWidth,
		"thumb-width", config.Global.ThumbWidth, "width for thumbnails")
	flag.IntVar(&config.Global.ThumbHeight,
		"thumb-height", config.Global.ThumbHeight, "height for thumbnails")
	flag.StringVar(&config.Global.ConfigFile,
		"config", config.Global.ConfigFile,
		"json file to get all the parameters from")
	flag.StringVar(&config.Global.Copyright,
		"copyright", config.Global.Copyright,
		"text to appear at the bottom of every page")

	flag.Bool("version", false, "show program version and build time")

	flag.Parse()

	rootArg := flag.Arg(0)
	if rootArg != "" {
		config.Global.Root = rootArg
	}
	return nil
}

func main() {
	exitCode := 0
	defer os.Exit(exitCode)

	if err := initGlobalsAndFlags(); err != nil {
		log.Println(err)
		exitCode = 1
		return
	}

	// Check for version flag
	if flag.Lookup("version").Value.(flag.Getter).Get().(bool) {
		fmt.Printf("foldergal %v, built on %v\n",
			BuildVersion, BuildTime.In(time.Local))
		exitCode = 1
		return
	}

	// NOTE: Variables from ConfigFile override all other from env or command line
	if config.Global.ConfigFile != "" {
		if err := config.Global.FromJson(config.Global.ConfigFile); err != nil {
			log.Printf("error: invalid config %v", err)
			exitCode = 1
			return
		}
	}
	if config.Global.Home == "" {
		if !config.Global.Quiet {
			log.Println("Creating temporary home folder...")
		}
		tempDir, err := os.MkdirTemp("", "foldergal")
		if err != nil {
			log.Println(err)
			exitCode = 1
			return
		}
		defer func() {
			if !config.Global.Quiet {
				log.Println("Removing temporary home folder...")
			}
			if err := os.RemoveAll(tempDir); err != nil {
				log.Println(err)
			}
		}()
		config.Global.Home = tempDir
	} else {
		config.Global.Home, _ = filepath.Abs(config.Global.Home)
	}
	config.Global.Root, _ = filepath.Abs(config.Global.Root)

	// Set up time location
	if config.Global.TimeZone == "" {
		config.Global.TimeZone = "Local"
	}
	var err error
	config.Global.TimeLocation, err = time.LoadLocation(config.Global.TimeZone)
	if err != nil {
		log.Println(err)
		exitCode = 1
		return
	}
	time.Local = config.Global.TimeLocation

	// Set up log file
	logFile := filepath.Join(config.Global.Home, "foldergal.log")
	logging, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY,
		0o0644) // #nosec Permit everybody to read the log file
	if err != nil {
		log.Printf("Log file cannot be created: %v", err)
		exitCode = 1
		return
	}
	defer logging.Close()

	config.Global.Log = log.New(logging, "foldergal: ", log.Lshortfile|log.LstdFlags)
	logger = config.Global.Log

	infoF("-- Starting v:%v --", BuildVersion)
	if !config.Global.Quiet {
		log.Printf("Log file: %s", logFile)
	}
	if config.Global.TimeZone != "Local" {
		infoF("Time zone: %s (%s)",
			config.Global.TimeLocation.String(), config.Global.TimeZone)
	}

	// Set root media folder
	if exists, err := os.Stat(config.Global.Root); os.IsNotExist(err) || !exists.IsDir() {
		log.Printf("Root folder does not exist: %v", config.Global.Root)
		exitCode = 1
		return
	}
	infoF("Root folder: %s", config.Global.Root)
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
		log.Println(err)
		exitCode = 1
		return
	}
	infoF("Cache folder: %s", config.Global.Cache)
	if config.Global.CacheExpiresAfter == 0 {
		storage.Cache = afero.NewBasePathFs(afero.NewOsFs(), config.Global.Cache)
	} else {
		storage.Cache = afero.NewCacheOnReadFs(
			afero.NewBasePathFs(afero.NewOsFs(), config.Global.Cache),
			afero.NewMemMapFs(),
			time.Duration(config.Global.CacheExpiresAfter))
		infoF("Cache in-memory expiration after: %v",
			time.Duration(config.Global.CacheExpiresAfter))
	}

	// Routing
	httpmux := http.NewServeMux()
	if config.Global.Prefix != "" {
		urlPrefix = fmt.Sprintf("/%s", strings.Trim(config.Global.Prefix, "/"))
		httpmux.Handle(urlPrefix,
			http.StripPrefix(urlPrefix, http.HandlerFunc(HttpHandler)))
	}
	httpmux.Handle("/favicon.ico",
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			staticHandler("res/favicon.ico", w, r)
		}))
	httpmux.Handle("/", http.HandlerFunc(HttpHandler))
	bind := fmt.Sprintf("%s:%d", config.Global.Host, config.Global.Port)

	paramMux := paramHandler(httpmux)
	metaMux := metadataHandler(paramMux)

	ffmpegPath := config.Global.Ffmpeg
	if config.Global.Ffmpeg == "" {
		ffmpegPath = "ffmpeg"
	}
	if ffmpegPath, err := exec.LookPath(ffmpegPath); err == nil {
		config.Global.Ffmpeg = ffmpegPath
		infoF("FFmpeg found at: %v", ffmpegPath)
	} else {
		config.Global.Ffmpeg = ""
	}

	// Server start sequence
	useTls := false
	if fileExists(config.Global.TlsCrt) && fileExists(config.Global.TlsKey) {
		useTls = true
		infoF("TLS certificate: %s, key: %s",
			config.Global.TlsCrt, config.Global.TlsKey)
	}
	if config.Global.DiscordWebhook != "" { // Start filesystem watcher
		go gallery.StartFsWatcher()
	}

	if config.Global.PublicHost != "" {
		config.Global.PublicUrl = strings.Trim(config.Global.PublicHost, "/") +
			urlPrefix + "/"
	} else {
		config.Global.PublicUrl = bind + urlPrefix + "/"
	}

	if useTls {
		config.Global.PublicUrl = "https://" + config.Global.PublicUrl
	} else {
		config.Global.PublicUrl = "http://" + config.Global.PublicUrl
	}
	logger.Printf("Running server at: %v", config.Global.PublicUrl)
	if !config.Global.Quiet {
		log.Printf("Running server at: %v\nPress ^C to stop...\n",
			config.Global.PublicUrl)
	}

	exitChan := make(chan os.Signal, 1)
	signal.Notify(exitChan, syscall.SIGINT, syscall.SIGTERM)

	srv := &http.Server{
		ReadHeaderTimeout: headerTimeout,
		Addr:              bind,
		Handler:           metaMux,
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
		srv.TLSConfig = tlsConfig
		go func() {
			err := srv.ListenAndServeTLS(config.Global.TlsCrt, config.Global.TlsKey)
			if !errors.Is(err, http.ErrServerClosed) {
				log.Println(err)
				exitChan <- os.Interrupt
			}
		}()
	} else { // Normal start
		go func() {
			if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
				log.Println(err)
				exitChan <- os.Interrupt
			}
		}()
	}

	// Graceful shutdown
	// see: https://dev.to/mokiat/proper-http-shutdown-in-go-3fji
	<-exitChan

	shutdownCtx, cancelShutdown := context.WithTimeout(
		context.Background(), shutdownTimeout)
	defer cancelShutdown()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
		_ = srv.Close()
		exitCode = 1
	}
}
