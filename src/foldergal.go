package main

import (
	"bytes"
	"github.com/disintegration/imaging"
	"github.com/go-errors/errors"
	"github.com/spf13/afero"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ThumbWidth  = 200
	ThumbHeight = 200
)

type media interface {
	media() *mediaFile // Expose our basic data structure in interface
	thumb() *afero.File
	thumbExists() bool
	thumbGenerate() error
	thumbExpired() bool

	file() *afero.File
	fileExists() bool
}

////////////////////////////////////////////////////////////////////////////////
type mediaFile struct {
	fullPath  string
	fileInfo  os.FileInfo
	thumbPath string
	thumbInfo os.FileInfo
}

func (f *mediaFile) media() *mediaFile {
	return f
}

func (f *mediaFile) file() *afero.File {
	file, err := rootFs.Open(f.fullPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *mediaFile) fileExists() (exists bool) {
	var err error
	exists, err = afero.Exists(rootFs, f.fullPath)
	// Ensure we refresh file stat
	f.fileInfo, err = rootFs.Stat(f.fullPath)
	if err != nil {
		return false
	}
	return
}
////////////////////////////////////////////////////////////////////////////////
type imageFile struct {
	mediaFile
}

func (f *imageFile) thumb() *afero.File {
	if !f.thumbExists() {
		return nil
	}
	file, err := cacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *imageFile) thumbExists() (exists bool) {
	exists, _ = afero.Exists(cacheFs, f.thumbPath)
	// Ensure we refresh thumb stat
	f.media().thumbInfo, _ = cacheFs.Stat(f.thumbPath)
	return
}

func (f *imageFile) thumbExpired() (expired bool) {
	if !f.thumbExists() {
		return true
	}
	m := f.media()
	diff := m.thumbInfo.ModTime().Sub(m.fileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *imageFile) thumbGenerate() (err error) {
	var (
		file afero.File
		img  image.Image
	)
	file, err = rootFs.Open(f.fullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return
	}
	img, err = imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		return
	}
	resized := imaging.Fit(img, ThumbWidth, ThumbHeight, imaging.Lanczos)
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, resized, nil)
	if err != nil {
		return
	}
	err = cacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = cacheFs.Create(f.thumbPath)
	if err != nil {
		return
	}
	_ = afero.WriteFile(cacheFs, f.thumbPath, buf.Bytes(), os.ModePerm)
	f.media().thumbInfo, err = cacheFs.Stat(f.thumbPath)
	return
}

////////////////////////////////////////////////////////////////////////////////
type svgFile struct {
	mediaFile
}

func (f *svgFile) thumb() *afero.File {
	if !f.thumbExists() {
		return nil
	}
	file, err := cacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *svgFile) thumbExists() (exists bool) {
	exists, _ = afero.Exists(cacheFs, f.thumbPath)
	// Ensure we refresh thumb stat
	f.media().thumbInfo, _ = cacheFs.Stat(f.thumbPath)
	return
}

