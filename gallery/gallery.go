package gallery

import (
	"bytes"
	"errors"
	"fmt"
	"foldergal/config"
	"foldergal/storage"
	"github.com/disintegration/imaging"
	"github.com/spf13/afero"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"log"
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

var (
	logger *log.Logger
)

type Media interface {
	Media() *MediaFile // Expose our basic data structure in interface
	Thumb() *afero.File
	ThumbExists() bool
	ThumbGenerate() error
	ThumbExpired() bool

	File() *afero.File
	FileExists() bool
}

////////////////////////////////////////////////////////////////////////////////
type MediaFile struct {
	FullPath  string
	FileInfo  os.FileInfo
	ThumbPath string
	ThumbInfo os.FileInfo
}

func (f *MediaFile) Media() *MediaFile {
	return f
}

func (f *MediaFile) File() *afero.File {
	file, err := storage.Root.Open(f.FullPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *MediaFile) FileExists() (exists bool) {
	var err error
	exists, err = afero.Exists(storage.Root, f.FullPath)
	// Ensure we refresh File stat
	f.FileInfo, err = storage.Root.Stat(f.FullPath)
	if err != nil {
		return false
	}
	return
}

////////////////////////////////////////////////////////////////////////////////
type ImageFile struct {
	MediaFile
}

func (f *ImageFile) Thumb() *afero.File {
	if !f.ThumbExists() {
		return nil
	}
	file, err := storage.Cache.Open(f.ThumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *ImageFile) ThumbExists() (exists bool) {
	exists, _ = afero.Exists(storage.Cache, f.ThumbPath)
	// Ensure we refresh Thumb stat
	f.Media().ThumbInfo, _ = storage.Cache.Stat(f.ThumbPath)
	return
}

func (f *ImageFile) ThumbExpired() (expired bool) {
	if !f.ThumbExists() {
		return true
	}
	m := f.Media()
	diff := m.ThumbInfo.ModTime().Sub(m.FileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *ImageFile) ThumbGenerate() (err error) {
	var (
		file afero.File
		img  image.Image
	)
	file, err = storage.Root.Open(f.FullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return
	}
	img, err = imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		return
	}
	resized := imaging.Fit(img, config.Global.ThumbWidth,
		config.Global.ThumbHeight, imaging.Lanczos)
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, resized, nil)
	if err != nil {
		return
	}
	err = storage.Cache.MkdirAll(filepath.Dir(f.ThumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = storage.Cache.Create(f.ThumbPath)
	if err != nil {
		return
	}
	_ = afero.WriteFile(storage.Cache, f.ThumbPath, buf.Bytes(), os.ModePerm)
	f.Media().ThumbInfo, err = storage.Cache.Stat(f.ThumbPath)
	return
}

////////////////////////////////////////////////////////////////////////////////
type SvgFile struct {
	MediaFile
}

func (f *SvgFile) Thumb() *afero.File {
	if !f.ThumbExists() {
		return nil
	}
	file, err := storage.Cache.Open(f.ThumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *SvgFile) ThumbExists() (exists bool) {
	exists, _ = afero.Exists(storage.Cache, f.ThumbPath)
	// Ensure we refresh Thumb stat
	f.Media().ThumbInfo, _ = storage.Cache.Stat(f.ThumbPath)
	return
}

func (f *SvgFile) ThumbExpired() (expired bool) {
	if !f.ThumbExists() {
		return true
	}
	m := f.Media()
	diff := m.ThumbInfo.ModTime().Sub(m.FileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *SvgFile) ThumbGenerate() (err error) {
	var (
		file     afero.File
		contents []byte
	)
	file, err = storage.Root.Open(f.FullPath)
	defer func() { _ = file.Close() }()
	if err != nil {
		return
	}
	contents, err = afero.ReadAll(file)
	if err != nil {
		return
	}
	err = storage.Cache.MkdirAll(filepath.Dir(f.ThumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = storage.Cache.Create(f.ThumbPath)
	if err != nil {
		return
	}
	err = afero.WriteFile(storage.Cache, f.ThumbPath, contents, os.ModePerm)
	if err != nil {
		return
	}
	// Since we just copy SVGs the Thumb File is the same as the original
	f.ThumbPath = f.FullPath
	f.Media().ThumbInfo, err = storage.Cache.Stat(f.ThumbPath)
	return
}

////////////////////////////////////////////////////////////////////////////////
type AudioFile struct {
	MediaFile
}

func (f *AudioFile) Thumb() *afero.File {
	thumb, _ := storage.Internal.Open("res/audio.svg")
	return &thumb
}

func (f *AudioFile) ThumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(storage.Internal, "res/audio.svg")
	if err != nil {
		return false
	}
	f.Media().ThumbInfo, err = storage.Internal.Stat("res/audio.svg")
	if err != nil {
		return false
	}
	return
}

func (f *AudioFile) ThumbExpired() (expired bool) {
	return true
}

func (f *AudioFile) ThumbGenerate() (err error) {
	_ = f.ThumbExists()
	return
}

////////////////////////////////////////////////////////////////////////////////
type VideoFile struct {
	MediaFile
}

func (f *VideoFile) Thumb() *afero.File {
	if !f.ThumbExists() {
		return nil
	}
	if config.Global.Ffmpeg == "" {
		thumb, _ := storage.Internal.Open("res/video.svg")
		return &thumb
	}
	file, err := storage.Cache.Open(f.ThumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *VideoFile) ThumbExists() (exists bool) {
	if config.Global.Ffmpeg != "" { // Check if we generated thumbnail already
		exists, _ = afero.Exists(storage.Cache, f.ThumbPath)
		// Ensure we refresh Thumb stat
		f.Media().ThumbInfo, _ = storage.Cache.Stat(f.ThumbPath)
		return
	}
	// Using internal images
	var err error
	exists, err = afero.Exists(storage.Internal, "res/video.svg")
	if err != nil {
		return false
	}
	f.Media().ThumbInfo, err = storage.Internal.Stat("res/video.svg")
	if err != nil {
		return false
	}
	return
}

func (f *VideoFile) ThumbExpired() (expired bool) {
	if !f.ThumbExists() {
		return true
	}
	m := f.Media()
	diff := m.ThumbInfo.ModTime().Sub(m.FileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *VideoFile) ThumbGenerate() (err error) {
	if config.Global.Ffmpeg == "" { // No ffmpeg no thumbnail
		return
	}

	movieFile := filepath.Join(config.Global.Root, f.FullPath)

	// Get the duration of the movie
	cmd := exec.Command(config.Global.Ffmpeg, "-hide_banner", "-i",
		movieFile) // #nosec Configuration is provided by the admin
	out, _ := cmd.CombinedOutput()
	re := regexp.MustCompile(`Duration: (\d{2}:\d{2}:\d{2})`)
	match := re.FindSubmatch(out)
	if len(match) < 2 {
		return errors.New("error: cannot find video duration: " + f.FullPath)
	}
	// Target the first third of the movie
	targetTime := toTimeCode(fromTimeCode(match[1]) / 3)
	thumbSize := fmt.Sprintf("%dx%d", config.Global.ThumbWidth, config.Global.ThumbHeight)

	// Generate the thumbnail to stdout
	thumbCmd := exec.Command(config.Global.Ffmpeg,
		"-hide_banner",
		"-loglevel", "quiet",
		"-noaccurate_seek",
		"-ss", targetTime, "-i", movieFile,
		"-vf", "scale="+thumbSize+":flags=lanczos:force_original_aspect_ratio=decrease",
		"-vframes", "1",
		"-f", "image2pipe", "-") // #nosec Configuration is provided by the admin
	outThumb, _ := thumbCmd.Output()
	if len(outThumb) == 0 { // Failed thumbnail
		return errors.New("error: empty thumbnail: " + f.ThumbPath)
	}
	err = storage.Cache.MkdirAll(filepath.Dir(f.ThumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = storage.Cache.Create(f.ThumbPath)
	if err != nil {
		return
	}
	// Save thumbnail
	err = afero.WriteFile(storage.Cache, f.ThumbPath, outThumb, os.ModePerm)
	f.Media().ThumbInfo, err = storage.Cache.Stat(f.ThumbPath)

	return
}

////////////////////////////////////////////////////////////////////////////////
type PdfFile struct {
	MediaFile
}

func (f *PdfFile) Thumb() *afero.File {
	thumb, _ := storage.Internal.Open("res/pdf.svg")
	return &thumb
}

func (f *PdfFile) ThumbExists() (exists bool) {
	var err error
	exists, err = afero.Exists(storage.Internal, "res/pdf.svg")
	if err != nil {
		return false
	}
	f.Media().ThumbInfo, err = storage.Internal.Stat("res/pdf.svg")
	if err != nil {
		return false
	}
	return
}

func (f *PdfFile) ThumbExpired() (expired bool) {
	return true
}

func (f *PdfFile) ThumbGenerate() (err error) {
	if !f.ThumbExists() {
		return errors.New("no video thumbnail")
	}
	return
}

////////////////////////////////////////////////////////////////////////////////

// Check if any /path/./starts/with/.dot/somewhere
func ContainsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

var mimePrefixes = regexp.MustCompile("^(image|video|audio|application/pdf)")

// Find the type of a file
//
// Careful: on unix uses specific files
//   /etc/mime.types
//   /etc/apache2/mime.types
//   /etc/apache/mime.types
func GetMediaClass(name string) (class string) {
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

// Check for valid media by content-type
func IsValidMedia(name string) bool {
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

func Initialize(log *log.Logger) {
	logger = log
}
