package gallery

import (
	"bytes"
	"errors"
	"fmt"
	"foldergal/config"
	"foldergal/storage"
	"image"
	"image/color"
	"image/draw"
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

	"github.com/disintegration/imaging"
	"github.com/spf13/afero"
)

var (
	ErrNotValid         = errors.New("invalid media")
	ErrFileNotFound     = errors.New("file for media not found")
	ErrThumbNotFound    = errors.New("thumbnail for media not found")
	ErrThumbNotPossible = errors.New("thumbnail cannot be generated (missing ffmpeg)")
)

type Media interface {
	thumbGenerate() error
	thumbExists() bool
	thumbExpired() bool

	Thumb() (afero.File, error)
	ThumbModTime() time.Time
	ThumbPath() string
	ThumbName() string

	File() (afero.File, error)
	FileModTime() time.Time
}

// Generates the media thumbnail
func GenerateThumb(m Media) error {
	if m.thumbExpired() {
		return m.thumbGenerate()
	}
	return nil
}

// //////////////////////////////////////////////////////////////////////////////
type mediaFile struct {
	fullPath  string
	fileInfo  os.FileInfo
	thumbPath string
	thumbInfo os.FileInfo
}

func NewMedia(fullPath string) (Media, error) {
	fileInfo, err := storage.Root.Stat(fullPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if fileInfo.IsDir() || !IsValidMedia(fullPath) {
		return nil, ErrNotValid
	}
	return &mediaFile{fullPath: fullPath, fileInfo: fileInfo}, nil
}

func (f *mediaFile) Thumb() (afero.File, error) {
	return storage.Cache.Open(f.thumbPath)
}

func (f *mediaFile) thumbExists() bool {
	var err error
	// Ensure we refresh Thumb stat
	f.thumbInfo, err = storage.Cache.Stat(f.thumbPath)
	return err == nil
}

func (f *mediaFile) thumbExpired() bool {
	if !f.thumbExists() {
		return true
	}
	diff := f.thumbInfo.ModTime().Sub(f.fileInfo.ModTime())
	return diff < 0*time.Second
}

func (f *mediaFile) thumbGenerate() (err error) {
	return errors.New("not implemented")
}

func (f *mediaFile) ThumbName() string {
	if f.thumbInfo == nil {
		return ""
	}
	return f.thumbInfo.Name()
}

func (f *mediaFile) ThumbPath() string {
	return f.thumbPath
}

func (f *mediaFile) ThumbModTime() time.Time {
	if f.thumbInfo == nil {
		return time.Time{}
	}
	return f.thumbInfo.ModTime()
}

func (f *mediaFile) File() (afero.File, error) {
	return storage.Root.Open(f.fullPath)
}

func (f *mediaFile) FileModTime() time.Time {
	return f.fileInfo.ModTime()
}

// //////////////////////////////////////////////////////////////////////////////
type imageFile struct {
	mediaFile
}

func NewImage(fullPath, thumbPath string) (Media, error) {
	fileInfo, err := storage.Root.Stat(fullPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if fileInfo.IsDir() || !IsValidMedia(fullPath) {
		return nil, ErrNotValid
	}
	return &imageFile{mediaFile{
		fullPath: fullPath, fileInfo: fileInfo, thumbPath: thumbPath}}, nil
}

func (f *imageFile) thumbGenerate() (err error) {
	var (
		file afero.File
		img  image.Image
	)
	file, err = storage.Root.Open(f.mediaFile.fullPath)
	if err != nil {
		return
	}
	defer file.Close()
	img, err = imaging.Decode(file, imaging.AutoOrientation(true))
	if err != nil {
		return
	}
	resized := imaging.Fit(img, config.Global.ThumbWidth,
		config.Global.ThumbHeight, imaging.CatmullRom)

	// Merge onto white background
	backgroundColor := color.RGBA{0xff, 0xff, 0xff, 0xff} // white
	dst := image.NewRGBA(resized.Bounds())
	draw.Draw(dst, dst.Bounds(), image.NewUniform(backgroundColor),
		image.Point{}, draw.Src)
	draw.Draw(dst, dst.Bounds(), resized, resized.Bounds().Min, draw.Over)

	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, dst, nil)
	if err != nil {
		return
	}
	err = storage.Cache.MkdirAll(filepath.Dir(f.mediaFile.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = storage.Cache.Create(f.mediaFile.thumbPath)
	if err != nil {
		return
	}
	_ = afero.WriteFile(storage.Cache, f.mediaFile.thumbPath, buf.Bytes(), os.ModePerm)
	f.thumbInfo, err = storage.Cache.Stat(f.mediaFile.thumbPath)
	return
}

// //////////////////////////////////////////////////////////////////////////////
type svgFile struct {
	mediaFile
}

func NewSvg(fullPath string) (Media, error) { // SVGs are their own thumbnails
	fileInfo, err := storage.Root.Stat(fullPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if fileInfo.IsDir() || !IsValidMedia(fullPath) {
		return nil, ErrNotValid
	}
	return &svgFile{mediaFile{
		fullPath: fullPath, fileInfo: fileInfo, thumbPath: fullPath}}, nil
}

func (f *svgFile) thumbGenerate() (err error) {
	var (
		file     afero.File
		contents []byte
	)
	file, err = storage.Root.Open(f.mediaFile.fullPath)
	if err != nil {
		return
	}
	defer file.Close()
	contents, err = afero.ReadAll(file)
	if err != nil {
		return
	}
	err = storage.Cache.MkdirAll(filepath.Dir(f.mediaFile.thumbPath), os.ModePerm)
	if err != nil {
		return
	}
	_, err = storage.Cache.Create(f.mediaFile.thumbPath)
	if err != nil {
		return
	}
	err = afero.WriteFile(storage.Cache, f.mediaFile.thumbPath, contents, os.ModePerm)
	if err != nil {
		return
	}
	// Since we just copy SVGs the Thumb File is the same as the original
	f.thumbPath = f.mediaFile.fullPath
	f.thumbInfo, err = storage.Cache.Stat(f.mediaFile.thumbPath)
	return
}

// //////////////////////////////////////////////////////////////////////////////
type audioFile struct {
	mediaFile
}

func NewAudio(fullPath, thumbPath string) (Media, error) {
	fileInfo, err := storage.Root.Stat(fullPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if fileInfo.IsDir() || !IsValidMedia(fullPath) {
		return nil, ErrNotValid
	}
	return &audioFile{mediaFile{
		fullPath: fullPath, fileInfo: fileInfo, thumbPath: thumbPath}}, nil
}

func (f *audioFile) Thumb() (afero.File, error) {
	if !f.thumbExists() {
		return nil, ErrThumbNotFound
	}
	if config.Global.Ffmpeg == "" {
		return nil, ErrThumbNotPossible
	}
	return storage.Cache.Open(f.mediaFile.thumbPath)
}

func (f *audioFile) thumbExists() bool {
	if config.Global.Ffmpeg == "" {
		return true
	}
	var err error
	// Ensure we refresh Thumb stat
	f.mediaFile.thumbInfo, err = storage.Cache.Stat(f.mediaFile.thumbPath)
	return err == nil
}

func (f *audioFile) thumbGenerate() error {
	if config.Global.Ffmpeg == "" { // No ffmpeg no thumbnail
		return nil
	}
	audioFile := filepath.Join(config.Global.Root, f.mediaFile.fullPath)
	var thumbData []byte

	// Check for cover art
	coverCmd := exec.Command(config.Global.Ffmpeg,
		"-hide_banner", "-loglevel", "quiet",
		"-i", audioFile,
		"-filter:v", fmt.Sprintf("scale=%d:-2", config.Global.ThumbWidth),
		"-an", "-f", "image2pipe", "-") // #nosec Executable path is provided by config
	if outCover, _ := coverCmd.Output(); len(outCover) != 0 {
		thumbData = outCover
	} else {
		// Generate waveform
		thumbSize := fmt.Sprintf("%dx%d", config.Global.ThumbWidth, config.Global.ThumbHeight)
		filter := []string{
			"color=c=black[color];",
			"aformat=channel_layouts=mono,",
			"showwavespic=s=" + thumbSize + ":colors=white[wave];",
			"[color][wave]scale2ref[bg][fg];",
			"[bg][fg]overlay=format=auto",
		}
		// ffmpeg to stdout
		thumbCmd := exec.Command(config.Global.Ffmpeg,
			"-hide_banner", "-loglevel", "quiet",
			"-i", audioFile,
			"-filter_complex", strings.Join(filter, " "),
			"-frames:v", "1",
			"-f", "image2pipe", "-") // #nosec Executable path is provided by config
		outThumb, _ := thumbCmd.Output()
		if len(outThumb) == 0 { // Failed thumbnail
			return errors.New("failed to generate thumbnail: " + f.mediaFile.thumbPath)
		}
		thumbData = outThumb
	}
	// Create all folders needed
	err := storage.Cache.MkdirAll(filepath.Dir(f.mediaFile.thumbPath), os.ModePerm)
	if err != nil {
		return err
	}
	_, err = storage.Cache.Create(f.mediaFile.thumbPath)
	if err != nil {
		return err
	}
	// Save thumbnail
	err = afero.WriteFile(storage.Cache, f.mediaFile.thumbPath, thumbData, os.ModePerm)
	if err != nil {
		return err
	}
	f.mediaFile.thumbInfo, err = storage.Cache.Stat(f.mediaFile.thumbPath)
	if err != nil {
		return err
	}
	return nil
}

// //////////////////////////////////////////////////////////////////////////////
type videoFile struct {
	mediaFile
}

func NewVideo(fullPath, thumbPath string) (Media, error) {
	fileInfo, err := storage.Root.Stat(fullPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if fileInfo.IsDir() || !IsValidMedia(fullPath) {
		return nil, ErrNotValid
	}
	return &videoFile{mediaFile{
		fullPath: fullPath, fileInfo: fileInfo, thumbPath: thumbPath}}, nil
}

func (f *videoFile) Thumb() (afero.File, error) {
	if !f.thumbExists() {
		return nil, ErrThumbNotFound
	}
	if config.Global.Ffmpeg == "" {
		return nil, ErrThumbNotPossible
	}
	return storage.Cache.Open(f.mediaFile.thumbPath)
}

func (f *videoFile) thumbExists() bool {
	if config.Global.Ffmpeg == "" {
		return true
	}
	var err error
	// Ensure we refresh Thumb stat
	f.mediaFile.thumbInfo, err = storage.Cache.Stat(f.mediaFile.thumbPath)
	return err == nil
}

var reDuration = regexp.MustCompile(`Duration: (\d{2}:\d{2}:\d{2})`)

func (f *videoFile) thumbGenerate() error {
	if config.Global.Ffmpeg == "" { // No ffmpeg no thumbnail
		return nil
	}
	movieFile := filepath.Join(config.Global.Root, f.mediaFile.fullPath)

	// Get the duration of the movie
	cmd := exec.Command(config.Global.Ffmpeg,
		"-hide_banner",
		"-i", movieFile) // #nosec Executable path is provided by config
	out, _ := cmd.CombinedOutput()

	match := reDuration.FindSubmatch(out)
	if len(match) < 2 {
		return errors.New("cannot find video duration: " + f.mediaFile.fullPath)
	}
	// Target the first third of the movie
	targetTime := toTimeCode(fromTimeCode(string(match[1])) / 3)
	thumbSize := fmt.Sprintf("%dx%d", config.Global.ThumbWidth, config.Global.ThumbHeight)

	// Generate the thumbnail to stdout
	thumbCmd := exec.Command(config.Global.Ffmpeg,
		"-hide_banner",
		"-loglevel", "quiet",
		"-noaccurate_seek",
		"-ss", targetTime, "-i", movieFile,
		"-vf", "scale="+thumbSize+":flags=lanczos:force_original_aspect_ratio=decrease",
		"-vframes", "1",
		"-f", "image2pipe", "-") // #nosec Executable path is provided by config
	outThumb, _ := thumbCmd.Output()
	if len(outThumb) == 0 { // Failed thumbnail
		return errors.New("failed to generate thumbnail: " + f.mediaFile.thumbPath)
	}
	err := storage.Cache.MkdirAll(filepath.Dir(f.mediaFile.thumbPath), os.ModePerm)
	if err != nil {
		return err
	}
	_, err = storage.Cache.Create(f.mediaFile.thumbPath)
	if err != nil {
		return err
	}
	// Save thumbnail
	err = afero.WriteFile(storage.Cache, f.mediaFile.thumbPath, outThumb, os.ModePerm)
	if err != nil {
		return err
	}
	f.mediaFile.thumbInfo, err = storage.Cache.Stat(f.mediaFile.thumbPath)
	if err != nil {
		return err
	}
	return nil
}

// //////////////////////////////////////////////////////////////////////////////
type pdfFile struct {
	mediaFile
}

func NewPdf(fullPath, thumbPath string) (Media, error) {
	fileInfo, err := storage.Root.Stat(fullPath)
	if err != nil {
		return nil, ErrFileNotFound
	}
	if fileInfo.IsDir() || !IsValidMedia(fullPath) {
		return nil, ErrNotValid
	}
	return &pdfFile{mediaFile{
		fullPath: fullPath, fileInfo: fileInfo, thumbPath: thumbPath}}, nil
}

func (f *pdfFile) Thumb() (afero.File, error) {
	return storage.Internal.Open("res/pdf.svg")
}

func (f *pdfFile) thumbExists() bool {
	return true
}

func (f *pdfFile) thumbGenerate() error {
	return nil
}

////////////////////////////////////////////////////////////////////////////////

// Checks if any /path/./starts/with/.dot/somewhere
func ContainsDotFile(name string) bool {
	parts := strings.Split(name, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

// Escapes a path with URL escape codes while keeping slashes unchanged
// TODO: look for built-in functions in stdlib for escaping a path
func EscapePath(s string) (r string) {
	parts := strings.Split(s, "/")
	eparts := make([]string, 0, len(parts))
	for _, part := range parts {
		eparts = append(eparts, url.PathEscape(part))
	}
	return strings.Join(eparts, "/")
}

// Finds the type of a file
func GetMediaClass(name string) string {
	// NOTE: on unix this uses specific files
	//   /etc/mime.types
	//   /etc/apache2/mime.types
	//   /etc/apache/mime.types
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
		return ""
	}
}

var mimePrefixes = regexp.MustCompile("^(image|video|audio|application/pdf)")

// Check for valid media by content-type
func IsValidMedia(name string) bool {
	ext := filepath.Ext(name)
	contentType := mime.TypeByExtension(ext)
	match := mimePrefixes.FindStringSubmatch(contentType)
	return match != nil
}

// Converts duration to timecode string 00:00:00
// negative time becomes positive
func toTimeCode(d time.Duration) string {
	h := int64(math.Abs(d.Hours()))
	m := int64(math.Mod(math.Abs(d.Minutes()), 60))
	s := int64(math.Mod(math.Abs(d.Seconds()), 60))
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// 00:00:00 -> duration
func fromTimeCode(timecode string) (d time.Duration) {
	m1, m2, m3 := 0, 0, 0
	tc := strings.Split(timecode, ":")
	for i := len(tc); i <= 3; i++ {
		tc = append(tc, "")
	}
	m1, _ = strconv.Atoi(tc[0])
	m2, _ = strconv.Atoi(tc[1])
	m3, _ = strconv.Atoi(tc[2])
	d = time.Duration(math.Abs(float64(m1)))*time.Hour +
		time.Duration(math.Abs(float64(m2)))*time.Minute +
		time.Duration(math.Abs(float64(m3)))*time.Second
	return
}

