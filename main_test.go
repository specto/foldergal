package main

import (
	"errors"
	"net/http"
    "net/http/httptest"
    "os"
	"testing"
)

func assertResponseBody(t testing.TB, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("response body is wrong, got %q want %q", got, want)
	}
}


func assertStatus(t testing.TB, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("did not get correct status, got %d, want %d", got, want)
	}
}

// // func TestHttpHandler(t *testing.T) {
// //     // Create a request to pass to our handler. We don't have any query parameters for now, so we'll
// //     // pass 'nil' as the third parameter.
// //     req, err := http.NewRequest("GET", "/", nil)
// //     if err != nil {
// //         t.Fatal(err)
// //     }

// //     // We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
// //     rr := httptest.NewRecorder()
// //     handler := http.HandlerFunc(HttpHandler)

// //     // Our handlers satisfy http.Handler, so we can call their ServeHTTP method
// //     // directly and pass in our Request and ResponseRecorder.
// //     handler.ServeHTTP(rr, req)

// //     // Check the status code is what we expect.
// //     if status := rr.Code; status != http.StatusOK {
// //         t.Errorf("handler returned wrong status code: got %v want %v",
// //             status, http.StatusOK)
// //     }

// //     // Check the response body is what we expect.
// //     expected := `{"alive": true}`
// //     if rr.Body.String() != expected {
// //         t.Errorf("handler returned unexpected body: got %v want %v",
// //             rr.Body.String(), expected)
// //     }
// // }

func TestMain(m *testing.M) {
	// Before main
	exitCode := m.Run()
	// Cleanup after main
	os.Exit(exitCode)
}

func TestPreviewHandler(t *testing.T) {
	// TODO: test existing audio
	// TODO: test existing video without preview
	t.Run("returns preview media", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/thisshouldnotexist-", nil)
		response := httptest.NewRecorder()
		previewHandler(response, request)
		assertStatus(t, response.Code, http.StatusNotFound)
	})
}

func TestFail404(t *testing.T) {
	t.Run("returns 404", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "", nil)
		response := httptest.NewRecorder()
		fail404(response, request)
		assertStatus(t, response.Code, http.StatusNotFound)
	})
}

func TestFail500(t *testing.T) {
	realInit()
	t.Run("returns 500", func(t *testing.T) {
		request, _ := http.NewRequest(http.MethodGet, "/", nil)
		response := httptest.NewRecorder()
		e := errors.New("this sucks")
		fail500(response, e, request)
		assertStatus(t, response.Code, http.StatusInternalServerError)
	})
}

func TestSanitizePath(t *testing.T) {

	tests := []struct {
		path   string
		want   string
	}{
		{`/windows/system32`, `system32`},
		{`../apath/like:this`, "like:this"},
		{"storage/res", "storage/res"},
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
