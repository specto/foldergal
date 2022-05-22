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
	envPrefix = "FOLDERGAL_"
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

type JsonDuration time.Duration

// Parses duration from float64 or string
func (d *JsonDuration) UnmarshalJSON(b []byte) error {
	var v interface{}
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

func fromEnv(envName string, parseVal func(string) interface{}) interface{} {
	if env, ok := os.LookupEnv(envPrefix + envName); ok {
		return parseVal(env)
	}
	return nil
}

// Gets a string from env with fallback to default value
func SfromEnv(envName, defaultVal string) string {
	s := fromEnv(envName, func(s string) interface{} {
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
func BfromEnv(envName string, defaultVal bool) bool {
	b := fromEnv(envName, func(s string) interface{} {
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
func DfromEnv(envName string, defaultVal JsonDuration) JsonDuration {
	d := fromEnv(envName, func(s string) interface{} {
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
func IfromEnv(envName string, defaultVal int) int {
	i := fromEnv(envName, func(s string) interface{} {
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
