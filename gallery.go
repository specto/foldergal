package main

import (
	"bytes"
	"errors"
	"fmt"
	"foldergal/embedded"
	"github.com/disintegration/imaging"
	"github.com/spf13/afero"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"mime"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
	thumb, _ := embedded.Fs.Open("res/audio.svg")
	return &thumb
}

func (f *audioFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(embedded.Fs, "res/audio.svg")
	if err != nil {
		return false
	}
	f.media().thumbInfo, err = embedded.Fs.Stat("res/audio.svg")
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
	if !f.thumbExists() {
		return nil
	}
	if Config.Ffmpeg == "" {
		thumb, _ := embedded.Fs.Open("res/video.svg")
		return &thumb
	}
	file, err := CacheFs.Open(f.thumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *videoFile) thumbExists() (exists bool) {
	if Config.Ffmpeg != "" { // Check if we generated thumbnail already
		exists, _ = afero.Exists(CacheFs, f.thumbPath)
		// Ensure we refresh thumb stat
		f.media().thumbInfo, _ = CacheFs.Stat(f.thumbPath)
		return
	}
	// Using internal images
	var err error
	exists, err = afero.Exists(embedded.Fs, "res/video.svg")
	if err != nil {
		return false
	}
	f.media().thumbInfo, err = embedded.Fs.Stat("res/video.svg")
	if err != nil {
		return false
	}
	return
}

func (f *videoFile) thumbExpired() (expired bool) {
	if !f.thumbExists() {
		return true
	}
	m := f.media()
	diff := m.thumbInfo.ModTime().Sub(m.fileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *videoFile) thumbGenerate() (err error) {
	if Config.Ffmpeg == "" { // No ffmpeg no thumbnail
		return
	}

	movieFile := filepath.Join(Config.Root, f.fullPath)

	// Get the duration of the movie
	cmd := exec.Command(Config.Ffmpeg, "-hide_banner", "-i",
		movieFile) // #nosec Config is provided by the admin
	out, _ := cmd.CombinedOutput()
	re := regexp.MustCompile(`Duration: (\d{2}:\d{2}:\d{2})`)
	match := re.FindSubmatch(out)
	if len(match) < 2 {
		return errors.New("error: cannot find video duration: " + f.fullPath)
	}
	// Target the first third of the movie
	targetTime := toTimeCode(fromTimeCode(match[1]) / 3)
	thumbSize := fmt.Sprintf("%dx%d", ThumbWidth, ThumbHeight)

	// Generate the thumbnail to stdout
	thumbCmd := exec.Command(Config.Ffmpeg,
		"-hide_banner",
		"-loglevel", "quiet",
		"-noaccurate_seek",
		"-ss", targetTime, "-i", movieFile,
		"-vf", "scale="+thumbSize+":flags=lanczos:force_original_aspect_ratio=decrease",
		"-vframes", "1",
		"-f", "image2pipe", "-") // #nosec Config is provided by the admin
	outThumb, _ := thumbCmd.Output()
	if len(outThumb) == 0 { // Failed thumbnail
		return errors.New("error: empty thumbnail: " + f.thumbPath)
	}
	err = CacheFs.MkdirAll(filepath.Dir(f.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = CacheFs.Create(f.thumbPath)
	if err != nil {
		return
	}
	// Save thumbnail
	err = afero.WriteFile(CacheFs, f.thumbPath, outThumb, os.ModePerm)
	f.media().thumbInfo, err = CacheFs.Stat(f.thumbPath)

	return
}

////////////////////////////////////////////////////////////////////////////////

type pdfFile struct {
	mediaFile
}

func (f *pdfFile) thumb() *afero.File {
	thumb, _ := embedded.Fs.Open("res/pdf.svg")
	return &thumb
}

func (f *pdfFile) thumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(embedded.Fs, "res/pdf.svg")
	if err != nil {
		return false
	}
	f.media().thumbInfo, err = embedded.Fs.Stat("res/pdf.svg")
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

func getMediaClass(name string) (class string) {
	ext := filepath.Ext(name)
	contentType := mime.TypeByExtension(ext)
	if strings.HasPrefix(contentType, "image/") {
		class = "image"
	} else if strings.HasPrefix(contentType, "audio/") {
		class = "audio"
	} else if strings.HasPrefix(contentType, "video/") {
		class = "video"
	} else if strings.HasPrefix(contentType, "application/pdf") {
		class = "pdf"
	}
	return
}

func validMedia(name string) bool {
	ext := filepath.Ext(name)
	contentType := mime.TypeByExtension(ext)
	match := mimePrefixes.FindStringSubmatch(contentType)
	return match != nil
}

// duration -> 00:00:00
func toTimeCode(d time.Duration) string {
	h := int64(math.Mod(d.Hours(), 24))
	m := int64(math.Mod(d.Minutes(), 60))
	s := int64(math.Mod(d.Seconds(), 60))
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// 00:00:00 -> duration
func fromTimeCode(timecode []byte) (d time.Duration) {
	m1, m2, m3 := 0, 0, 0
	m1, _ = strconv.Atoi(string(timecode[0:2]))
	m2, _ = strconv.Atoi(string(timecode[3:5]))
	m3, _ = strconv.Atoi(string(timecode[6:8]))
	d, _ = time.ParseDuration(fmt.Sprintf("%dh%dm%ds", m1, m2, m3))
	return
}
