//go:build ignore
// +build ignore

package main

import (
	"encoding/hex"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	f, err := os.Create("storage/generated.go")
	defer func() { _ = f.Close() }()
	if err != nil {
		panic(err)
	}

	containsDotFile := func(name string) bool {
		parts := strings.Split(name, "/")
		for _, part := range parts {
			if strings.HasPrefix(part, ".") {
				return true
			}
		}
		return false
	}

	processFilesfunc := func(walkPath string, info os.FileInfo, err error) error {
		name := walkPath
		if info.IsDir() || containsDotFile(walkPath) {
			return nil
		}

		f.WriteString(`
	func () {
		content := []byte("`)

		fileBytes, err := ioutil.ReadFile(walkPath)
		if err != nil {
			panic(err)
		}
		hexBytes := make([]byte, hex.EncodedLen(len(fileBytes)))
		hex.Encode(hexBytes, fileBytes)
		f.WriteString(string(hexBytes))

		f.WriteString(`")
		decoded := make([]byte, hex.DecodedLen(len(content)))
		_, err := hex.Decode(decoded, content)
		if err != nil { panic(err) }`)
		f.WriteString("\n")

		f.WriteString("\t\t")
		f.WriteString(`generatedFiles["`)
		f.WriteString(name)
		f.WriteString(`"] = decoded`)
		f.WriteString("\n")
		f.WriteString("\t}()")
		//fmt.Printf("Packaged: %v\n", walkPath)
		return nil
	}

	f.WriteString(`// Code generated automatically DO NOT EDIT.
// Last modified: `)
	f.WriteString(time.Now().String())
	f.WriteString(`

package storage

import "encoding/hex"

func generateFiles() map[string][]byte {
	generatedFiles := make(map[string][]byte)`)
	err = filepath.Walk("res", processFilesfunc)
	if err != nil {
		panic(err)
	}

	f.WriteString(`
	return generatedFiles
}`)

}
