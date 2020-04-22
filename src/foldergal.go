package main

import (
	"./templates"
	"bytes"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/spf13/afero"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
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
	thumb() (io.ReadSeeker, afero.File, error)
	thumbExists() bool
	thumbGenerate() error
	thumbExpired() bool

	file() (io.ReadSeeker, afero.File, error)
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

func (f mediaFile) thumbExists() bool {
	exists, err := afero.Exists(cacheFs, f.thumbPath)
	if err != nil {
		return false
	}
	return exists
}

func (f mediaFile) fileExists() bool {
	exists, err := afero.Exists(rootFs, f.fullPath)
	if err != nil {
		return false
	}
	return exists
}

func (f mediaFile) thumbModTime() time.Time {
	return f.thumbInfo.ModTime()
}

func (f mediaFile) fileModTime() time.Time {
	return f.fileInfo.ModTime()
}

func (f mediaFile) thumbExpired() bool {
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

func (f mediaFile) thumb() (io.ReadSeeker, afero.File, error) {
	file, err := cacheFs.Open(f.thumbPath)
	if err != nil {
		return nil, nil, err
	}
	return io.ReadSeeker(file), file, nil
}

func (f mediaFile) file() (io.ReadSeeker, afero.File, error) {
	file, err := rootFs.Open(f.fullPath)
	if err != nil {
		return nil, nil, err
	}
	return io.ReadSeeker(file), file, nil
}

func makeMedia(fullPath string, thumbPath string) (mediaFile, error) {
	fileStat, err := rootFs.Stat(fullPath)
	if err != nil {
		return mediaFile{}, err
	}
	thumbStat, err := cacheFs.Stat(thumbPath)
	if err != nil {
		// Thumb is missing, so what
		return mediaFile{
			fullPath:  fullPath,
			fileInfo:  fileStat,
			thumbPath: thumbPath,
		}, nil
	}
	return mediaFile{
		fullPath:  fullPath,
		fileInfo:  fileStat,
		thumbPath: thumbPath,
		thumbInfo: thumbStat,
	}, nil
}

type imageFile struct {
	mediaFile
}

func (f imageFile) thumbGenerate() error {
	file, err := rootFs.Open(f.fullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return err
	}
	img, err := imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}
	resized := imaging.Fit(img, ThumbWidth, ThumbHeight, imaging.Lanczos)
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, resized, nil)
	if err != nil {
		return err
	}
	err = cacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return err
	}
	_, err = cacheFs.Create(f.thumbPath)
	if err != nil {
		return err
	}
	_ = afero.WriteFile(cacheFs, f.thumbPath, buf.Bytes(), os.ModePerm)
	f.thumbInfo, err = cacheFs.Stat(f.thumbPath)
	if err != nil {
		return err
	}
	return nil
}

type svgFile struct {
	mediaFile
}

func (f svgFile) thumbGenerate() error {
	file, err := rootFs.Open(f.fullPath)
	//defer func() { _ = file.Close() }()
	if err != nil {
		return err
	}

	contents, err := afero.ReadAll(file)
	if err != nil {
		return err
	}
	err = cacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return err
	}
	_, err = cacheFs.Create(f.thumbPath)
	if err != nil {
		return err
	}
	_ = afero.WriteFile(cacheFs, f.thumbPath, contents, os.ModePerm)
	f.thumbInfo, err = cacheFs.Stat(f.thumbPath)
	if err != nil {
		return err
	}
	// Since we just copy SVGs the thumb file is the same as the original
	f.thumbPath = f.fullPath
	return nil
}

type audioFile struct {
	mediaFile
}

type videoFile struct {
	mediaFile
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
	m, err := makeMedia(fullPath, thumbPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	// TODO: implement thumbs for other media files
	if strings.HasPrefix(contentType, "image/svg") {
		w.Header().Set("Content-Type", contentType)
		f = &svgFile{m}
	} else if strings.HasPrefix(contentType, "image/") {
		f = &imageFile{m}
	} else {
		// TODO: return default "broken" image
		http.NotFound(w, r)
		return
	}
	if !f.fileExists() {
		http.NotFound(w, r)
		return
	}
	if !f.thumbExists() || f.thumbExpired() {
		err := f.thumbGenerate()
		if err != nil {
			fail500(w, err, r)
			return
		}
	}
	thumb, file, err := f.thumb()
	defer func() { _ = file.Close() }()
	if err != nil {
		fail500(w, err, r)
		return
	}
	http.ServeContent(w, r, f.media().thumbPath, f.media().thumbInfo.ModTime(), thumb)
}

func listHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	contents, err := afero.ReadDir(rootFs, fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	var (
		parentUrl string
		title     string
	)
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
		var thumb string
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
	// TODO: check for correctness
	file, err := rootFs.Open(fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	defer func() { _ = file.Close() }()
	// TODO: fix modtime
	http.ServeContent(w, r, fullPath, time.Now(), io.ReadSeeker(file))
}

// Elaborate router
func httpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	//logger.Printf("URL: %v", r.URL)

	if _, ok := r.URL.Query()["thumb"]; ok { // Thumbnails are marked with &thumb in the query string
		previewHandler(w, r)
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
