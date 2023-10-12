package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"reflect"
	"testing"

	"specto.org/projects/foldergal/internal/templates"
)

// func assertResponseBody(t testing.TB, got, want string) {
// 	t.Helper()
// 	if got != want {
// 		t.Errorf("wrong response body, got %q want %q", got, want)
// 	}
// }

func assertStatus(t testing.TB, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("incorrect status, got %d, want %d", got, want)
	}
}

// func TestHttpHandler(t *testing.T) {
//     // Create a request to pass to our handler. We don't have any query parameters for now, so we'll
//     // pass 'nil' as the third parameter.
//     req, err := http.NewRequest("GET", "/", nil)
//     if err != nil {
//         t.Fatal(err)
//     }

//     // We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
//     rr := httptest.NewRecorder()
//     handler := http.HandlerFunc(HttpHandler)

//     // Our handlers satisfy http.Handler, so we can call their ServeHTTP method
//     // directly and pass in our Request and ResponseRecorder.
//     handler.ServeHTTP(rr, req)

//     // Check the status code is what we expect.
//     if status := rr.Code; status != http.StatusOK {
//         t.Errorf("handler returned wrong status code: got %v want %v",
//             status, http.StatusOK)
//     }

//     // Check the response body is what we expect.
//     expected := `{"alive": true}`
//     if rr.Body.String() != expected {
//         t.Errorf("handler returned unexpected body: got %v want %v",
//             rr.Body.String(), expected)
//     }
// }

func TestMain(m *testing.M) {
	os.Chdir("../..")
	fmt.Println("-> Preparing...")
	result := m.Run()
	fmt.Println("-> Finishing...")
	os.Exit(result)
}

func Test_previewHandler(t *testing.T) {
	// TODO: test existing audio
	// TODO: test existing video without preview
	t.Run("returns preview media", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/thisshouldnotexist-", http.NoBody)
		response := httptest.NewRecorder()
		previewHandler(response, request)
		assertStatus(t, response.Code, http.StatusNotFound)
	})
}

func Test_fail404(t *testing.T) {
	t.Run("returns 404", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "", http.NoBody)
		response := httptest.NewRecorder()
		fail404(response, request)
		assertStatus(t, response.Code, http.StatusNotFound)
	})
}

func Test_fail500(t *testing.T) {
	realInit()
	t.Run("returns 500", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", http.NoBody)
		response := httptest.NewRecorder()
		e := errors.New("this sucks")
		fail500(response, e, request)
		assertStatus(t, response.Code, http.StatusInternalServerError)
	})
}

func Test_sanitizePath(t *testing.T) {

	tests := []struct {
		path string
		want string
	}{
		{`/windows/system32`, `system32`},
		{`../apath/like:this`, "like:this"},
		{"internal/storage/res", "internal/storage/res"},
		{"", "."},
		{"./../../../z", "z"},
		{"./../../../", "."},
		{"a/../../../b", "b"},
	}

	for _, tc := range tests {
		if result := sanitizePath(tc.path); result != tc.want {
			t.Fatalf("sanitizePath(%v) = %v, want %v", tc.path, result, tc.want)
		}
	}
}

func Test_splitUrlToBreadCrumbs(t *testing.T) {
	defaultTitle := "#:\\"
	toUrl := func(u string) *url.URL {
		uu, _ := url.Parse(u)
		return uu
	}
	tests := []struct {
		input *url.URL
		want  []templates.BreadCrumb
	}{
		{toUrl("http://example.com/what/is/this"),
			[]templates.BreadCrumb{
				{Url: "/", Title: defaultTitle},
				{Url: "/what", Title: "what"},
				{Url: "/what/is", Title: "is"},
				{Url: "/what/is/this", Title: "this"},
			}},
		{toUrl(""),
			[]templates.BreadCrumb{
				{Url: "/", Title: defaultTitle},
			}},
		{toUrl("some text ."),
			[]templates.BreadCrumb{
				{Url: "/", Title: defaultTitle},
				{Url: "/some text .", Title: "some text ."},
			}},
	}
	for _, tc := range tests {
		result := splitUrlToBreadCrumbs(tc.input)
		if !reflect.DeepEqual(result, tc.want) {
			t.Fatalf("splitUrlToBreadCrumbs(%v)\n%v\n===\n%v",
				tc.input, result, tc.want)
		}
	}
}

