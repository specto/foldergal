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

//func validMediaFile(file http.File) bool {
//	buffer := make([]byte, 512)
//	_, err := file.Read(buffer)
//	if err != nil {
//		return false
//	}
//	// Reset seek to start to be able to read the file later
//	_, _ = file.Seek(0, 0)
//	contentType := http.DetectContentType(buffer)
//	match := mimePrefixes.FindStringSubmatch(contentType)
//	return match != nil
//}

func fail500(w http.ResponseWriter, err error, _ *http.Request) {
	logger.Print(err)
	http.Error(w, "500 internal server error", http.StatusInternalServerError)
}


func previewHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	exists, _ := afero.Exists(cacheFs, fullPath)
	if !exists { // Generate thumbnail for the first time
		// TODO: check for valid media file
		file, err := rootFs.Open(fullPath)
		if err != nil {
			fail500(w, err, r)
			return
		} else {
			// TODO: change image library with one that supports cross compilation and has correct rotation
			img, err := imaging.Decode(file)
			if err != nil {
				fail500(w, err, r)
				return
			}
			resized := imaging.Resize(img, 200, 0, imaging.Lanczos)
			buf := new(bytes.Buffer)
			err = jpeg.Encode(buf, resized, nil)
			if err != nil {
				fail500(w, err, r)
				return
			}
			_ = storePreviewFile(strings.TrimSuffix(fullPath, filepath.Ext(fullPath)) + ".jpg", buf.Bytes())
		}
		defer file.Close()
	}
	// TODO: compare cached date with root date and refresh if needed
	thumbFile, err := cacheFs.Open(fullPath)
	if err != nil {
		fail500(w, err, r)
		return
	}
	defer thumbFile.Close()
	// TODO: correct modtime
	http.ServeContent(w, r, fullPath, time.Now(), io.ReadSeeker(thumbFile))
}


func httpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	logger.Printf("URL: %v", r.URL)

	if _, ok := r.URL.Query()["thumb"]; ok { // Check if thumb is in the query map
		previewHandler(w, r)
		return
	}

	exists, _ := afero.Exists(rootFs, fullPath)
	if !exists {
		http.NotFound(w, r)
		return
	}
	isDir, _ := afero.IsDir(rootFs, fullPath)
	if isDir { // Prepare and render folder contents
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
				thumb = fmt.Sprintf("%s?w=%d&h=%d&thumb", childPath, 200, 200)
			}
			children = append(children, templates.ListItem{
				Url:  childPath,
				Name: child.Name(),
				Thumb: thumb,
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
	} else {
		// TODO: check for correctness
		file, err := rootFs.Open(fullPath)
		defer file.Close()
		if err != nil {
			fail500(w, err, r)
			return
		}
		// TODO: fix modtime
		http.ServeContent(w, r, fullPath, time.Now(), io.ReadSeeker(file))
		return
	}
}
