package config

import (
	"encoding/base64"
	"encoding/json"
)

type CookieSettings struct {
	Sort  string `json:"sort"`
	Order bool   `json:"order"`
	Show  string `json:"show"`
}

func (cs *CookieSettings) Encode() (string, error) {
	val, err := json.Marshal(cs)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(val), nil
}

func (cs *CookieSettings) FromString(from string) (err error) {
	s, err := base64.StdEncoding.DecodeString(from)
	if err != nil {
		return
	}
	err = json.Unmarshal(s, &cs)
	return
}

func DefaultCookieSettings() CookieSettings {
	return CookieSettings{Sort: "date", Order: true, Show: "files"}
}
