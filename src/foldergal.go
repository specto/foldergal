package main

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

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

func validMediaFile(file http.File, name string) bool {
	buffer := make([]byte, 512)
	_, err := file.Read(buffer)
	// Reset seek to start to be able to read the file later
	_, _ = file.Seek(0, 0)
	if err != nil {
		return false
	}
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
	if !stat.IsDir() && !validMediaFile(file, name) {
		return nil, os.ErrNotExist
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