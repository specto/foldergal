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
	Home              string
	Root              string
	Cache             string `json:"-"`
	Prefix            string
	TlsCrt            string
	TlsKey            string
	CacheExpiresAfter JsonDuration
	NotifyAfter       JsonDuration
	DiscordName       string
	DiscordWebhook    string
	PublicHost        string
	Copyright         string
	Ffmpeg            string
	ConfigFile        string `json:"-"`
	PublicUrl         string `json:"-"`
	TimeZone          string
	TimeLocation      *time.Location `json:"-"`
	Log               *log.Logger    `json:"-"`
	Port              int
	ThumbWidth        int
	ThumbHeight       int
	Quiet             bool
	Http2             bool
}

// Loads configuration from json file
func (c *Configuration) FromJson(configFile string) (err error) {
	var file *os.File

	if file, err = os.Open(filepath.Clean(configFile)); err != nil {
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&c)
	return
}

func (c *Configuration) LoadEnv(execFolder string) {
	c.Host = strFromEnv("HOST", "localhost")
	c.Port = intFromEnv("PORT", 8080)
	c.Home = strFromEnv("HOME", "")
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
	c.ThumbWidth = intFromEnv("THUMB_WIDTH", 400)
	c.ThumbHeight = intFromEnv("THUMB_HEIGHT", 400)
	c.Copyright = strFromEnv("COPYRIGHT", "")
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

func fromEnv[T comparable](envName string, parseVal func(string) T) T {
	if env, ok := os.LookupEnv(EnvPrefix + envName); ok {
		return parseVal(env)
	}
	return parseVal("")
}

// Gets a string from env with fallback to default value
func strFromEnv(envName, defaultVal string) string {
	if s := fromEnv(envName, func(s string) string { return s }); s != "" {
		return s
	}
	return defaultVal
}

// Gets a boolean from env with fallback to default value
func boolFromEnv(envName string, defaultVal bool) bool {
	return fromEnv(envName, func(s string) bool {
		if b, err := strconv.ParseBool(s); err == nil {
			return b
		}
		return defaultVal
	})
}

// Gets a (json)Duration from env with fallback to default value
func durationFromEnv(envName string, defaultVal JsonDuration) JsonDuration {
	return fromEnv(envName, func(s string) JsonDuration {
		if d, err := time.ParseDuration(s); err == nil {
			return JsonDuration(d)
		}
		return defaultVal
	})
}

// Gets an integer from env with fallback to default value
func intFromEnv(envName string, defaultVal int) int {
	return fromEnv(envName, func(s string) int {
		if i, err := strconv.Atoi(s); err == nil {
			return i
		}
		return defaultVal
	})
}
