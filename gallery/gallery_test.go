package gallery

import (
	"math/rand"
	"strconv"
	"testing"
)

type generator struct {
	src *rand.Rand
}

func (g *generator) NextInt(max int) int {
	return g.src.Intn(max)
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
	for i := 0; i < numlen; i++ {
		num += strconv.Itoa(g.src.Intn(10))
	}
	// Put it all together
	for i := 0; i < strlen+1; i++ {
		if i == numpos {
			str += num
		} else {
			str += string('a' + g.src.Intn(16))
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

func TestNaturalLess(t *testing.T) {
	testset := []struct {
		s1, s2 string
		less   bool
	}{
		{"0", "00", true},
		{"00", "0", false},
		{"aa", "ab", true},
		{"ab", "abc", true},
		{"abc", "ad", true},
		{"ab1", "ab2", true},
		{"ab1c", "ab1c", false},
		{"ab12", "abc", true},
		{"ab2a", "ab10", true},
		{"a0001", "a0000001", true},
		{"a10", "abcdefgh2", true},
		{"аб2аб", "аб10аб", true},
		{"2аб", "3аб", true},
		//
		{"a1b", "a01b", true},
		{"a01b", "a1b", false},
		{"ab01b", "ab010b", true},
		{"ab010b", "ab01b", false},
		{"a01b001", "a001b01", true},
		{"a001b01", "a01b001", false},
		{"a1", "a1x", true},
		{"1ax", "1b", true},
		{"1b", "1ax", false},
		//
		{"082", "83", true},
		//
		{"083a", "9a", false},
		{"9a", "083a", true},
	}
	for _, v := range testset {
		if res := NaturalLess(v.s1, v.s2); res != v.less {
			t.Errorf("Compared %#q to %#q: expected %v, got %v",
				v.s1, v.s2, v.less, res)
		}
	}
}

func BenchmarkNaturalLess(b *testing.B) {
	set := randomStringArray(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range set[0] {
			k := (j + 1) % len(set[0])
			_ = NaturalLess(set[0][j], set[0][k])
		}
	}
}

func BenchmarkStdStringLess(b *testing.B) {
	set := randomStringArray(1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := range set[0] {
			k := (j + 1) % len(set[0])
			_ = set[0][j] < set[0][k]
		}
	}
}
