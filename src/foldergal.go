package main

import (
	"./templates"
	"bytes"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/spf13/afero"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ThumbWidth  = 200
	ThumbHeight = 200
)

type media interface {
	media() *mediaFile // Expose our basic data structure in interface
	thumb() *afero.File
	thumbExists() bool
	thumbGenerate() error
	thumbExpired() bool

	file() *afero.File
	fileExists() bool
}

type mediaFile struct {
	fullPath  string
	fileInfo  os.FileInfo
	thumbPath string
	thumbInfo os.FileInfo
}

func (f *mediaFile) media() *mediaFile {
	return f
}

func (f *mediaFile) extractThumbStat() (err error) {
	f.thumbInfo, err = cacheFs.Stat(f.thumbPath)
	return
}

func (f *mediaFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(cacheFs, f.thumbPath)
	if err != nil {
		return false
	}
	// Ensure we refresh thumb stat
	err = f.extractThumbStat()
	if err != nil {
		return false
	}
	return
}

func (f *mediaFile) fileExists() bool {
	exists, err := afero.Exists(rootFs, f.fullPath)
	if err != nil {
		return false
	}
	return exists
}

func (f *mediaFile) thumbExpired() bool {
	if !f.thumbExists() {
		return true
	}
	m := f.media()
	diff := m.thumbInfo.ModTime().Sub(m.fileInfo.ModTime())
	if diff < 0*time.Second {
		return true
	}
	return false
}

func (f *mediaFile) thumb() *afero.File {
	if !f.thumbExists() {
		return nil
	}
	file, err := cacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *mediaFile) file() *afero.File {
	file, err := rootFs.Open(f.fullPath)
	if err != nil {
		return nil
	}
	return &file
}

func makeMedia(fullPath string, thumbPath string) (m mediaFile, err error) {
	var fstat os.FileInfo
	m = mediaFile{
		fullPath:  fullPath,
		thumbPath: thumbPath,
	}
	fstat, err = rootFs.Stat(fullPath)
	if err != nil {
		return
	}
	m.fileInfo = fstat
	// Ignore non-existing thumbs
	_ = m.extractThumbStat()
	return
}

type imageFile struct {
	mediaFile
}

func (f *imageFile) thumbGenerate() (err error) {
	var (
		file afero.File
		img  image.Image
	)
	file, err = rootFs.Open(f.fullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return
	}
	img, err = imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		return
	}
	resized := imaging.Fit(img, ThumbWidth, ThumbHeight, imaging.Lanczos)
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, resized, nil)
	if err != nil {
		return
	}
	err = cacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = cacheFs.Create(f.thumbPath)
	if err != nil {
		return
	}
	_ = afero.WriteFile(cacheFs, f.thumbPath, buf.Bytes(), os.ModePerm)
	_ = f.extractThumbStat()
	return
}

type svgFile struct {
	mediaFile
}

func (f *svgFile) thumbGenerate() (err error) {
	var (
		file     afero.File
		contents []byte
	)
	file, err = rootFs.Open(f.fullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return
	}
	contents, err = afero.ReadAll(file)
	if err != nil {
		return
	}
	err = cacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = cacheFs.Create(f.thumbPath)
	if err != nil {
		return
	}
	err = afero.WriteFile(cacheFs, f.thumbPath, contents, os.ModePerm)
	if err != nil {
		return
	}
	// Since we just copy SVGs the thumb file is the same as the original
	f.thumbPath = f.fullPath
	_ = f.extractThumbStat()
	return
}

//type audioFile struct {
//	mediaFile
//}
//
//type videoFile struct {
//	mediaFile
//}

func containsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

var mimePrefixes = regexp.MustCompile("^(image|video|audio|application/pdf)")

func validMediaByExtension(name string) bool {
	ext := filepath.Ext(name)
	contentType := mime.TypeByExtension(ext)
	match := mimePrefixes.FindStringSubmatch(contentType)
	return match != nil
}

func fail500(w http.ResponseWriter, err error, _ *http.Request) {
	logger.Print(err)
	http.Error(w, "500 internal server error", http.StatusInternalServerError)
}

