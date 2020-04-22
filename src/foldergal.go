package main

import (
	"./templates"
	"fmt"
	"github.com/daddye/vips"
	"github.com/spf13/afero"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var thumbOptions = vips.Options{
	Width:        400,
	Height:       400,
	Crop:         false,
	Extend:       vips.EXTEND_WHITE,
	Interpolator: vips.BILINEAR,
	Gravity:      vips.CENTRE,
	Quality:      95,
}

func storePreviewFile(name string, contents []byte) error {
	err := cacheFs.MkdirAll(filepath.Dir(name), os.ModePerm)
	if err != nil {
		return err
	}
	_, _ = cacheFs.Create(name)
	buf, err := vips.Resize(contents, thumbOptions)
	if err != nil {
		return err
	} else {
		return afero.WriteFile(cacheFs, name, buf, os.ModePerm)
	}
}

func getPreviewFile(name string) ([]byte, error) {
	return afero.ReadFile(cacheFs, name)
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

type filteredFile struct {
	http.File
}

type filteredFileSystem struct {
	http.FileSystem
}

func (fs filteredFileSystem) Open(name string) (http.File, error) {
	if containsDotFile(name) {
		return nil, os.ErrNotExist
	}

	file, err := fs.FileSystem.Open(name)
	if err != nil {
		return nil, err
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !stat.IsDir() {
		if !validMediaFile(file) {
			return nil, os.ErrNotExist
		}
		contents, err := afero.ReadFile(rootFs, filepath.Join(root, name))
		if err != nil {
			logger.Print(err)
		} else {
			_ = storePreviewFile(name, contents)
		}
	}
	return filteredFile{file}, nil
}

func (f filteredFile) Readdir(n int) (fis []os.FileInfo, err error) {
	files, err := f.File.Readdir(n)
	for _, file := range files { // Filter out the dot and non-media files from listing
		if !containsDotFile(file.Name()) && (file.IsDir() || validMediaByExtension(file.Name())) {
			fis = append(fis, file)
		}
	}
	return
}

func httpHandler(w http.ResponseWriter, r *http.Request) {
	fullPath := filepath.Join(root, r.URL.Path)
	exists, _ := afero.Exists(rootFs, fullPath)
	if !exists {
		http.NotFound(w, r)
		return
	}
	isDir, _ := afero.IsDir(rootFs, fullPath)
	if isDir { // Prepare and render folder contents
		contents, err := afero.ReadDir(rootFs, fullPath)
		if err != nil {
			logger.Print(err)
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
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
			children = append(children, templates.ListItem{
				Url:  childPath,
				Name: child.Name(),
			})
		}
		var (
			parentUrl string
			title     string
		)
		if fullPath != root {
			title = filepath.Base(r.URL.Path)
			parentUrl = filepath.Join(urlPrefix, r.URL.Path, "..")
		}
		err = templates.ListTpl.ExecuteTemplate(w, "layout", &templates.List{
			Page:      templates.Page{Title: title, Prefix: urlPrefix},
			ParentUrl: parentUrl,
			Items:     children,
		})
		if err != nil {
			logger.Print(err)
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
		}
	} else {
		// TODO: Serve files from rootFs!
		http.ServeFile(w, r, fullPath)
		return
	}

	fmt.Fprintf(w, "URL: %v", fullPath)
}
