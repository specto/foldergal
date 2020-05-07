package embedded

import (
	"github.com/spf13/afero"
)

var Resources afero.Fs

func init() {
	Resources = afero.NewMemMapFs()
	for name, data := range Files {
		file, _ := Resources.Create(name)
		_, _ = file.Write(data)
	}
	// Seal as readonly
	Resources = afero.NewReadOnlyFs(Resources)
}
