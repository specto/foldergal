package config

import (
	"bufio"
	"path/filepath"

	"specto.org/projects/foldergal/internal/storage"

	yaml "github.com/goccy/go-yaml"
)

var MetafileName = "_foldergal.yaml"

type FolderSettings struct {
	Description string
	Copyright   string
}

func ReadFolderSettings(path string) (FolderSettings, error) {
	fs := FolderSettings{}
	yamlFile, errRead := storage.Root.Open(filepath.Join(path, MetafileName))
	if errRead != nil {
		return fs, errRead
	}
	defer yamlFile.Close()

	stats, statsErr := yamlFile.Stat()
	if statsErr != nil {
		return fs, statsErr
	}
	var size int64 = stats.Size()
	yamlData := make([]byte, size)

	_, errRead = bufio.NewReader(yamlFile).Read(yamlData)
	if errRead != nil {
		return fs, errRead
	}

	if err := yaml.Unmarshal(yamlData, &fs); err != nil {
		return fs, err
	}
	return fs, nil
}

func HasFolderSettings(path string) bool {
	file := filepath.Join(path, MetafileName)
	if file, err := storage.Root.Stat(file); err != nil || file.IsDir() {
		return false
	}
	return true
}
