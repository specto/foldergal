package embedded

import (
	"github.com/spf13/afero"
)

var Fs afero.Fs

func init() {
	Fs = afero.NewMemMapFs()
	for name, data := range files {
		file, _ := Fs.Create(name)
		_, _ = file.Write(data)
	}
	// Seal as readonly
	Fs = afero.NewReadOnlyFs(Fs)
}
