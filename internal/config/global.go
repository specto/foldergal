package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

var (
	Global    Configuration
	EnvPrefix = "FOLDERGAL_"
)

type Configuration struct {
	Host              string
	Port              int
	Home              string
	Root              string
	Cache             string `json:"-"`
	Prefix            string
	TlsCrt            string
	TlsKey            string
	Http2             bool
	CacheExpiresAfter JsonDuration
	NotifyAfter       JsonDuration
	DiscordName       string
	DiscordWebhook    string
	PublicHost        string
	Quiet             bool
	Ffmpeg            string
	ConfigFile        string `json:"-"`
	PublicUrl         string `json:"-"`
	ThumbWidth        int
	ThumbHeight       int
	TimeZone          string
	TimeLocation      *time.Location `json:"-"`
	Log               *log.Logger    `json:"-"`
}

// Loads configuration from json file
func (c *Configuration) FromJson(configFile string) (err error) {
	var file *os.File

	if file, err = os.Open(filepath.Clean(configFile)); err != nil {
		return
	}
	defer func() { _ = file.Close() }()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&c)
	return
}

func (c *Configuration) LoadEnv(execFolder string) {
	c.Host = strFromEnv("HOST", "localhost")
	c.Port = intFromEnv("PORT", 8080)
	c.Home = strFromEnv("HOME", execFolder)
	c.Root = strFromEnv("ROOT", execFolder)
	c.Prefix = strFromEnv("PREFIX", "")
	c.TlsCrt = strFromEnv("TLS_CRT", "")
	c.TlsKey = strFromEnv("TLS_KEY", "")
	c.Http2 = boolFromEnv("HTTP2", false)
	c.CacheExpiresAfter = durationFromEnv("CACHE_EXPIRES_AFTER", 0)
	c.NotifyAfter = durationFromEnv("NOTIFY_AFTER", JsonDuration(30*time.Second))
	c.DiscordWebhook = strFromEnv("DISCORD_WEBHOOK", "")
	c.DiscordName = strFromEnv("DISCORD_NAME", "Gallery")
	c.PublicHost = strFromEnv("PUBLIC_HOST", "")
	c.Quiet = boolFromEnv("QUIET", false)
	c.ConfigFile = strFromEnv("CONFIG", "")
	c.ThumbWidth = intFromEnv("THUMB_W", 400)
	c.ThumbHeight = intFromEnv("THUMB_H", 400)
}

type JsonDuration time.Duration

// Parses duration from float64 or string
func (d *JsonDuration) UnmarshalJSON(b []byte) error {
	var v any
	_ = json.Unmarshal(b, &v)
	switch value := v.(type) {
	case float64:
		*d = JsonDuration(time.Duration(value))
		return nil
	case string:
		tmp, err := time.ParseDuration(value)
		if err != nil {
			return err
		}
		*d = JsonDuration(tmp)
		return nil
	default:
		return errors.New("invalid duration")
	}
}

////////////////////////////////////////////////////////////////////////////////

func fromEnv(envName string, parseVal func(string) any) any {
	if env, ok := os.LookupEnv(EnvPrefix + envName); ok {
		return parseVal(env)
	}
	return nil
}

// Gets a string from env with fallback to default value
func strFromEnv(envName, defaultVal string) string {
	s := fromEnv(envName, func(s string) any {
		return s
	})
	switch v := s.(type) {
	case string:
		return v
	default:
		return defaultVal
	}
}

// Gets a boolean from env with fallback to default value
func boolFromEnv(envName string, defaultVal bool) bool {
	b := fromEnv(envName, func(s string) any {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil
		}
		return b
	})
	switch v := b.(type) {
	case bool:
		return v
	default:
		return defaultVal
	}
}

// Gets a (json)Duration from env with fallback to default value
func durationFromEnv(envName string, defaultVal JsonDuration) JsonDuration {
	d := fromEnv(envName, func(s string) any {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil
		}
		return JsonDuration(d)
	})
	switch v := d.(type) {
	case JsonDuration:
		return v
	default:
		return defaultVal
	}
}

// Gets an integer from env with fallback to default value
func intFromEnv(envName string, defaultVal int) int {
	i := fromEnv(envName, func(s string) any {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		return i
	})
	switch v := i.(type) {
	case int:
		return v
	default:
		return defaultVal
	}
}
