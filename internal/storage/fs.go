package storage

import (
	"embed"
	"github.com/spf13/afero"
	"net/http"
)

var (
	// Internal resources: images, html, css...
	//go:embed res
	embedded     embed.FS
	Internal     afero.Fs
	InternalHttp http.FileSystem
	// The main storage
	Root afero.Fs
	// Thumbnails live here
	Cache afero.Fs
)

func init() {
	InternalHttp = http.FS(embedded)
	Internal = afero.NewReadOnlyFs(afero.FromIOFS{embedded})
}
