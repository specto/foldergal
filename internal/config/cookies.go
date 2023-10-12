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

// Serializes to base64 encoded json
func (cs *CookieSettings) Marshal() (string, error) {
	val, _ := json.Marshal(cs)
	return base64.StdEncoding.EncodeToString(val), nil
}

// Deserializes from base64 encoded json string
func (cs *CookieSettings) Unmarshal(from string) (err error) {
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
