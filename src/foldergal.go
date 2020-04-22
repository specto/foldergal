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

func storePreviewFile(fullpath string, contents []byte) error {
	err := cacheFs.MkdirAll(filepath.Dir(fullpath), os.ModePerm)
	if err != nil {
		return err
	}
	_, err = cacheFs.Create(fullpath)
	if err != nil {
		return err
	} else {
		return afero.WriteFile(cacheFs, fullpath, contents, os.ModePerm)
	}
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

func validMediaFile(file http.File) bool {
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	if err != nil {
		return false
	}
	// Reset seek to start to be able to read the file later
	_, _ = file.Seek(0, 0)
	contentType := http.DetectContentType(buffer)
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
	exists, _ := afero.Exists(cacheFs, fullPath)
	if !exists { // Generate thumbnail for the first time
		file, err := rootFs.Open(fullPath)
		if err != nil {
			fail500(w, err, r)
			return
		} else {
			if !validMediaFile(file) { // check for valid media file
				http.NotFound(w, r)
				return
			}
			// TODO: correct for exif image rotation
			// TODO: implement thumbs for other media files
			img, err := imaging.Decode(file)
			if err != nil {
				fail500(w, err, r)
				return
			}
			resized := imaging.Fit(img, ThumbWidth, ThumbHeight, imaging.Lanczos)
			buf := new(bytes.Buffer)
			err = jpeg.Encode(buf, resized, nil)
			if err != nil {
				fail500(w, err, r)
				return
			}
			_ = storePreviewFile(strings.TrimSuffix(fullPath, filepath.Ext(fullPath))+".jpg", buf.Bytes())
		}
		defer func() { _ = file.Close() }()
	}
	// TODO: compare cached date with root date and refresh if needed
	thumbFile, err := cacheFs.Open(fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	defer func() { _ = thumbFile.Close() }()
	// TODO: correct modtime
	http.ServeContent(w, r, fullPath, time.Now(), io.ReadSeeker(thumbFile))
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
			H:	   ThumbHeight,
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
