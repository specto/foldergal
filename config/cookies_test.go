package config

import (
	"encoding/base64"
	"testing"
)

func TestCookieSettings(t *testing.T) {
	a := CookieSettings{Sort: "bla", Order: false, Show: "foo"}
	aStr, err := a.Marshal()
	if err != nil {
		t.Error(err)
	}
	b := DefaultCookieSettings()
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
}
