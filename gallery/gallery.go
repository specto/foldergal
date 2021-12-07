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
	"math"
	"mime"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
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
	defer file.Close()
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

func (f *SvgFile) ThumbExists() bool {
	exists, _ := afero.Exists(storage.Cache, f.ThumbPath)
	// Ensure we refresh Thumb stat
	f.Media().ThumbInfo, _ = storage.Cache.Stat(f.ThumbPath)
	return exists
}

func (f *SvgFile) ThumbExpired() bool {
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
	defer file.Close()
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
	if !f.ThumbExists() {
		return nil
	}
	if config.Global.Ffmpeg == "" {
		thumb, _ := storage.Internal.Open("res/audio.svg")
		return &thumb
	}
	file, err := storage.Cache.Open(f.ThumbPath)
	if err != nil {
		return nil
	}
	return &file
}

func (f *AudioFile) ThumbExists() bool {
	if config.Global.Ffmpeg != "" { // Check if we generated thumbnail already
		exists, _ := afero.Exists(storage.Cache, f.ThumbPath)
		// Ensure we refresh Thumb stat
		f.Media().ThumbInfo, _ = storage.Cache.Stat(f.ThumbPath)
		return exists
	}
	thumb, err := storage.Internal.Open("res/audio.svg")
	defer thumb.Close()
	if err != nil {
		return false
	}
	f.Media().ThumbInfo, err = thumb.Stat()
	if err != nil {
		return false
	}
	return true
}

