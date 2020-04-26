package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"github.com/disintegration/imaging"
	"github.com/spf13/afero"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime"
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
	file, err := RootFs.Open(f.fullPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *mediaFile) fileExists() (exists bool) {
	var err error
	exists, err = afero.Exists(RootFs, f.fullPath)
	// Ensure we refresh file stat
	f.fileInfo, err = RootFs.Stat(f.fullPath)
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
	file, err := CacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *imageFile) thumbExists() (exists bool) {
	exists, _ = afero.Exists(CacheFs, f.thumbPath)
	// Ensure we refresh thumb stat
	f.media().thumbInfo, _ = CacheFs.Stat(f.thumbPath)
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
	file, err = RootFs.Open(f.fullPath)
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
	err = CacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = CacheFs.Create(f.thumbPath)
	if err != nil {
		return
	}
	_ = afero.WriteFile(CacheFs, f.thumbPath, buf.Bytes(), os.ModePerm)
	f.media().thumbInfo, err = CacheFs.Stat(f.thumbPath)
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
	file, err := CacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *svgFile) thumbExists() (exists bool) {
	exists, _ = afero.Exists(CacheFs, f.thumbPath)
	// Ensure we refresh thumb stat
	f.media().thumbInfo, _ = CacheFs.Stat(f.thumbPath)
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
	file, err = RootFs.Open(f.fullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return
	}
	contents, err = afero.ReadAll(file)
	if err != nil {
		return
	}
	err = CacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = CacheFs.Create(f.thumbPath)
	if err != nil {
		return
	}
	err = afero.WriteFile(CacheFs, f.thumbPath, contents, os.ModePerm)
	if err != nil {
		return
	}
	// Since we just copy SVGs the thumb file is the same as the original
	f.thumbPath = f.fullPath
	f.media().thumbInfo, err = CacheFs.Stat(f.thumbPath)
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

func validMedia(name string) bool {
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
	faviconImage
	css
)

var embeddedFiles = make(map[embeddedFileId][]byte)

var memoryFs afero.Fs

func unzip(data []byte) (out []byte, err error) {
	var (
		gz  *gzip.Reader
		buf bytes.Buffer
	)
	gz, err = gzip.NewReader(bytes.NewBuffer(data))
	if err != nil {
		return
	}
	_, err = io.Copy(&buf, gz)
	if err != nil {
		return
	}
	err = gz.Close()
	if err != nil {
		return
	}
	return buf.Bytes(), nil
}

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
	embeddedFiles[css] = []byte(`body
    {
        font-family: sans-serif;
        color: black;
        background: #EDEDED;
    }

    a
    {
        text-decoration: none;
        border-radius: 0.5em;
    }

    a.media { background: white; }
    a:hover { background-color: #D6D6D6; }

    a:active
    {
        background-color: #D1DDF0;
        color: cornflowerblue;
    }

    body > header a
    {
        color: black;
        display: inline-block;
        border-radius: 0;
        border-bottom-width: 2px;
        border-bottom-style: dotted;
        border-bottom-color: transparent;
        padding-right: 0.2em;
		max-width: 18em;
		overflow: hidden;
		white-space: nowrap;
 		text-overflow: ellipsis;
    }
	body > header span {
		overflow: hidden;
		display: inline-block;
		white-space: nowrap;
		color: silver;
		border-bottom-width: 2px;
        border-bottom-style: dotted;
        border-bottom-color: transparent;
	}

    body > header a:hover
    {
        border-bottom-color: black;
        background: none;
    }

    body > header a:active { border-bottom-color: cornflowerblue; }

    body > header a::after
    {
        content: ' \005C';
        display: inline-block;
        padding-left: 0.2em;
    }
	body > header a:only-of-type { color: silver; }
    body > header a:first-child::after,
	body > header a:last-of-type::after { content: ''; }

    body > footer
    {
        margin: 2em 1em;
        padding-top: 0.5em;
        color: gray;
        font-size: 0.8em;
        text-align: right;
    }

    body > footer::before
    {
        content: '…(˶‾᷄ ⁻̫ ‾᷅˵)…';
        display: block;
    }

    main ul
    {
        display: flex;
        flex-wrap: wrap;
        align-items: flex-start;
        flex-direction: row;
        padding: 0;
        margin: 0;
    }

    main li
    {
        display: flex;
        padding: 0.25em;
        margin: 0;
        list-style: none;
        align-items: center;
    }

    main a
    {
        width: 10em;
        min-height: 7em;
        display: flex;
        padding: 0.5em;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        color: black;
    }

    main a span
    {
        display: flex;
        flex-grow: 1;
        overflow-wrap: break-word;
        word-break: break-all;
        text-align: center;
        align-items: center;
        justify-content: center;
    }

    main a img
    {
        max-height: 10em;
        width: 10em;
        object-fit: contain;
        object-position: center top;
        display: inline-block;
    }

    main .title-container { position: relative; }

    main .title
    {
        position: absolute;
        right: 0;
        bottom: 0;
        display: flex;
        color: white;
        font-size: 0.8em;
        overflow: hidden;
        white-space: nowrap;
        text-overflow: ellipsis;
    }

    main .title
    {
        left: 0;
        padding-right: 0.4em;
        padding-left: 0.4em;
        width: 13em;
        background: rgba(0, 0, 0, 0.6);
        border-radius: 0 0 0.5em 0.5em;
        text-align: left;
        z-index: 5;
    }

    main .folder .title
    {
		align-items: center;
		justify-content: center;
		text-align: center;
		height: 90%;

        background: none;
        color: black;
		font-weight: bold;
        text-align: center;
        padding: 0 3em;
        width: auto;
		white-space: normal;
    }`)

	faviconGzip := []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x5d\x09\x54\xd5\xd5\xd6\xff\xdd\x0b\xc8\x27\xb1\x00\x3f\x50\x64\x50\x40\x1c\x10\x89\xd0\xe2\x33\x24\x07\x92\xa7\x25\x99\x69\x4e\xa0\x39\xa5\xa6\x99\x96\x96\x66\xb6\x5e\xa0\xf9\x1c\xcb\x65\x9a\x33\x3e\x9e\x2e\x2d\x7d\x84\x73\x3e\xc9\x09\xa3\x41\x2d\x96\x62\x39\x4f\x68\xa5\x17\x47\x48\x09\x50\x64\x7f\xeb\x9c\xbb\xff\xb7\xff\xbd\xde\x7b\xb9\x57\x41\xed\xfb\xfe\xbf\xb5\x8e\x07\xff\xc3\x3e\x7b\x9f\xff\x19\xf6\xd9\x7b\x9f\x73\x01\x1d\x5c\xd1\xa1\x83\xc8\x43\x11\xf1\x2c\xd0\x09\x40\x4c\x8c\xf1\xff\x6b\x9a\x00\x63\x9e\x05\x42\x43\xf9\xff\x3e\x40\xbd\x11\x80\x8f\x8f\xf1\xff\x23\x5c\x81\x5d\x13\x81\x08\x00\x1d\x00\xa4\xc1\x78\x5d\xa2\x83\x31\x6b\x06\x0d\x1a\x34\x68\xd0\xa0\x41\x83\x06\x0d\x1a\x34\x68\xd0\xa0\x41\xc3\x03\x82\x1e\x80\x2b\x27\xfd\xc3\x66\xa6\x06\x21\x64\xf3\x03\xf0\x14\x80\x14\x00\x7f\x07\xb0\x08\xc0\x5a\x00\x1b\x39\x89\xbf\x17\xf3\xbd\x14\x7e\xd6\xef\x2f\x5c\x2f\x82\xef\x86\x00\x92\x01\xa4\x03\x38\x08\xe0\x3a\x80\x3b\x00\xc8\xc5\xc5\x85\x3c\x3c\x3c\xc8\xd3\xd3\x93\x6a\xd7\xae\x4d\x7a\xbd\x9e\xc4\x75\x4e\x77\xf8\xd9\x83\xfc\x6e\x32\xd3\xfa\x2b\xd4\x45\x6d\x00\xed\x01\x2c\x00\x70\x12\xc0\x6d\x21\x93\x90\x2f\x38\x38\x98\x7a\xf4\xe8\x41\x33\x66\xcc\xa0\xac\xac\x2c\xda\xb3\x67\x0f\x7d\xf7\xdd\x77\xb4\x6b\xd7\x2e\x5a\xb3\x66\x0d\xa5\xa6\xa6\x52\xe7\xce\x9d\xc9\xcf\xcf\x4f\x5d\x17\xc4\x34\x4e\x32\xcd\xf6\x5c\xc6\xa3\x06\x0f\x00\x2f\x00\xd8\x00\xa0\x58\xe1\x5d\xa7\xd3\x51\xf3\xe6\xcd\x69\xe6\xcc\x99\x74\xec\xd8\x31\xba\x75\xeb\x16\xd9\x43\x69\x69\x29\xe5\xe5\xe5\xd1\x84\x09\x13\x64\x7d\x59\xd4\x03\x31\xed\x0d\x5c\x96\xc7\xc3\x16\x9a\xc7\x2e\xf1\x4d\xd6\x01\xb8\xa9\xe6\x55\xb4\xed\x31\x63\xc6\xd0\xd9\xb3\x67\xed\xca\x6c\x0d\x95\x95\x95\x74\xf0\xe0\x41\xea\xdd\xbb\x37\xb9\xb9\xb9\x59\xab\x87\x9b\x5c\x66\x07\xe6\xe1\x61\x20\x0c\xc0\x5c\x00\x57\x2d\xf9\xab\x5f\xbf\x3e\x65\x64\x64\x58\xfd\xde\xb7\x6f\xdf\xa6\x4b\x97\x2f\xd3\x89\x13\x27\xe8\xe7\xc3\x87\xe9\xd4\xe9\xd3\x74\xed\xda\x35\xba\x73\xe7\xce\x5d\xcf\xde\xb8\x71\x83\xa6\x4c\x99\x22\xeb\xd2\x4a\x1d\x10\x97\xfd\x09\xf3\xf2\xa0\x50\x0b\x40\x1f\x00\x87\xac\xf1\xe4\xef\xef\x4f\xeb\xd6\xad\xbb\x4b\x96\xcb\x97\x2f\xd3\xbf\x33\x33\xe9\xb5\x11\x23\xa8\x7d\x42\x02\x3d\x11\x13\x43\x2d\xa2\xa2\x28\xa6\x55\x2b\x4a\xec\xd4\x89\xc6\xbe\xfd\x36\x6d\xdb\xb6\x4d\xca\x6c\x59\x5f\x73\xe6\xcc\x91\xe3\xa5\x8d\x3a\x10\xe9\x27\xe6\xc9\xbd\x86\x65\x0f\x04\x30\xdf\xb2\xad\x2b\x49\xf0\xb8\x6c\xd9\xb2\xbb\xf8\xdf\xba\x75\x2b\x25\x75\xed\x4a\x0d\x42\x42\xc8\x3f\x20\x80\x02\x82\x82\x28\x30\x38\xd8\x94\xea\x07\x06\xca\xeb\x61\xe1\xe1\xd4\xff\x95\x57\x68\xdf\xfe\xfd\xb2\x0f\x28\x10\xed\x68\xe2\xc4\x89\x96\xf3\x84\xb5\x3e\x21\x78\x0b\xaa\x21\xd9\xe3\x00\xe4\x02\xa8\xb4\xc5\xc3\x6b\xaf\xbd\x46\x65\x65\x65\x66\xe3\xd9\xdc\x4f\x3e\xa1\xa6\x11\x11\x52\xc6\xa0\x06\x0d\xec\x26\x51\x17\xfe\x81\x81\xd4\xf2\xc9\x27\x29\xf3\x8b\x2f\xcc\xfa\xc4\x95\x2b\x57\x28\x31\x31\xd1\x9e\xfc\xc4\xbc\xe5\x32\xaf\xd5\x05\x57\xd6\x49\xce\xd9\x2b\x3b\x3c\x3c\x9c\x8e\x1e\x3d\x6a\xe2\xb7\xa2\xa2\x82\xe6\x7f\xfa\x29\x85\x84\x85\x49\xb9\xaa\x92\x5d\x9d\x44\xfb\x88\x6c\xd1\x82\x36\x6e\xdc\x68\xd6\x96\xb2\xb3\xb3\xc9\xdb\xdb\xbb\xaa\x3a\x20\xe6\x35\xa5\x1a\xc6\x46\x31\xd7\xbe\x0b\xa0\xa8\xaa\x32\xdf\x7f\xff\x7d\x33\x5e\xb7\xef\xd8\x41\x11\x91\x91\x4e\xcb\xae\x24\xd1\x5e\xe2\xe2\xe3\xe9\xc8\x91\x23\x26\x9a\xa2\x6d\xf5\xec\xd9\xd3\x11\xf9\x95\xb9\x72\xe2\x7d\xe8\x0b\x5e\x00\x3e\x06\x50\x56\x55\x59\x42\x67\xf9\xf1\xc7\x1f\x4d\x7c\x16\x15\x15\xd1\xcb\xbd\x7a\x39\xd4\xe6\xab\xaa\x83\xb7\xc7\x8f\x97\x63\x88\x82\xb5\x6b\xd7\xda\x9a\x13\xad\xa5\x32\x96\xc1\xcb\x49\xd9\xff\x1b\xc0\x52\x00\x15\x8e\x94\x93\x90\x90\x40\x37\x6f\xde\x34\xf1\xb8\xf5\x3f\xff\xa1\xd0\x46\x8d\xee\x4b\x76\x65\x3c\x78\xa2\x65\x4b\x3a\x7c\xf8\xb0\x89\x76\x41\x41\x01\x35\x6a\xd4\xc8\x51\xf9\x89\x65\x58\xc6\x32\x39\x2a\x7b\x86\xa2\xab\x3b\x92\xc6\x8d\x1b\x67\xd6\xf6\x27\x4c\x9c\x28\xc7\xf3\xfb\x95\x5f\xa9\x83\xf4\xf4\x74\x13\xed\xf2\xf2\x72\xea\xd2\xa5\x8b\x33\xf2\x13\xcb\xf2\x2f\x07\xea\xc0\x9b\xd7\x1c\x0e\xcb\x2e\xd2\xa2\x45\x8b\x4c\xfc\x89\xf9\xfb\xc5\x6e\xdd\xee\xbb\xed\x2b\x49\xd4\xe3\x9b\x6f\xbd\x65\x36\x1f\x8e\x1e\x3d\xda\x59\xf9\x95\x3a\x48\x67\x19\xad\xc1\x83\xf5\x39\x87\xda\xbc\x92\xc4\x5a\x2e\x33\x33\xd3\xc4\x9b\xc1\x60\xa0\xf8\xb6\x6d\xe5\x18\x5e\x2d\xf2\x07\x06\x52\xdf\x94\x14\xb3\x79\x75\xf2\xe4\xc9\xf7\x22\xbf\xd2\x17\xe6\xda\x58\x3b\x84\x02\x38\xec\x2c\x4d\x57\x57\x57\x5a\xbf\x7e\xbd\x89\xb7\xdf\x7e\xfb\x8d\x9e\x8e\x8b\xab\x36\xf9\x45\x3b\x12\x63\xa9\xd0\x25\x14\x4c\x9b\x36\xed\x5e\xe5\x27\x96\x31\xd4\x46\x1b\x10\xeb\xaa\x0b\xce\xd0\x13\x6b\xbc\x55\xab\x56\x99\xe9\x29\x09\x1d\x3b\x9a\xe4\x57\xf4\x3b\x91\xee\x65\x2e\x14\xed\x7f\xe0\xe0\xc1\x66\x73\xc0\xa4\x49\x93\xee\x55\xf6\x0b\x2c\xa3\x3d\xf4\x06\x50\xe8\x0c\x5d\xb1\xbe\x55\x8f\x4f\x42\x87\x55\xf4\xdc\xc6\x4d\x9b\x52\xe7\xe7\x9f\xa7\xe7\xbb\x74\x91\x7a\xa0\xb3\xed\x42\xd0\x49\x4d\x4b\x33\xd1\x17\xe3\xc0\xa0\x41\x83\xee\x45\xf6\x42\x96\xad\x2a\xe8\x78\x2d\x61\x70\x94\xf6\xc0\x81\x03\xcd\xc6\xa7\xb9\x73\xe7\x4a\xbe\x9f\x8c\x8d\x95\x3a\x9c\xd0\x07\x8a\x7f\xff\x5d\xae\x6f\x84\x4e\xe3\x4c\x1d\x08\xfd\x71\xcb\x96\x2d\x26\xda\xc5\xc5\xc5\x14\x17\x17\xe7\xac\xec\x06\x96\x49\xe7\x80\xfc\x4a\x1d\x74\x07\x70\xde\x11\xfa\x51\x51\x51\x72\xdc\x53\x70\xe8\xa7\x9f\x28\x3a\x26\x46\xea\xbf\x96\x58\xb1\x62\x05\x35\x68\xd8\xd0\xe1\xbe\xff\xb7\x4e\x9d\xa8\xb0\xb0\xd0\xf4\x7e\x5e\x5e\x9e\x35\x1b\x91\xbd\x74\x9e\x65\x71\x54\x76\x35\x12\x01\x1c\xa9\xaa\x0c\x77\x77\x77\xb3\x35\xaf\xd0\xfd\xa7\xfe\xe3\x1f\x94\x9b\x9b\x7b\x97\xfc\x07\x0e\x1c\x70\x58\x2f\x16\xeb\xc5\x8c\x8c\x0c\xb3\xf7\xa7\x4f\x9f\xee\x8c\xec\x47\x58\x86\xfb\x41\x4b\x00\x5f\x57\x55\xd6\xcb\x2f\xbf\x4c\x7f\xfc\xf1\x87\x89\xcf\x5f\x7f\xfd\x95\x4e\x9d\x3a\x75\x97\xfc\x62\x0d\x23\xd6\xb9\x8e\x7c\xfb\xa1\xc3\x87\xcb\x7e\xa3\xe0\xc2\x85\x0b\xd4\xb2\x65\x4b\x47\x65\xff\x9a\x79\xaf\x0e\x34\x00\xb0\x02\xc0\x2d\x5b\xe5\x79\x7a\x7a\x9a\xb5\x01\x31\x1e\x88\x31\x5b\xac\x61\x95\xb1\x41\xd4\x49\x72\x4a\x8a\xdd\xfe\xaf\xcc\x17\x62\xce\x13\xba\xae\x1a\xb3\x66\xcd\xaa\xca\x06\x40\xcc\xe3\x4a\xe6\xb9\x3a\xe1\x09\x60\x3c\x80\x4b\xb6\xca\x8e\x8d\x8d\xbd\x8b\xe7\x92\x92\x12\xfa\x6a\xfb\x76\xd9\x1f\x12\x3b\x75\xb2\xd9\xee\x15\xb9\xc5\x78\x37\xf2\xf5\xd7\xe9\xdc\xb9\x73\x66\x74\xbe\xff\xfe\x7b\x6a\xd0\xa0\x41\x55\xb2\x5f\x62\x1e\x3d\xab\x59\x76\x05\x7a\x00\x1d\x01\x7c\x6b\xcb\x0e\xd2\xaf\x5f\x3f\xba\x7e\xfd\xba\x19\xef\x86\xc2\x42\x9a\x37\x7f\x3e\x75\x78\xf6\x59\xb9\x2e\x52\xec\x3d\x4a\x12\xff\x0f\x6f\xd2\x84\x5e\x7c\xe9\x25\x5a\xb3\x76\xad\xd9\x5a\x4a\xe0\xcc\x99\x33\xf4\xcc\x33\xcf\xd8\x93\xbb\x92\x79\xea\xf8\x80\xfc\x04\x01\x00\xa6\x01\xb8\x6c\xc9\x8b\xd0\x87\x87\x0f\x1f\x4e\x57\xaf\x5e\x35\x93\x41\xf4\x01\x31\x8e\x8b\xfe\x3f\xfb\xa3\x8f\xe8\xad\xb1\x63\x69\xc4\xc8\x91\xf4\xf6\x3b\xef\xc8\xba\xf9\x3a\x37\x57\xce\x93\x96\x38\x7d\xfa\x34\x3d\xf7\xdc\x73\xf6\x64\x17\x3c\x4c\x67\x9e\x1e\x24\x5c\x00\xb4\x03\xb0\x09\x40\xa9\x9a\x27\xd1\x47\xbb\x77\xef\x6e\x66\xbb\xb0\x84\xa8\x0f\x6b\x36\x5f\xf5\x7d\x31\x7f\xd8\x99\xeb\x4b\xb9\xec\x76\xcc\xcb\xc3\x82\x27\xeb\x16\x39\x96\xb6\x92\xa6\x4d\x9b\xd2\xc2\x85\x0b\xa5\xed\xd7\x51\x08\xb9\x45\xdf\x4f\x4b\x4b\x93\x76\x74\x2b\x72\x97\x03\xd8\xc3\x65\xd6\x54\x3f\xbf\x17\xf8\x30\x4f\x5f\xaa\x7d\x40\x6e\x6e\x6e\xf4\xd4\x53\x4f\xc9\x79\x7b\xff\xfe\xfd\xb2\x5f\xa8\x75\x79\x62\xfb\xae\xe8\x1b\x39\x39\x39\xd2\xce\xdb\xac\x59\x33\x6b\xe3\xbc\xa0\xb9\x15\x40\x5f\x2e\xeb\x51\x85\x58\x5b\xb6\x05\x30\x1b\xc0\x01\x00\x25\xe0\x75\x92\x8f\x8f\x0f\xc5\xc4\xc4\xd0\x4b\x2f\xbd\x44\xc3\x86\x0d\xa3\x51\xa3\x46\xd1\x90\x21\x43\x28\x29\x29\x89\x22\x23\x23\xad\xf9\x39\x4a\x98\xc6\x47\x4c\xf3\x51\xf0\x79\x39\x0a\x1d\xfb\xb0\x13\x00\x4c\x02\x90\xc5\x3e\x13\x03\xdb\xea\x6f\xf1\x7a\xbc\x82\xff\xbe\xc9\xf7\x0e\xf1\xb3\xe2\x9d\x67\x99\xc6\xbd\xe8\xae\x8f\x1a\x5c\x59\x96\x08\x00\xcf\xf0\x3a\xb4\x27\xa7\x17\xf8\x5a\x04\x3f\xf3\xb0\x7c\x7a\x1a\x34\x68\xd0\xa0\x41\x83\x06\x0d\x1a\x34\x68\xd0\xa0\x41\xc3\xff\x49\x38\x6c\x98\xab\x7e\x94\x85\x18\x73\x66\xa2\x0c\x90\x17\x0a\x00\x77\x91\xa7\x01\x3a\x22\xaa\x14\x5c\xa6\x12\x55\x88\xbc\x3d\x51\x91\xc8\xbd\xe5\x63\xf2\xc1\x34\x91\xeb\x04\x15\x29\x4e\xa5\x31\x4f\xad\x30\xe6\xed\xcb\x8c\x79\x48\x91\x31\xf7\x2e\x30\xe6\xee\x39\xc6\xdc\x45\xc9\x95\xeb\x4a\xae\x3c\xaf\xbc\xaf\xd0\x55\xca\x31\x95\x2b\x09\xb8\xfc\xc9\x8f\x7c\x21\xe4\x4f\x7e\x49\xa9\xe5\x1c\xf9\x98\x14\xc0\x5b\xe4\x15\xc6\xdb\x44\x39\x2e\x0f\xe9\x03\x08\x44\x00\x88\x01\x30\x48\x7d\x4e\x44\x63\x63\xa6\x9d\x13\xa1\x41\x83\x06\x0d\x1a\x34\x68\x78\x04\xa1\x03\xe0\xc6\x31\xfe\xb5\xf9\xef\x47\xd9\x57\xe6\xc6\x7b\xb5\xba\x02\x78\x0f\xc0\x72\x00\x9b\x01\xec\xe2\xb4\x99\xaf\x4d\xe2\x67\xc2\xf8\x9d\x87\x09\x1d\xef\x7b\x1a\x00\xe0\xdf\x00\x0a\xf4\x7a\x7d\xb9\xbf\xbf\x3f\xb5\x69\xd3\x86\xfa\xf4\xe9\x23\x7d\xb8\xc9\xc9\xc9\xd4\xae\x5d\x3b\x0a\x0c\x0c\x94\x31\x20\xec\xa7\x2f\xe0\x77\x06\xf0\xbe\xa4\x07\xf9\x6d\x44\x59\xe1\xbc\xa7\xf8\x30\x80\xdb\x7a\xbd\x5e\xc6\x8f\xcd\x99\x33\x87\x7e\xfe\xf9\x67\x19\xab\xae\xc4\x60\x54\x56\x56\xca\x58\xa5\xe3\xc7\x8f\xd3\xe2\xc5\x8b\x29\x3e\x3e\x5e\xc6\x36\xb3\x1f\xfa\x36\xd3\xf8\x80\x69\xd6\xb4\x1c\xf5\x38\x8e\xe9\x84\x12\x67\xe4\xe5\xe5\x45\xef\xbd\xf7\x9e\x8c\x81\x73\x04\x57\xae\x5c\x91\x71\xb9\x75\xeb\xd6\xb5\x8c\x13\x3a\xc9\xb4\xeb\xd5\x00\xdf\xae\x00\x9e\x07\xf0\x8d\x3a\x8e\xde\xcf\xcf\x4f\xc6\x31\x2a\x71\x0f\xa2\xae\x85\x1c\x9b\xb7\x6c\x91\xb1\x40\x7f\xff\xe0\x03\x9a\x31\x73\x26\x65\xad\x5b\x27\x63\x3f\xd4\xdf\x64\xc3\x86\x0d\xd6\xe2\xbe\x2a\x38\xde\xa9\x4b\x35\xfa\x8a\xeb\x72\x1c\xd1\x75\x75\x59\x1e\x1e\x1e\xb2\x3d\x28\x31\x7c\xc5\xc5\xc5\xb4\x64\xc9\x12\xb9\x87\xb1\x61\x68\xa8\x29\x96\x4b\xe4\x0d\x42\x42\x64\x7c\xef\x47\x1f\x7f\x6c\x16\xf3\x92\x99\x99\x49\x75\xea\xd4\xb1\x16\xcb\x72\x9d\xcb\xac\x7b\x9f\xbc\x3f\x01\x60\x9b\xb5\x7d\x1b\xaf\xbe\xfa\xaa\x69\xef\xc0\xa5\x4b\x97\x68\xd4\x1b\x6f\x50\x70\xc3\x86\x36\x63\x10\x95\xbd\x89\xfd\x07\x0c\x30\xc5\xe0\x89\xef\x21\xda\x9e\x8d\x38\x24\x51\x66\x36\xf3\xe0\x2c\x74\xfc\x0d\x8f\x5a\xa3\x1d\x14\x14\x24\xf7\x01\x13\xef\x25\x7c\x67\xfc\x78\x87\x63\xa7\xc5\xf7\x18\x32\x74\xa8\x29\xa6\xec\xec\xd9\xb3\x72\x2f\xb6\x9d\x38\xb2\xa3\xcc\x8b\xa3\x7d\xdb\x05\xc0\x40\x7b\x71\xe2\x62\x5c\x54\xda\xf2\xa6\xcd\x9b\x65\xdc\xab\x33\x7b\x00\xc4\x77\xfa\xa7\x2a\xfe\xd7\xce\x37\x50\x92\x81\x79\xaa\x2a\x1e\x4c\xf4\x99\x51\x96\x6d\xdd\x32\x8e\xef\xf3\xcf\x3f\x37\xd5\xfd\x2b\x03\x07\x3a\xbd\xa7\x49\x7c\x83\xa4\xae\x5d\x4d\xf1\x90\x3b\x77\xee\xac\x6a\x1f\xaf\xd2\x27\x46\xd9\xe9\xd7\x2e\x7c\xff\x77\x7b\x74\x7c\x7d\x7d\x4d\x6d\xe7\xd4\xa9\x53\x72\x4f\xa9\xb3\xfb\x2f\xc4\xf3\xcd\x9a\x37\xa7\x1f\xf3\xf2\x24\x9d\x82\x82\x02\x0a\x0d\x0d\xad\x8a\x7f\x62\xde\xde\xb0\xf1\x1d\x86\x38\xb2\x9f\x52\x94\x73\xfe\xfc\x79\x59\xee\xb7\xdf\x7e\x2b\xe3\x43\x9d\xdd\x3b\xa2\xc4\x90\x6f\xdc\xb4\x49\xd2\xb9\x76\xed\x9a\x33\xf1\xd3\x45\xcc\xab\x65\xbb\x59\xec\xc8\xfb\xe1\xe1\xe1\x32\xde\x59\x20\x67\xcf\x1e\xb3\x98\x6f\x67\xfb\xc0\x17\x59\x59\x92\x8e\xe8\xcb\xb1\xb1\xb1\x8e\xf2\x4f\xcc\xab\x65\x3b\x0a\xe4\xb8\x49\xbb\xef\x0a\x3d\xe6\xc4\x89\x13\xb2\xdc\x83\xf9\xf9\xd4\xbc\x45\x0b\x39\xf6\x88\xfa\x8c\x88\x8c\x94\x63\xbf\x23\x72\x08\xb9\x77\xed\xde\x2d\xe9\x18\x0c\x86\xaa\xc6\x20\x75\xda\xcc\xbc\x5a\x43\x43\x00\x5b\xec\xbd\x5f\xbb\x76\x6d\xda\xb1\x63\x87\x2c\xf7\xea\xd5\xab\x32\xf6\xba\xc5\xe3\x8f\x53\xfa\xf2\xe5\x74\xe8\xd0\x21\x5a\xb1\x72\x65\x95\x7d\x42\xc8\xdb\xfa\xe9\xa7\x4d\xf3\xc0\x81\x03\x07\x64\xbf\x72\x80\xf7\x2d\xcc\xa3\x3d\x04\x03\xc8\xb4\xb7\x9f\xfc\xc3\x0f\x3f\x34\x8d\x7d\xd3\x67\xcc\xa0\xb4\xc9\x93\xcd\x74\x1c\xa1\x3b\xd8\x9b\x0f\xc4\xf8\x33\xfe\xdd\x77\x4d\x63\xf0\xd2\xa5\x4b\x65\x1c\xa2\x1d\xbe\x05\x2f\x5f\x30\x6f\x8e\xc0\x8f\xdb\x58\xb9\x35\x7a\xad\x5b\xb7\x36\xe9\x01\x62\xfe\xf9\xe6\x9b\x6f\xcc\xf8\x5f\xb5\x6a\x95\xcd\xfa\x17\xbc\x8b\xba\x17\x7a\x2a\x71\x1c\x7d\x15\xfb\x3e\xcb\x99\x17\x3f\x07\x79\x57\xf0\x18\xef\x93\xbe\x66\x49\x53\xe8\xbf\x42\xf7\x51\x74\x31\x31\x0f\x54\x54\x54\xc8\xfa\x2c\x2c\x2c\x94\xfb\xbc\xac\xd5\xbf\xe0\x3d\x2a\x3a\x5a\xea\x6e\x0a\xb2\xb2\xb2\xe8\xb1\xc7\x1e\xb3\xc5\xfb\x35\xe6\xe1\x31\x27\x79\x57\x20\xc6\xd9\x17\xf9\x1c\x08\x33\xda\x4d\x9a\x34\x31\xcd\x03\xca\x18\xb2\x6a\xf5\x6a\x4a\x7a\xe1\x05\xc9\xa7\xe0\x5f\x49\x62\x7e\x13\xdf\x43\xf4\x95\x6d\xd9\xd9\xa6\x76\x73\xe6\xcc\x19\x19\xc3\x6b\x83\x77\x51\x66\xb7\x6a\x8a\xc1\x6e\x0c\xe0\x9f\x4a\xcc\xac\x92\x12\x12\x12\x64\x1c\xbc\x82\x8b\x17\x2f\xd2\xbf\x56\xac\xa0\x41\x83\x07\x53\xbb\x0e\x1d\x28\xb6\x75\x6b\x7a\xa6\x5d\x3b\x4a\xe9\xdf\x9f\x96\x2e\x5b\x26\xf7\x60\xaa\x9f\xed\xd1\xa3\x87\x35\xbe\x4b\xb8\xac\x26\xd5\xc0\xb7\x1a\xff\xc5\xfb\xec\xf6\xa9\xd7\x00\x6d\xdb\xb6\xa5\x7d\xfb\xf6\x99\xed\x93\x2b\x2f\x2f\x97\x3a\xa9\x98\xe7\x44\x7b\x52\xef\x71\x15\x10\x6d\x3f\x29\x29\xc9\xb2\xcf\x56\x30\xed\xde\x5c\x56\x4d\xc1\x1f\xc0\x5b\x00\xf2\x15\x39\x82\x83\x83\x69\xea\xd4\xa9\xf2\x5b\xd8\x8a\xdd\x17\xf2\x89\x79\x6f\xee\xdc\xb9\xb2\xed\x59\xf0\x9d\xcf\x34\xfd\x6b\x90\x6f\x4b\x04\x00\x18\xca\x3a\xfa\x35\xbd\x5e\x5f\x19\x16\x16\x26\xf7\x2c\x2e\x5c\xb8\x50\x9e\x4d\x92\x93\x93\x23\xf7\x26\xa6\xa7\xa7\xcb\x33\x3c\x22\x22\x22\x94\x35\x7c\x25\xf7\xcd\x6c\xa6\xf1\xa0\xf7\x58\xa8\xe1\x01\x20\x16\xc0\x38\xb6\x29\xe4\xeb\x74\x3a\x43\xad\x5a\xb5\x6e\x78\x78\x78\x94\xba\xbb\xbb\x97\xea\x74\xba\x1b\xaa\x18\xea\x4c\x7e\x36\xf6\x11\x8c\x1b\x57\xc7\x42\xb7\xe1\x3d\x7a\x89\xfc\xb7\x16\x03\xad\x41\x83\x06\x0d\x1a\x34\x68\x78\x24\x60\xd4\x6e\x6b\x22\x2f\x0b\x31\xe6\x69\x32\x88\xcb\x18\x16\x05\x18\xc3\xa4\x00\x63\x38\x18\x60\x0c\x07\x33\x2a\xd2\xe2\x1f\x63\x58\x98\x71\xc3\x29\x42\x8c\xc6\x2a\x78\x1b\x1d\x4e\x70\x37\x6e\x46\x15\x2b\x32\x63\x98\xd6\x9f\xb9\x72\x5d\x79\x4e\x79\x4f\xa1\x63\x0c\xdb\xfa\x93\xbe\xa9\x5c\x19\xbe\xa5\xe2\x4b\x86\xab\x09\xbe\x95\x54\xe0\x5e\x33\xf5\x63\xcc\x23\xf8\xd0\x9a\x0e\xea\x38\x2d\xde\x05\xaa\xc5\x69\x69\xd0\xf0\x97\x86\x8e\xcf\x2e\xf5\xe2\xe4\xfe\x00\x7c\xcc\xb5\x00\xb4\x00\x30\x4c\xa7\xd3\x2d\xf1\xf2\xf2\xfa\x2a\x20\x20\x60\x9f\x9f\x9f\xdf\x3e\x37\x37\xb7\xaf\x00\x2c\x11\xf7\x00\x44\xf1\xb3\xd5\x05\xb1\x16\x4f\x02\xb0\xd6\xd5\xd5\xd5\x10\x17\x17\x57\x39\x6b\xd6\x2c\xda\xbd\x7b\xb7\xb4\x4b\xff\xf0\xc3\x0f\xb4\x7a\xf5\x6a\x4a\x49\x49\xa1\x3a\x75\xea\x54\xf2\x7a\x7e\x2d\xbf\x73\x3f\xeb\x78\x3d\xaf\xaf\xb3\x00\xdc\xf4\xf2\xf2\x92\x67\xeb\x2a\xe7\x54\x14\x17\x17\x4b\x3b\xb7\xc1\x60\x90\x7e\x6a\x91\xb2\xb3\xb3\xd5\xfe\x92\x12\x7e\xb7\xcd\x3d\x9c\xb1\xe1\xc3\x31\x1d\x06\xc5\x36\x3f\x6f\xde\x3c\x69\x43\x3a\x77\xfe\x3c\x7d\x38\x75\xaa\xb4\x39\xb6\x7c\xf2\x49\x7a\xba\x4d\x1b\xe9\xb7\xdc\xb5\x6b\x97\xbc\x2f\xea\x24\x3a\x3a\x5a\x6d\x47\x32\x30\x2d\x47\xcf\x00\x68\xc2\x7c\x9b\x6c\x71\x03\x06\x0c\x90\xb6\xb7\xa3\x47\x8f\x52\x17\xb6\x83\x2a\xe7\x93\x29\x76\xd0\x88\xc8\x48\x93\xef\x71\xfd\xfa\xf5\x96\x7b\xf3\x2b\x98\x66\x55\xb6\xc3\xff\x01\xb0\x5f\x6d\x77\xf4\xf1\xf1\x91\x76\xf0\xd2\xd2\x52\x79\xae\x99\x2d\x9f\xa2\xe0\x23\x2a\x3a\x9a\xbe\xdf\xbb\x57\xda\x05\x6d\xd8\xbe\xf7\x73\x19\xd6\xd0\xd6\x9a\x5f\x39\x2e\x2e\x4e\x9e\xaf\xf2\x75\x6e\xae\xf4\xa9\xd9\xf3\x7f\x08\xde\xc6\x4f\x98\x20\xeb\x60\xce\x9c\x39\xb6\xec\xbf\x47\xf9\x0c\x0e\x4b\xb9\x8f\x59\x7b\xbe\x6f\xdf\xbe\x92\xde\xe2\x25\x4b\xaa\xf4\x65\x2b\xfe\x53\x51\x57\x9b\x36\x6d\x52\xc7\xa3\x58\xa6\x63\x16\xf5\xd0\x5d\x7d\x06\x85\x3a\xf5\xef\xdf\x5f\x96\x3f\x6f\xfe\x7c\x53\xdc\x82\xad\xb3\xe9\xc4\xf5\xce\xcf\x3d\x27\xeb\x6b\xeb\xd6\xad\x54\xab\x56\x2d\x5b\xe5\x17\x73\x99\x0a\x5c\x00\xbc\x6e\x8d\x87\xc4\xc4\x44\x79\xde\xc5\xa6\xcd\x9b\xa5\x3f\x6a\xf2\x94\x29\xb2\xfd\x47\x46\x45\xdd\xc5\x83\xe0\x6f\xe8\xb0\x61\x92\xdf\xf4\xf4\x74\x7b\x65\x8f\xb2\x62\x7f\x77\x61\x9b\xa5\xd9\x79\x45\x41\x41\x41\xb2\xdd\x5f\xbc\x78\x91\x3e\x5d\xb0\x40\xf6\x03\xd1\xcf\xde\x1c\x3b\xd6\xac\x2d\x0a\x5e\x42\xc2\xc2\x4c\x67\x3a\xd9\x38\x73\xee\x12\x97\x61\xcb\xf6\xaf\x67\xbf\xc4\x71\xf5\x7b\x69\x69\x69\xd2\x4e\x7c\xfd\xfa\x75\x59\x17\x62\xcc\x49\xee\xd7\xcf\x74\x66\xa6\x12\x83\xf1\xce\xf8\xf1\xb2\xed\xe7\xe5\xe5\x51\x40\x40\x80\x65\xd9\xc7\x99\xb6\x23\x63\x51\x34\xfb\x3c\xe5\x79\xfc\x82\x96\xe2\x1b\x14\x65\x7f\x90\x9a\x2a\xfd\xdc\xc1\x0d\x1b\x4a\x99\x5b\xc7\xc5\x49\xdf\x5a\x51\x71\xb1\xf4\x19\x77\xef\xde\x5d\x5d\xee\x6d\xa6\x15\xed\x40\xb9\x6a\x78\x03\x78\x13\xc0\x29\x41\xa7\x59\xb3\x66\xd2\xe6\x2c\xea\xbe\xa8\xa8\x88\xf6\xee\xdd\x4b\xeb\x37\x6c\x90\x67\x45\x29\x7e\xef\x5f\x7e\xf9\x45\x8e\x55\xaa\xb3\x50\x4e\xb1\xfd\xdc\xd6\x19\x9e\x55\x41\xcc\xa9\x4d\xf9\xec\xa2\x93\x7e\x7e\x7e\x15\x63\xc6\x8c\x91\xfe\x75\x21\xa7\x68\x0b\x25\x25\x25\x74\xf2\xe4\x49\xe9\xf7\x6a\xd5\xaa\x15\xe9\x74\xba\x0a\x8e\x99\x9a\xc6\xef\x56\xc7\xbc\xac\x33\xae\x36\x30\x18\xc0\x2a\x5f\x5f\xdf\xfc\x56\xad\x5a\x19\x3a\x76\xec\x58\x1c\x1f\x1f\x5f\x1c\x12\x12\x62\x70\x75\x75\xcd\x07\xb0\x9a\x7d\xf0\xa1\x35\x78\xae\x93\x68\xbb\xbe\x6c\x77\x8e\xe5\xa4\xd8\xa0\x1f\xe6\xb9\x42\x1a\x34\x68\x70\x12\x56\x9d\x92\x0a\xca\x42\x8c\xbb\xcd\xd8\xcc\x52\x66\xdc\x17\xe7\x5d\x60\xdc\x0f\xe7\x92\x06\xe8\x94\x94\x63\xdc\x13\xe7\x2e\x9e\x11\xcf\x56\x02\xa9\xd2\x44\x53\xe4\x6d\xb7\x88\x08\x56\x4c\x43\xd5\x76\x0a\xf6\x64\xfd\x3f\xb1\x53\xb8\xf3\x78\xea\xeb\xc4\xef\x51\x88\xf9\xa0\x91\x97\x97\xd7\xb8\xe4\xe4\xe4\x2f\x67\xcf\x9e\x7d\x28\x35\x35\xf5\x50\x7c\x7c\xfc\x97\x7a\xbd\x7e\x9c\xb8\x67\x67\xde\xf1\x14\xfa\x8f\x87\x87\xc7\x89\x29\x53\xa6\x54\xee\xdc\xb9\x53\xae\x2d\xd2\x97\x2f\x97\x7e\xdd\x9e\x3d\x7b\x56\x72\x1c\xf1\x28\x2b\xe7\xa5\xd5\xe7\xb3\xce\xcb\x7b\xf5\xea\x45\xdb\xb7\x6f\x97\xb1\x04\xca\xd9\xbd\x7d\xfa\xf6\x95\xbf\xd3\xd3\xb8\x71\x63\x25\xbe\x24\x83\xdf\x01\xfb\x82\xd7\x2b\x7a\xc1\xa2\x45\x8b\x68\xec\xb8\x71\xe4\x5b\xb7\xae\x99\x1e\xf7\xd9\x67\x9f\xc9\x33\x11\x55\xfa\xc3\x7a\x7e\x37\x94\x7f\xab\x41\x5e\x5f\xba\x6c\x99\x3c\x37\xbd\x6b\xb7\x6e\xf2\x9c\x60\x65\x2d\xb2\x62\xe5\x4a\xe9\xab\x56\xbd\x9f\xab\xfa\xbd\x93\x66\x1c\xf7\x5a\x39\x78\xf0\x60\x19\xbb\x2d\x52\x42\xc7\x8e\xe4\x57\xaf\x1e\xfd\xad\x73\x67\xa9\xe7\x3f\xfe\xf8\xe3\x8a\x2f\x7b\x9b\x95\x66\x58\x0f\xc0\x0c\x1f\x1f\x9f\xcb\x42\xc7\xcc\xcf\xcf\x97\xf1\xc8\x42\x27\x15\xb2\x8f\x1c\x39\x52\xe8\x00\x97\xc5\x33\x76\x62\xa6\x45\x0b\x7f\xba\x7e\xfd\xfa\xf3\x46\x8f\x1e\x7d\x20\x23\x23\xe3\xc2\x82\x05\x0b\x2e\x76\xeb\xd6\xed\x80\xbb\xbb\xfb\x3c\x71\xcf\x41\x7f\xae\x9e\xcf\x68\x6f\xca\xc9\xf7\x2f\xf2\xbb\x4f\x4e\x83\x54\xb6\xd3\x8a\xf6\x40\x81\x3b\x90\xe3\x02\xa4\xa9\x5a\xb9\xf8\x5b\x5c\x13\xf7\xca\x42\xcc\xdf\xf9\xdf\x00\x00\x00\xff\xff\x7d\x44\x96\x72\x26\x7d\x00\x00")
	faviconData, _ := unzip(faviconGzip)
	embeddedFiles[faviconImage] = faviconData

	memoryFs = afero.NewMemMapFs()
	mmfile, _ := memoryFs.Create("video.svg")
	_, _ = mmfile.Write(embeddedFiles[videoImage])
	mmfile, _ = memoryFs.Create("audio.svg")
	_, _ = mmfile.Write(embeddedFiles[audioImage])
	mmfile, _ = memoryFs.Create("pdf.svg")
	_, _ = mmfile.Write(embeddedFiles[pdfImage])
}
