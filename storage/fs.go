package storage

import "github.com/spf13/afero"

var (
	Internal afero.Fs
	Root     afero.Fs
	Cache    afero.Fs
)

func Intialize() {
	Internal = afero.NewMemMapFs()
	for name, data := range generatedFiles {
		file, err := Internal.Create(name)
		if err != nil {
			panic(err)
		}
		_, _ = file.Write(data)
	}
	// Seal as readonly
	Internal = afero.NewReadOnlyFs(Internal)
}