func (f *AudioFile) ThumbExpired() bool {
	if !f.ThumbExists() {
		return true
	}
	m := f.Media()
	diff := m.ThumbInfo.ModTime().Sub(m.FileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *AudioFile) ThumbGenerate() (err error) {
	if config.Global.Ffmpeg == "" { // No ffmpeg no thumbnail
		return
	}
	audioFile := filepath.Join(config.Global.Root, f.FullPath)
	thumbSize := fmt.Sprintf("%dx%d", config.Global.ThumbWidth, config.Global.ThumbHeight)

	// Check for cover art
	// ffmpeg -i mp3.mp3 -an -vcodec copy cover.png
	// ffmpeg -i ?? -filter:v scale={thumbw}:-2 -an ??.jpg
	coverCmd := exec.Command(config.Global.Ffmpeg,
		"-hide_banner",
		"-loglevel", "quiet",
		"-i", audioFile,
		"-filter:v", fmt.Sprintf("scale=%d:-2", config.Global.ThumbWidth),
		"-an", "-f", "image2pipe", "-")
	outCover, _ := coverCmd.Output()
	if len(outCover) != 0 {
		err = storage.Cache.MkdirAll(filepath.Dir(f.ThumbPath), os.ModePerm)
		if err != nil {
			return
		}
		_, err = storage.Cache.Create(f.ThumbPath)
		if err != nil {
			return
		}
		// Save the cover art as thumbnail
		err = afero.WriteFile(storage.Cache, f.ThumbPath, outCover, os.ModePerm)
		f.Media().ThumbInfo, err = storage.Cache.Stat(f.ThumbPath)
		return
	}
	// Generate waveform

	// ffmpeg \
	// -hide_banner -loglevel panic \
	// -i "{in}" \
	// -filter_complex \
	// 		"[0:a]aformat=channel_layouts=mono, \
	// 		compand=gain=5, \
	// 		showwavespic=s=400x400:colors=#0c8cc8[fg]; \
	// 		color=s=400x400:color=#d7ebf2, \
	// 		drawgrid=width=iw/6:height=ih/6:color=#0c8cc8@0.3[bg]; \
	// 		[bg][fg]overlay=format=rgb, \
	// 		drawbox=x=(iw-w)/2:y=(ih-h)/2:w=iw:h=1:color=#d7ebf2" \
	// -vframes 1 \
	// -y "{out}"
	filter := []string{
		"[0:a]aformat=channel_layouts=mono,",
		"compand=gain=5,",
		"showwavespic=s="+thumbSize+":colors=#0c8cc8[fg];",
		"color=s="+thumbSize+":color=#d7ebf2,",
		"drawgrid=width=iw/6:height=ih/6:color=#0c8cc8@0.3[bg];",
		"[bg][fg]overlay=format=rgb,",
		"drawbox=x=(iw-w)/2:y=(ih-h)/2:w=iw:h=1:color=#d7ebf2",
	}
	// Generate the waveform to stdout
	thumbCmd := exec.Command(config.Global.Ffmpeg,
		"-hide_banner",
		"-loglevel", "quiet",
		"-i", audioFile,
		"-filter_complex", strings.Join(filter, " "),
		"-vframes", "1",
		"-f", "image2pipe", "-")
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

func (f *VideoFile) ThumbExists() bool {
	if config.Global.Ffmpeg != "" { // Check if we generated thumbnail already
		exists, _ := afero.Exists(storage.Cache, f.ThumbPath)
		// Ensure we refresh Thumb stat
		f.Media().ThumbInfo, _ = storage.Cache.Stat(f.ThumbPath)
		return exists
	}
	// Using internal images
	thumb, err := storage.Internal.Open("res/video.svg")
	defer thumb.Close()
	if err != nil {
		return false
	}
	f.Media().ThumbInfo, err = thumb.Stat()
	if err != nil {
		return false
	}
	return true
}

func (f *VideoFile) ThumbExpired() bool {
	if !f.ThumbExists() {
		return true
	}
	m := f.Media()
	diff := m.ThumbInfo.ModTime().Sub(m.FileInfo.ModTime())
	return diff < 0*time.Second
}

var reDuration = regexp.MustCompile(`Duration: (\d{2}:\d{2}:\d{2})`)

func (f *VideoFile) ThumbGenerate() (err error) {
	if config.Global.Ffmpeg == "" { // No ffmpeg no thumbnail
		return
	}

	movieFile := filepath.Join(config.Global.Root, f.FullPath)

	// Get the duration of the movie
	cmd := exec.Command(config.Global.Ffmpeg, "-hide_banner", "-i",
		movieFile) // #nosec Configuration is provided by the admin
	out, _ := cmd.CombinedOutput()

	match := reDuration.FindSubmatch(out)
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

func (f *PdfFile) ThumbExists() bool {
	thumb, err := storage.Internal.Open("res/pdf.svg")
	defer thumb.Close()
	if err != nil {
		return false
	}
	f.Media().ThumbInfo, err = thumb.Stat()
	if err != nil {
		return false
	}
	return true
}

func (f *PdfFile) ThumbExpired() bool {
	return true
}

func (f *PdfFile) ThumbGenerate() error {
	if !f.ThumbExists() {
		return errors.New("no video thumbnail")
	}
	return nil
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

// Escape a path while keeping slashes unchanged
func EscapePath(s string) (r string) {
	parts := strings.Split(s, "/")
	eparts := make([]string, 0, len(parts))
	for _, part := range parts {
		eparts = append(eparts, url.PathEscape(part))
	}
	return strings.Join(eparts, "/")
}

var mimePrefixes = regexp.MustCompile("^(image|video|audio|application/pdf)")

// Find the type of a file
//
// Careful: on unix uses specific files
//   /etc/mime.types
//   /etc/apache2/mime.types
//   /etc/apache/mime.types
func GetMediaClass(name string) (class string) {
	switch contentType := mime.TypeByExtension(filepath.Ext(name)); {
	case strings.HasPrefix(contentType, "image/"):
		return "image"
	case strings.HasPrefix(contentType, "audio/"):
		return "audio"
	case strings.HasPrefix(contentType, "video/"):
		return "video"
	case strings.HasPrefix(contentType, "application/pdf"):
		return "pdf"
	default:
		return
	}
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

func isdigit(b byte) bool { return '0' <= b && b <= '9' }

// From
// https://github.com/fvbommel/util/blob/master/sortorder/natsort.go
func NaturalLess(str1, str2 string) bool {
	idx1, idx2 := 0, 0
	for idx1 < len(str1) && idx2 < len(str2) {
		c1, c2 := str1[idx1], str2[idx2]
		dig1, dig2 := isdigit(c1), isdigit(c2)
		switch {
		case dig1 != dig2: // Digits before other characters.
			return dig1 // True if LHS is a digit, false if the RHS is one.
		case !dig1: // && !dig2, because dig1 == dig2
			// UTF-8 compares bytewise-lexicographically, no need to decode
			// codepoints.
			if c1 != c2 {
				return c1 < c2
			}
			idx1++
			idx2++
		default: // Digits
			// Eat zeros.
			for ; idx1 < len(str1) && str1[idx1] == '0'; idx1++ {
			}
			for ; idx2 < len(str2) && str2[idx2] == '0'; idx2++ {
			}
			// Eat all digits.
			nonZero1, nonZero2 := idx1, idx2
			for ; idx1 < len(str1) && isdigit(str1[idx1]); idx1++ {
			}
			for ; idx2 < len(str2) && isdigit(str2[idx2]); idx2++ {
			}
			// If lengths of numbers with non-zero prefix differ, the shorter
			// one is less.
			if len1, len2 := idx1-nonZero1, idx2-nonZero2; len1 != len2 {
				return len1 < len2
			}
			// If they're equal, string comparison is correct.
			if nr1, nr2 := str1[nonZero1:idx1], str2[nonZero2:idx2]; nr1 != nr2 {
				return nr1 < nr2
			}
			// Otherwise, the one with less zeros is less.
			// Because everything up to the number is equal, comparing the index
			// after the zeros is sufficient.
			if nonZero1 != nonZero2 {
				return nonZero1 < nonZero2
			}
		}
		// They're identical so far, so continue comparing.
	}
	// So far they are identical. At least one is ended. If the other continues,
	// it sorts last.
	return len(str1) < len(str2)
}
