package config

import (
	"encoding/base64"
	"net/url"
	"testing"
)

func TestRequestSettings(t *testing.T) {
	a := RequestSettings{Sort: "bla", Order: false, Display: "foo"}
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

func TestQueryRequestSettings(t *testing.T) {
	tests := []struct {
		input url.Values
		want  string
	}{
		{url.Values{
			"s":    []string{"n"},
			"display": []string{"direct"},
		}, "?display/direct/o/a/s/n"},
		{url.Values{
			"o": []string{"a"},
		}, "?o/a/s/t"},
		{url.Values{
			"o": []string{"d"},
		}, "?o/d/s/t"},
		{url.Values{
			"s": []string{"t"},
		}, "?o/d/s/t"},
		{url.Values{
			"s": []string{"n"},
		}, "?o/a/s/n"},
	}

	for _, tc := range tests {
		query := RequestSettingsFromQuery(tc.input)
		if result := query.QueryString(); result != tc.want {
			t.Fatalf("got %v, want %v", result, tc.want)
		}
	}
}
