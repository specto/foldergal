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

func (f *mediaFile) file() *afero.File {
	file, err := rootFs.Open(f.fullPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *mediaFile) fileExists() (exists bool) {
	exists, _ = afero.Exists(rootFs, f.fullPath)
	// Ensure we refresh file stat
	f.fileInfo, _ = cacheFs.Stat(f.fullPath)
	return
}

type imageFile struct {
	*mediaFile
}

func (f *imageFile) thumb() *afero.File {
	if !f.thumbExists() {
		return nil
	}
	file, err := cacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *imageFile) thumbExists() (exists bool) {
	exists, _ = afero.Exists(cacheFs, f.thumbPath)
	// Ensure we refresh thumb stat
	f.thumbInfo, _ = cacheFs.Stat(f.thumbPath)
	return
}

func (f *imageFile) thumbExpired() (expired bool) {
	if !f.thumbExists() {
		return
	}
	m := f.media()
	diff := m.thumbInfo.ModTime().Sub(m.fileInfo.ModTime())
	return diff < 0*time.Second
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
	f.thumbInfo, err = cacheFs.Stat(f.thumbPath)
	return
}

type svgFile struct {
	*mediaFile
}

func (f *svgFile) thumb() *afero.File {
	if !f.thumbExists() {
		return nil
	}
	file, err := cacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *svgFile) thumbExists() (exists bool) {
	exists, _ = afero.Exists(cacheFs, f.thumbPath)
	// Ensure we refresh thumb stat
	f.thumbInfo, _ = cacheFs.Stat(f.thumbPath)
	return
}

func (f *svgFile) thumbExpired() (expired bool) {
	if !f.thumbExists() {
		return
	}
	m := f.media()
	diff := m.thumbInfo.ModTime().Sub(m.fileInfo.ModTime())
	return diff < 0*time.Second
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
	f.thumbInfo, err = cacheFs.Stat(f.thumbPath)
	return
}

type audioFile struct {
	*mediaFile
}

func (f *audioFile) thumb() *afero.File {
	thumb, _ := memoryFs.Open("audio.svg")
	return &thumb
}

func (f *audioFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(memoryFs, "audio.svg")
	if err != nil {
		return false
	}
	f.thumbInfo, err = memoryFs.Stat("audio.svg")
	if err != nil {
		return false
	}
	return
}

func (f *audioFile) thumbExpired() (expired bool) {
	return true
}

func (f *audioFile) thumbGenerate() (err error) {
	_ = f.thumbExists()
	return
}

type videoFile struct {
	*mediaFile
}

func (f *videoFile) thumb() *afero.File {
	thumb, _ := memoryFs.Open("video.svg")
	return &thumb
}

func (f *videoFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(memoryFs, "video.svg")
	if err != nil {
		return false
	}
	f.thumbInfo, err = memoryFs.Stat("video.svg")
	if err != nil {
		return false
	}
	return
}

func (f *videoFile) thumbExpired() (expired bool) {
	return true
}

func (f *videoFile) thumbGenerate() (err error) {
	_ = f.thumbExists()
	return
}

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

	if strings.HasPrefix(contentType, "image/svg") {
		w.Header().Set("Content-Type", contentType)
		f = &svgFile{&mediaFile{fullPath: fullPath, thumbPath: thumbPath}}
	} else if strings.HasPrefix(contentType, "image/") {
		f = &imageFile{&mediaFile{fullPath: fullPath, thumbPath: thumbPath}}
	} else if strings.HasPrefix(contentType, "audio/") {
		f = &audioFile{&mediaFile{fullPath: fullPath}}
	} else if strings.HasPrefix(contentType, "video/") {
		f = &videoFile{&mediaFile{fullPath: fullPath}}
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

type embeddedFileId int

const (
	brokenImage = iota + 1
	folderImage
	upImage
	audioImage
	videoImage
)

var embeddedFiles = make(map[embeddedFileId][]byte)

var memoryFs afero.Fs

func init() {
	embeddedFiles[brokenImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
			 viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
			.st0{fill:#FFFFFF;stroke:#999999;stroke-width:64;stroke-miterlimit:10;}
			.st1{fill:none;stroke:#999999;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<rect x="151" y="96" class="st0" width="722" height="576"/>
		<line class="st1" x1="272.5" y1="367" x2="442.5" y2="197"/>
		<line class="st1" x1="272.5" y1="197" x2="442.5" y2="367"/>
		<line class="st1" x1="581.5" y1="367" x2="751.5" y2="197"/>
		<line class="st1" x1="581.5" y1="197" x2="751.5" y2="367"/>
		<path class="st1" d="M614.26,590c0-37-44.89-78-100.26-78s-100.26,41-100.26,78"/>
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
	// Todo: change with actual audio icon
	embeddedFiles[audioImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
		viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
		.st0{fill:#FFFFFF;stroke:orange;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.39,0-49.6-22.21-49.6-49.6V139.1c0-27.39,22.21-49.6,49.6-49.6h199.97l24.29,80H849.9
		c27.39,0,49.6,22.21,49.6,49.6v409.8C899.5,656.29,877.29,678.5,849.9,678.5z"/>
		</svg>`)
	// Todo: change with actual video icon
	embeddedFiles[videoImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
		viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
		.st0{fill:#FFFFFF;stroke:blue;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.39,0-49.6-22.21-49.6-49.6V139.1c0-27.39,22.21-49.6,49.6-49.6h199.97l24.29,80H849.9
		c27.39,0,49.6,22.21,49.6,49.6v409.8C899.5,656.29,877.29,678.5,849.9,678.5z"/>
		</svg>`)

	memoryFs = afero.NewMemMapFs()
	mmfile, _ := memoryFs.Create("video.svg")
	_, _ = mmfile.Write(embeddedFiles[videoImage])
	mmfile, _ = memoryFs.Create("audio.svg")
	_, _ = mmfile.Write(embeddedFiles[audioImage])
}

func embeddedFileHandler(w http.ResponseWriter, r *http.Request, id embeddedFileId) {
	w.Header().Set("Content-Type", "image/svg+xml")
	http.ServeContent(w, r, r.URL.Path, time.Now(), bytes.NewReader(embeddedFiles[id]))
}
