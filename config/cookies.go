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

func (cs *CookieSettings) Encode() string {
	val, err := json.Marshal(cs)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(val)
}

func (cs *CookieSettings) FromString(from string) (err error) {
	s, err := base64.StdEncoding.DecodeString(from)
	if err != nil {
		return err
	}
	err = json.Unmarshal(s, &cs)
	return
}

func NewCookieSettings() CookieSettings {
	return CookieSettings{Sort: "date", Order: true, Show: "files"}
}
