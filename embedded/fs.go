package embedded

import "github.com/spf13/afero"

var Fs afero.Fs

func Intialize() {
	Fs = afero.NewMemMapFs()
	for name, data := range generatedFiles {
		file, err := Fs.Create(name)
		if err != nil {
			panic(err)
		}
		_, _ = file.Write(data)
	}
	// Seal as readonly
	Fs = afero.NewReadOnlyFs(Fs)
}
