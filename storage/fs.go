package storage

import (
	"embed"
	"net/http"
	"github.com/spf13/afero"
)

var (
	// Internal resources: images, html, css...
	//go:embed res
	embedded embed.FS
	Internal afero.Fs
	InternalHttp http.FileSystem
	// The main storage
	Root afero.Fs
	// Thumbnails live here
	Cache afero.Fs
)


func init() {
	InternalHttp = http.FS(embedded)
	Internal = afero.NewReadOnlyFs(afero.FromIOFS{embedded})
// 	fmt.Println(content)
// 	fmt.Println("LISTING CONTENT...")
// 	fmt.Println(content.ReadDir("static"))
// // 	Internal = content
}