func (f *svgFile) thumbExpired() (expired bool) {
	if !f.thumbExists() {
		return true
	}
	m := f.media()
	diff := m.thumbInfo.ModTime().Sub(m.fileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *svgFile) thumbGenerate() (err error) {
	var (
		file     afero.File
		contents []byte
	)
	file, err = rootFs.Open(f.fullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return
	}
	contents, err = afero.ReadAll(file)
	if err != nil {
		return
	}
	err = cacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = cacheFs.Create(f.thumbPath)
	if err != nil {
		return
	}
	err = afero.WriteFile(cacheFs, f.thumbPath, contents, os.ModePerm)
	if err != nil {
		return
	}
	// Since we just copy SVGs the thumb file is the same as the original
	f.thumbPath = f.fullPath
	f.media().thumbInfo, err = cacheFs.Stat(f.thumbPath)
	return
}

////////////////////////////////////////////////////////////////////////////////
type audioFile struct {
	mediaFile
}

func (f *audioFile) thumb() *afero.File {
	thumb, _ := memoryFs.Open("audio.svg")
	return &thumb
}

func (f *audioFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(memoryFs, "audio.svg")
	if err != nil {
		return false
	}
	f.media().thumbInfo, err = memoryFs.Stat("audio.svg")
	if err != nil {
		return false
	}
	return
}

func (f *audioFile) thumbExpired() (expired bool) {
	return true
}

func (f *audioFile) thumbGenerate() (err error) {
	_ = f.thumbExists()
	return
}

////////////////////////////////////////////////////////////////////////////////
type videoFile struct {
	mediaFile
}

func (f *videoFile) thumb() *afero.File {
	thumb, _ := memoryFs.Open("video.svg")
	return &thumb
}

func (f *videoFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(memoryFs, "video.svg")
	if err != nil {
		return false
	}
	f.media().thumbInfo, err = memoryFs.Stat("video.svg")
	if err != nil {
		return false
	}
	return
}

func (f *videoFile) thumbExpired() (expired bool) {
	return true
}

func (f *videoFile) thumbGenerate() (err error) {
	if !f.thumbExists() {
		return errors.New("no video thumbnail")
	}
	return
}

////////////////////////////////////////////////////////////////////////////////
type pdfFile struct {
	mediaFile
}

func (f *pdfFile) thumb() *afero.File {
	thumb, _ := memoryFs.Open("pdf.svg")
	return &thumb
}

func (f *pdfFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(memoryFs, "pdf.svg")
	if err != nil {
		return false
	}
	f.media().thumbInfo, err = memoryFs.Stat("pdf.svg")
	if err != nil {
		return false
	}
	return
}

func (f *pdfFile) thumbExpired() (expired bool) {
	return true
}

func (f *pdfFile) thumbGenerate() (err error) {
	if !f.thumbExists() {
		return errors.New("no video thumbnail")
	}
	return
}

////////////////////////////////////////////////////////////////////////////////

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

type embeddedFileId int

const (
	brokenImage = iota + 1
	folderImage
	upImage
	audioImage
	videoImage
	pdfImage
)

var embeddedFiles = make(map[embeddedFileId][]byte)

var memoryFs afero.Fs

func init() {
	embeddedFiles[brokenImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
			 viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
			.st0{fill:#FFFFFF;stroke:#999999;stroke-width:64;stroke-miterlimit:10;}
			.st1{fill:none;stroke:#999999;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<rect x="151" y="96" class="st0" width="722" height="576"/>
		<line class="st1" x1="272.5" y1="367" x2="442.5" y2="197"/>
		<line class="st1" x1="272.5" y1="197" x2="442.5" y2="367"/>
		<line class="st1" x1="581.5" y1="367" x2="751.5" y2="197"/>
		<line class="st1" x1="581.5" y1="197" x2="751.5" y2="367"/>
		<path class="st1" d="M614.26,590c0-37-44.89-78-100.26-78s-100.26,41-100.26,78"/>
		</svg>`)
	embeddedFiles[folderImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
		viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
		.st0{fill:#FFFFFF;stroke:#000000;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.39,0-49.6-22.21-49.6-49.6V139.1c0-27.39,22.21-49.6,49.6-49.6h199.97l24.29,80H849.9
		c27.39,0,49.6,22.21,49.6,49.6v409.8C899.5,656.29,877.29,678.5,849.9,678.5z"/>
		</svg>`)
	embeddedFiles[upImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
			 viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
			.st0{fill:#FFFFFF;stroke:#000000;stroke-width:64;stroke-miterlimit:10;}
			.st1{fill:none;stroke:#000000;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.39,0-49.6-22.21-49.6-49.6V139.1c0-27.39,22.21-49.6,49.6-49.6h199.97l24.29,80H849.9
			c27.39,0,49.6,22.21,49.6,49.6v409.8C899.5,656.29,877.29,678.5,849.9,678.5z"/>
		<polyline class="st1" points="434,385.79 434,541.45 693,541.45 		"/>
		<polygon points="570.06,404.03 297.94,404.03 434,268 			"/>
		</svg>`)
	embeddedFiles[audioImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
			 viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
			.st0{fill:#FFFFFF;stroke:#999999;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.4,0-49.6-22.2-49.6-49.6V139.1c0-27.4,22.2-49.6,49.6-49.6h200h24.3h451.5
			c27.4,0,49.6,22.2,49.6,49.6v489.8C899.5,656.3,877.3,678.5,849.9,678.5z"/>
		<path d="M511,212.7c-104.2,0-189,84.8-189,189v63c0,26,6.5,51.8,18.8,74.6c1.4,2.5,4,4.1,6.9,4.1h38.7c3.3,9.1,11.9,15.7,22.2,15.7
			c13,0,23.6-10.6,23.6-23.6V393.9c0-13-10.6-23.6-23.6-23.6c-10.3,0-18.9,6.6-22.2,15.7h-38.7c-2.9,0-5.6,1.6-6.9,4.1
			c-1,1.9-1.8,4-2.7,6c1.4-42.7,18.3-81.5,45.2-111l16.4,16.4c3.1,3.1,8.1,3.1,11.1,0C437.5,274.7,473.1,260,511,260
			s73.5,14.8,100.2,41.5c1.5,1.5,3.6,2.3,5.6,2.3s4-0.8,5.6-2.3l16.4-16.4c27,29.5,43.8,68.3,45.2,111c-1-2-1.7-4-2.7-6
			c-1.4-2.5-4-4.1-6.9-4.1h-38.7c-3.3-9.1-11.9-15.7-22.2-15.7c-13,0-23.6,10.6-23.6,23.6v141.8c0,13,10.6,23.6,23.6,23.6
			c10.3,0,18.9-6.6,22.2-15.7h38.7c2.9,0,5.6-1.6,6.9-4.1c12.3-22.8,18.8-48.6,18.8-74.6v-63C700,297.5,615.2,212.7,511,212.7z"/>
		</svg>`)
	embeddedFiles[videoImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
			 viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
			.st0{fill:#FFFFFF;stroke:#999999;stroke-width:64;stroke-miterlimit:10;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.4,0-49.6-22.2-49.6-49.6V139.1c0-27.4,22.2-49.6,49.6-49.6h200h24.3h451.5
			c27.4,0,49.6,22.2,49.6,49.6v489.8C899.5,656.3,877.3,678.5,849.9,678.5z"/>
		<path d="M716,289.9l-124,56.4v-57.8c0-23-18.6-41.6-41.6-41.6H349.6c-23,0-41.6,18.6-41.6,41.6v194.8c0,23,18.6,41.6,41.6,41.6
			h200.8c23,0,41.6-18.6,41.6-41.6v-77l124,61.6V289.9z"/>
		</svg>`)
	embeddedFiles[pdfImage] = []byte(`<?xml version="1.0" encoding="utf-8"?>
		<svg version="1.1" id="Layer_1" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" x="0px" y="0px"
			 viewBox="0 0 1024 768" style="enable-background:new 0 0 1024 768;" xml:space="preserve">
		<style type="text/css">
			.st0{fill:#FFFFFF;stroke:#999999;stroke-width:64;stroke-miterlimit:10;}
			.st1{fill:#D61716;}
			.st2{fill:#878482;}
			.st3{fill:#54504F;}
			.st4{fill:#FFFFFF;}
		</style>
		<path class="st0" d="M849.9,678.5H174.1c-27.4,0-49.6-22.2-49.6-49.6V139.1c0-27.4,22.2-49.6,49.6-49.6h200h24.3h451.5
			c27.4,0,49.6,22.2,49.6,49.6v489.8C899.5,656.3,877.3,678.5,849.9,678.5z"/>
		<path class="st1" d="M720.2,457.2c-0.3-3.1-3.1-38.7-66.4-37.2c-63.3,1.5-78.7,5.5-78.7,5.5s-47.3-47.9-64.5-85.1
			c0,0,20.9-61.2,20-99.6c-0.9-38.4-10.1-60.5-39.6-60.2c-29.5,0.3-33.8,26.1-29.9,64.6c3.5,34.5,21.3,75,21.3,75
			s-13.6,42.4-31.7,84.6c-18,42.2-30.3,64.3-30.3,64.3s-61,20.4-87.4,45c-26.4,24.6-37.2,43.5-23.3,62.4c12,16.3,54,20,91.5-29.2
			c37.5-49.2,54.4-79.9,54.4-79.9s57.2-15.7,75-20c17.8-4.3,39.3-7.7,39.3-7.7s52.2,52.6,102.7,50.7
			C723,488.6,720.6,460.3,720.2,457.2 M322.9,568.2c-31.4-18.7,65.8-76.5,83.3-78.4C406.2,489.8,355.7,587.8,322.9,568.2z M471.9,231
			c0-30.4,9.8-38.7,17.5-38.7c7.7,0,16.3,3.7,16.6,30.1c0.3,26.4-16.6,78.1-16.6,78.1C483.6,294.3,471.9,261.4,471.9,231z
			 M512.4,439.4c-31.4,7.7-47.2,15.7-47.2,15.7s0,0,12.9-28.9s26.1-68.2,26.1-68.2c17.8,33.2,53.2,72.2,53.2,72.2
			S543.8,431.7,512.4,439.4z M588,436.7c0,0,102.2-18.5,102.2,16.4C690.2,488,626.9,473.8,588,436.7z"/>
		<g>
			<polyline class="st2" points="156.9,184.3 62.4,184.3 62.4,273.3 359.8,273.3 359.8,184.3 156.9,184.3 	"/>
			<rect x="64.3" y="182.7" class="st1" width="297.1" height="89.1"/>
			<g>
				<g>
					<path class="st3" d="M217.4,227.7c-0.4,0-0.8,0-1.2,0v-19.3h2.5c2.9,0,4.7,0.9,6,2.4c1.6,1.8,2.4,5,2.4,7.3c0,3.2,0,6.1-2.9,8.2
						C222.6,227.3,220.1,227.7,217.4,227.7 M219.8,197.8c-0.1,0-0.3,0-0.4,0c-1.3,0-2.2,0-2.7,0h-13.9v62.6h13.4v-21.2l3,0.2
						c3.1,0,5.9-0.7,8.4-1.5c2.5-0.8,4.6-2.2,6.4-3.8c1.8-1.6,3.5-3.6,4.4-5.9c1.4-3.5,1.7-8.4,1.4-11.8c-0.4-3.4-0.5-6.2-1.5-8.4
						c-1-2.2-2.3-4-3.8-5.4c-1.6-1.4-3.3-2.4-5.1-3.1c-1.9-0.7-3.6-1.1-5.4-1.4C222.4,197.9,221.1,197.8,219.8,197.8"/>
					<path class="st4" d="M216.4,226.7c-0.4,0-0.8,0-1.2,0v-19.3h2.5c2.9,0,4.7,0.9,6,2.4c1.6,1.8,2.4,5,2.4,7.3c0,3.2,0,6.1-2.9,8.2
						C221.6,226.3,219.1,226.7,216.4,226.7 M218.8,196.8c-0.1,0-0.3,0-0.4,0c-1.3,0-2.2,0-2.7,0h-13.9v62.6h13.4v-21.2l3,0.2
						c3.1,0,5.9-0.7,8.4-1.5c2.5-0.8,4.6-2.2,6.4-3.8c1.8-1.6,3.5-3.6,4.4-5.9c1.4-3.5,1.7-8.4,1.4-11.8c-0.4-3.4-0.5-6.2-1.5-8.4
						c-1-2.2-2.3-4-3.8-5.4c-1.6-1.4-3.3-2.4-5.1-3.1c-1.9-0.7-3.6-1.1-5.4-1.4C221.4,196.9,220.1,196.8,218.8,196.8"/>
				</g>
				<g>
					<path class="st3" d="M262.8,248.9c-0.4,0-0.8,0-1.3,0V209c0.1,0,0.1,0,0.2,0c2.7,0,6.1,0,7.8,1.4c1.8,1.4,3.2,3.2,4.2,5.2
						c1,2.1,1.7,4.4,1.8,6.9c0.1,2.9,0,5.2,0,7.2c0,1.9,0,4.3-0.4,6.7c-0.4,2.4-1.2,4.5-2.2,6.6c-1.1,2-3,3.4-4.7,4.7
						C266.8,248.6,265,248.9,262.8,248.9 M265.2,197.6c-1.4,0-2.9,0.1-3.8,0.1c-1.7,0.1-2.7,0.1-3.1,0.1h-10.2v62.6h12
						c5.3,0,9.7-0.8,13.4-2.3c3.7-1.5,6.6-3.7,8.9-6.4c2.2-2.7,3.9-6,4.9-9.8c1-3.8,1.5-7.9,1.5-12.4c0-5.8,0-10.3-1.1-14.7
						c-1-4-3.1-7.2-5.3-9.6c-2.1-2.4-4.5-4.1-7-5.2c-2.5-1.1-4.9-1.9-7.2-2.3C267.3,197.7,266.2,197.6,265.2,197.6"/>
					<path class="st4" d="M261.8,247.9c-0.4,0-0.8,0-1.3,0V208c0.1,0,0.1,0,0.2,0c2.7,0,6.1,0,7.8,1.4c1.8,1.4,3.2,3.2,4.2,5.2
						c1,2.1,1.7,4.4,1.8,6.9c0.1,2.9,0,5.2,0,7.2c0,1.9,0,4.3-0.4,6.7c-0.4,2.4-1.2,4.5-2.2,6.6c-1.1,2-3,3.4-4.7,4.7
						C265.8,247.6,264,247.9,261.8,247.9 M264.2,196.6c-1.4,0-2.9,0.1-3.8,0.1c-1.7,0.1-2.7,0.1-3.1,0.1h-10.2v62.6h12
						c5.3,0,9.7-0.8,13.4-2.3c3.7-1.5,6.6-3.7,8.9-6.4c2.2-2.7,3.9-6,4.9-9.8c1-3.8,1.5-7.9,1.5-12.4c0-5.8,0-10.3-1.1-14.7
						c-1-4-3.1-7.2-5.3-9.6c-2.1-2.4-4.5-4.1-7-5.2c-2.5-1.1-4.9-1.9-7.2-2.3C266.3,196.7,265.2,196.6,264.2,196.6"/>
				</g>
				<g>
					<polyline class="st3" points="329.1,197.8 297.4,197.8 297.4,260.5 310.8,260.5 310.8,235.6 327.7,235.6 327.7,224 310.8,224 
						310.8,209.5 329.1,209.5 329.1,197.8 			"/>
					<polyline class="st4" points="328.1,196.8 296.4,196.8 296.4,259.5 309.8,259.5 309.8,234.6 326.7,234.6 326.7,223 309.8,223 
						309.8,208.5 328.1,208.5 328.1,196.8 			"/>
				</g>
			</g>
		</g>
		</svg>`)

	memoryFs = afero.NewMemMapFs()
	mmfile, _ := memoryFs.Create("video.svg")
	_, _ = mmfile.Write(embeddedFiles[videoImage])
	mmfile, _ = memoryFs.Create("audio.svg")
	_, _ = mmfile.Write(embeddedFiles[audioImage])
	mmfile, _ = memoryFs.Create("pdf.svg")
	_, _ = mmfile.Write(embeddedFiles[pdfImage])
}

func embeddedFileHandler(w http.ResponseWriter, r *http.Request, id embeddedFileId) {
	w.Header().Set("Content-Type", "image/svg+xml")
	http.ServeContent(w, r, r.URL.Path, time.Now(), bytes.NewReader(embeddedFiles[id]))
}