func Test_fileExists(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"internal/storage/res", false},
		{"storage/", false},
		{"", false},
		{"internal/storage/res/folder.svg", true},
	}

	for _, tc := range tests {
		if result := fileExists(tc.path); result != tc.want {
			t.Fatalf("fileExists(%v) = %v, want %v", tc.path, result, tc.want)
		}
	}
}

type Values url.Values

type parseTest struct {
	query string
	out   Values
	ok    bool
}

var parseTests = []parseTest{
	{
		query: "a/1",
		out:   Values{"a": []string{"1"}},
		ok:    true,
	},
	{
		query: "a/1/b/2",
		out:   Values{"a": []string{"1"}, "b": []string{"2"}},
		ok:    true,
	},
	{
		query: "a/1/a/2/a/banana",
		out:   Values{"a": []string{"1", "2", "banana"}},
		ok:    true,
	},
	{
		query: "ascii/%3Ckey%3A+0x90%3E",
		out:   Values{"ascii": []string{"<key: 0x90>"}},
		ok:    true,
	}, {
		query: "a/1;b/2",
		out:   Values{},
		ok:    false,
	}, {
		query: "a;b/1",
		out:   Values{},
		ok:    false,
	}, {
		query: "a/%3B", // hex encoding for semicolon
		out:   Values{"a": []string{";"}},
		ok:    true,
	},
	{
		query: "a%3Bb/1",
		out:   Values{"a;b": []string{"1"}},
		ok:    true,
	},
	{
		query: "a/1/a/2;a/banana",
		out:   Values{"a": []string{"1"}},
		ok:    false,
	},
	{
		query: "a;b/c/1",
		out:   Values{"c": []string{"1"}},
		ok:    false,
	},
	{
		query: "a/1/b/2;a/3/c/4",
		out:   Values{"a": []string{"1"}, "c": []string{"4"}},
		ok:    false,
	},
	{
		query: "a/1/b/2;c/3",
		out:   Values{"a": []string{"1"}},
		ok:    false,
	},
	{
		query: ";",
		out:   Values{},
		ok:    false,
	},
	{
		query: "a/1;",
		out:   Values{},
		ok:    false,
	},
	{
		query: "a/1/;",
		out:   Values{"a": []string{"1"}},
		ok:    false,
	},
	{
		query: ";a/b/2",
		out:   Values{"b": []string{"2"}},
		ok:    false,
	},
	{
		query: "a/1/b/2;",
		out:   Values{"a": []string{"1"}},
		ok:    false,
	},
	{
		query: "a/%A",
		out:   Values{},
		ok:    false,
	},
	{
		query: "%A/1",
		out:   Values{},
		ok:    false,
	},
}

// Copied from url tests
func Test_parseQuery(t *testing.T) {
	for _, test := range parseTests {
		t.Run(test.query, func(t *testing.T) {
			form, err := parseQuery(test.query)
			if test.ok != (err == nil) {
				want := "<error>"
				if test.ok {
					want = "<nil>"
				}
				t.Errorf("Unexpected error: %v, want %v", err, want)
			}
			if len(form) != len(test.out) {
				t.Errorf("len(form) = %d, want %d", len(form), len(test.out))
			}
			for k, evs := range test.out {
				vs, ok := form[k]
				if !ok {
					t.Errorf("Missing key %q", k)
					continue
				}
				if len(vs) != len(evs) {
					t.Errorf("len(form[%q]) = %d, want %d", k, len(vs), len(evs))
					continue
				}
				for j, ev := range evs {
					if v := vs[j]; v != ev {
						t.Errorf("form[%q][%d] = %q, want %q", k, j, v, ev)
					}
				}
			}
		})
	}
}

// TODO Find a way to test fatal errors in main?
