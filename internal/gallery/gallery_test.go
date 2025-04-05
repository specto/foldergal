package gallery

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"
)

type generator struct {
	src *rand.Rand
}

func (g *generator) NextInt(maximum int) int {
	return g.src.Intn(maximum)
}

// Gets random random-length alphanumeric string.
func (g *generator) NextString() (str string) {
	// Random-length 3-8 chars part
	strlen := g.src.Intn(6) + 3
	// Random-length 1-3 num
	numlen := g.src.Intn(3) + 1
	// Random position for num in string
	numpos := g.src.Intn(strlen + 1)
	// Generate the number
	var num string
	for range numlen {
		num += strconv.Itoa(g.src.Intn(10))
	}
	// Put it all together
	for i := range strlen + 1 {
		if i == numpos {
			str += num
		} else {
			str += fmt.Sprint('a' + g.src.Intn(16))
		}
	}
	return str
}

// Get 1000 arrays of 10000-string-arrays (less if -short is specified).
func randomStringArray(seed int) [][]string {
	gen := &generator{
		src: rand.New(rand.NewSource(
			int64(seed),
		)),
	}
	n := 1000
	if testing.Short() {
		n = 1
	}
	set := make([][]string, n)
	for i := range set {
		strings := make([]string, 10000)
		for idx := range strings {
			// Generate a random string
			strings[idx] = gen.NextString()
		}
		set[i] = strings
	}
	return set
}

func TestContainsDotFile(t *testing.T) {
	for _, v := range []struct {
		path string
		dot  bool
	}{
		{".", true},
		{"inner.dot", false},
		{".starting", true},
		{"/folder/./subfolder", true},
		{"/root/.hidden/subfolder", true},
		{"home/not.hidden/subfolder", false},
	} {
		if res := ContainsDotFile(v.path); res != v.dot {
			t.Errorf("Tested %#q: expected %v, got %v", v.path, v.dot, res)
		}
	}
}

func TestIsValidMedia(t *testing.T) {
	for _, v := range []struct {
		filename string
		isvalid  bool
	}{
		{"bla.jpg", true},
		{"bla.JPEG", true},
		{"bla.png", true},
		{"bla.mp4", true},
		{"bla.mp3", true},
		{"bla.pdf", true},
		{"bla.doc", false},
		{"bla.something", false},
	} {
		if res := IsValidMedia(v.filename); res != v.isvalid {
			t.Errorf("%#q: expected %v, got %v", v.filename, v.isvalid, res)
		}
	}
}

func BenchmarkStdStringLess(b *testing.B) {
	set := randomStringArray(1)
	b.ResetTimer()
	for b.Loop() {
		for j := range set[0] {
			k := (j + 1) % len(set[0])
			_ = set[0][j] < set[0][k]
		}
	}
}

func BenchmarkEscapePath(b *testing.B) {
	set := randomStringArray(1)
	b.ResetTimer()
	for b.Loop() {
		for j := range set[0] {
			_ = EscapePath(set[0][j])
		}
	}
}

func TestGetMediaClass(t *testing.T) {
	for _, v := range []struct {
		filepath  string
		mediatype string
	}{
		{"/some/path/file.unknown", ""},
		{"/some/path/file.jpg", "image"},
		{"/some/path/file.jpeg", "image"},
		{"/some/path/file.mp3", "audio"},
		{"file.mp4", "video"},
		{"/some/path/file.mp4", "video"},
		{"file.pdf", "pdf"},
		{"/some/path/file.pdf", "pdf"},
		{"doc.docx", ""},
	} {
		if res := GetMediaClass(v.filepath); res != MediaClass(v.mediatype) {
			t.Errorf("Tested %#q: expected %v, got %v", v.filepath, v.mediatype, res)
		}
	}
}

type EscapeTest struct {
	in  string
	out string
	err error
}

var pathEscapeTests = []EscapeTest{
	{"", "", nil},
	{" ", "%20", nil},
	{"abй日", "ab%D0%B9%E6%97%A5", nil},
	{"/", "/", nil},
	{"//", "//", nil},
	{"a/", "a/", nil},
	{"abc+def", "abc+def", nil},
	{"a/b", "a/b", nil},
	{"/a/b", "/a/b", nil},
	{"/a//b/", "/a//b/", nil},
	{"one two", "one%20two", nil},
	{"10%", "10%25", nil},
	{
		" ?&=#+%!<>#\"{}|\\^[]`☺\t:/@$'()*,;",
		"%20%3F&=%23+%25%21%3C%3E%23%22%7B%7D%7C%5C%5E%5B%5D%60%E2%98%BA%09:/@$%27%28%29%2A%2C%3B",
		nil,
	},
}

func TestEscapePath(t *testing.T) {
	for _, tt := range pathEscapeTests {
		actual := EscapePath(tt.in)
		if tt.out != actual {
			t.Errorf("PathEscape(%q) = %q, want %q", tt.in, actual, tt.out)
		}
	}
}

func TestFromTimeCode(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"00:20:00", 20 * time.Minute},
		{"00:-20:00", 20 * time.Minute},
		{"", 0},
		{"totally:invalid:this:is", 0},
		{"-1:invalid:10", 1*time.Hour + 10*time.Second},
	}

	for _, tc := range tests {
		if result := fromTimeCode(tc.input); result != tc.want {
			t.Fatalf("fromTimeCode(%v) = %v, want %v", tc.input, result, tc.want)
		}
	}
}

func TestToTimeCode(t *testing.T) {
	tests := []struct {
		input time.Duration
		want  string
	}{
		{0, `00:00:00`},
		{1 * time.Hour, `01:00:00`},
		{200 * time.Minute, `03:20:00`},
		{23*time.Hour + 59*time.Minute + 59*time.Second, `23:59:59`},
		{1200 * time.Hour, `1200:00:00`},
		{(100 * time.Hour) + 10*time.Second, `100:00:10`},
		{-30 * time.Second, "00:00:30"},
		{-math.MaxInt64, "2562047:47:16"},
	}

	for _, tc := range tests {
		if result := toTimeCode(tc.input); result != tc.want {
			t.Fatalf("toTimeCode(%v) = %v, want %v", tc.input, result, tc.want)
		}
	}
}
