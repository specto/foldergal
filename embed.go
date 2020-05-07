// +build ignore

package main

import (
	"os"
	"text/template"
	"time"
)

func main() {
	f, err := os.Create("embedded/generated.go")
	defer f.Close()
	genTemplate.Execute(f, struct {
		Timestamp time.Time
		Contents  string
	}{
		Timestamp: time.Now(),
		Contents: `
			Files["asdf.svg"] = []byte("well ....")
		`,
	})
	if err != nil {
		panic("Failed to generate file")
	}
}

var genTemplate = template.Must(template.New("").Parse(`// Code generated automatically DO NOT EDIT.
// Last modified: {{ .Timestamp }}

package embedded

var Files = make(map[string][]byte)

func init() {
{{ .Contents }}
}
`))