// Generate and serve image previews of media files
func previewHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	ext := filepath.Ext(fullPath)
	contentType := mime.TypeByExtension(ext)
	var f media
	// All thumbnails are jpeg, except when they are not...
	thumbPath := strings.TrimSuffix(fullPath, filepath.Ext(fullPath)) + ".jpg"
	m, err := makeMedia(fullPath, thumbPath)
	if err != nil {
		logger.Print(err)
		embeddedFileHandler(w, r, brokenImage)
		return
	}
	// TODO: implement thumbs for other media files
	if strings.HasPrefix(contentType, "image/svg") {
		w.Header().Set("Content-Type", contentType)
		f = &svgFile{m}
	} else if strings.HasPrefix(contentType, "image/") {
		f = &imageFile{m}
	} else {
		embeddedFileHandler(w, r, brokenImage)
		return
	}
	if !f.fileExists() {
		embeddedFileHandler(w, r, brokenImage)
		return
	}
	if f.thumbExpired() {
		err := f.thumbGenerate()
		if err != nil {
			logger.Print(err)
			embeddedFileHandler(w, r, brokenImage)
			return
		}
	}
	thumb := f.thumb()
	if thumb == nil || *thumb == nil {
		embeddedFileHandler(w, r, brokenImage)
		return
	}
	thP := f.media().thumbPath
	thT := f.media().thumbInfo.ModTime()
	http.ServeContent(w, r, thP, thT, *thumb)
}

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
			thumb = fmt.Sprintf("%s?w=%d&h=%d&thumb", childPath, ThumbWidth, ThumbHeight)
		}
		children = append(children, templates.ListItem{
			Url:   childPath,
			Name:  child.Name(),
			Thumb: thumb,
			W:     ThumbWidth,
			H:     ThumbHeight,
		})
	}
	err = templates.ListTpl.ExecuteTemplate(w, "layout", &templates.List{
		Page:      templates.Page{Title: title, Prefix: urlPrefix},
		ParentUrl: parentUrl,
		Items:     children,
	})
	if err != nil {
		fail500(w, err, r)
		return
	}
}

func fileHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	thumbPath := strings.TrimSuffix(fullPath, filepath.Ext(fullPath)) + ".jpg"
	m, err := makeMedia(fullPath, thumbPath)
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

type embeddedFileId int

const (
	brokenImage = iota + 1
	folderImage
	upImage
)

var embeddedFiles = make(map[embeddedFileId][]byte)

func init() {
	embeddedFiles[brokenImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
		viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
		.st0{fill:#FFFFFF;stroke:#000000;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.39,0-49.6-22.21-49.6-49.6V139.1c0-27.39,22.21-49.6,49.6-49.6h199.97l24.29,80H849.9
		c27.39,0,49.6,22.21,49.6,49.6v409.8C899.5,656.29,877.29,678.5,849.9,678.5z"/>
		</svg>`)
	embeddedFiles[folderImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
		viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
		.st0{fill:#FFFFFF;stroke:#000000;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.39,0-49.6-22.21-49.6-49.6V139.1c0-27.39,22.21-49.6,49.6-49.6h199.97l24.29,80H849.9
		c27.39,0,49.6,22.21,49.6,49.6v409.8C899.5,656.29,877.29,678.5,849.9,678.5z"/>
		</svg>`)
	embeddedFiles[upImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
			 viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
			.st0{fill:#FFFFFF;stroke:#000000;stroke-width:64;stroke-miterlimit:10;}
			.st1{fill:none;stroke:#000000;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.39,0-49.6-22.21-49.6-49.6V139.1c0-27.39,22.21-49.6,49.6-49.6h199.97l24.29,80H849.9
			c27.39,0,49.6,22.21,49.6,49.6v409.8C899.5,656.29,877.29,678.5,849.9,678.5z"/>
		<polyline class="st1" points="434,385.79 434,541.45 693,541.45 		"/>
		<polygon points="570.06,404.03 297.94,404.03 434,268 			"/>
		</svg>`)
}

func embeddedFileHandler(w http.ResponseWriter, r *http.Request, id embeddedFileId) {
	w.Header().Set("Content-Type", "image/svg+xml")
	http.ServeContent(w, r, r.URL.Path, time.Now(), bytes.NewReader(embeddedFiles[id]))
}
