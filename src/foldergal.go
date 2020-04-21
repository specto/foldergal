package main

import (
	"github.com/gabriel-vasile/mimetype"
	"net/http"
	"os"
	"strings"
)

func containsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			Logger.Printf("Detected dot: %s", name)
			return true
		}
	}
	return false
}

func validMediaFile(name string) bool {
	detectedMime, _ := mimetype.DetectFile(name)
	isMedia := false
	mimePrefixes := []string{"image", "video", "audio"}
	for mime := detectedMime; mime != nil; mime = mime.Parent() {
		for _, mimePrefix := range mimePrefixes {
			if strings.HasPrefix(mime.String(), mimePrefix) {
				isMedia = true
			}
		}
	}
	return isMedia
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
	return filteredFile{file}, err
}

func (f filteredFile) Readdir(n int) (fis []os.FileInfo, err error) {
	files, err := f.File.Readdir(n)
	for _, file := range files { // Filter out the dot files from listing
		if !containsDotFile(file.Name()) {
			fis = append(fis, file)
		}
	}
	return
}