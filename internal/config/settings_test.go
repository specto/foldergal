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

	fromValues := RequestSettingsFromQuery(url.Values{
		"sort":    []string{"name"},
		"display": []string{"direct"},
	})
	if expected, result := "?display/direct/order/asc/sort/name", fromValues.QueryString(); result != expected {
		t.Error("Bad query string:", result, "expected:", expected)
	}

	fromValues = RequestSettingsFromQuery(url.Values{
		"order": []string{"asc"},
	})
	if expected, result := "?order/asc", fromValues.QueryString(); result != expected {
		t.Error("Bad query string:", result, "expected:", expected)
	}

	fromValues = RequestSettingsFromQuery(url.Values{
		"order": []string{"desc"},
	})
	if expected, result := "", fromValues.QueryString(); result != expected {
		t.Error("Expected empty query string:", result, "expected:", expected)
	}

	if queryParam(-1).String() != "" {
		t.Error("WTF queryParam?!")
	}
}
