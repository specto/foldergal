package config

import (
	"encoding/base64"
	"net/url"
	"testing"
)

func TestRequestSettings(t *testing.T) {
	a := RequestSettings{Sort: "bla", Order: QueryOrderDesc, Display: "foo"}
	aStr, err := a.Marshal()
	if err != nil {
		t.Error(err)
	}
	b := NewRequestSettings()
	b.Unmarshal(aStr)
	bStr, err := b.Marshal()
	if err != nil {
		t.Error(err)
	}
	if bStr != aStr {
		t.Error("Expected a and b to be equal, they are not")
	}
	if err = a.Unmarshal("invalid base64"); err == nil {
		t.Error("Allowing invalid base64")
	}
	if err = a.Unmarshal(base64.StdEncoding.EncodeToString([]byte("Invalid json []"))); err == nil {
		t.Error("Allowing invalid json")
	}

	if queryParam(-1).String() != "" {
		t.Error("WTF queryParam?!")
	}
}

func TestQueryString(t *testing.T) {
	tests := []struct {
		input url.Values
		want  string
	}{
		{url.Values{
			QKeySort.String():    []string{string(QuerySortName)},
			QKeyDisplay.String(): []string{string(QueryDisplayFile)},
		}, "?y/f/s/n"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderAsc)},
		}, "?o/a"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderDesc)},
			QKeySort.String():  []string{string(QuerySortDate)},
		}, ""},
		{url.Values{
			QKeySort.String(): []string{string(QuerySortName)},
		}, "?s/n"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderAsc)},
			QKeySort.String():  []string{string(QuerySortName)},
		}, "?s/n"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderDesc)},
			QKeySort.String():  []string{string(QuerySortName)},
		}, "?o/z/s/n"},
		{url.Values{
			QKeyOrder.String(): []string{"invalid"},
			"nonexisting":      []string{"invalid"},
		}, ""},
	}

	for _, tc := range tests {
		query := RequestSettingsFromQuery(tc.input)
		if result := query.QueryString(); result != tc.want {
			t.Fatalf("got %v, want %v", result, tc.want)
		}
	}
}

func TestQueryFull(t *testing.T) {
	tests := []struct {
		input url.Values
		want  string
	}{
		{url.Values{
			QKeySort.String():    []string{string(QuerySortName)},
			QKeyDisplay.String(): []string{string(QueryDisplayFile)},
		}, "?y/f/o/a/s/n"},
		{url.Values{
			QKeyDisplay.String(): []string{string(QueryDisplayShow)},
		}, "?y/w/o/z/s/d"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderAsc)},
		}, "?y/w/o/a/s/d"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderDesc)},
		}, "?y/w/o/z/s/d"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderAsc)},
			QKeySort.String():  []string{string(QuerySortName)},
		}, "?y/w/o/a/s/n"},
		{url.Values{
			QKeyOrder.String(): []string{string(QueryOrderDesc)},
			QKeySort.String():  []string{string(QuerySortName)},
		}, "?y/w/o/z/s/n"},
	}

	for _, tc := range tests {
		query := RequestSettingsFromQuery(tc.input)
		if result := query.QueryFull(); result != tc.want {
			t.Fatalf("got %v, want %v", result, tc.want)
		}
	}
}
