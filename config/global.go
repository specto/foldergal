package config

import (
	"encoding/json"
	"errors"
	"log"
	"os"
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

func (c *Configuration) FromJson(configFile string) (err error) {
	var file *os.File
	/* #nosec */
	if file, err = os.Open(configFile); err != nil {
		return
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&c)
	return
}

type JsonDuration time.Duration

//func (d JsonDuration) MarshalJSON() ([]byte, error) {
//	return json.Marshal(time.Duration(d).String())
//}

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

// Get a string from env or use default
func SfromEnv(envName string, defaultVal string) string {
	s := fromEnv(envName, func(s string) interface{} {
		return s
	})
	switch s.(type) {
	case string:
		return s.(string)
	default:
		return defaultVal
	}
}

// Get a boolean from env or use default
func BfromEnv(envName string, defaultVal bool) bool {
	b := fromEnv(envName, func(s string) interface{} {
		b, err := strconv.ParseBool(s)
		if err != nil {
			return nil
		}
		return b
	})
	switch b.(type) {
	case bool:
		return b.(bool)
	default:
		return defaultVal
	}
}

// Get a (json)Duration from env or use default
func DfromEnv(envName string, defaultVal JsonDuration) JsonDuration {
	d := fromEnv(envName, func(s string) interface{} {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil
		}
		return JsonDuration(d)
	})
	switch d.(type) {
	case JsonDuration:
		return d.(JsonDuration)
	default:
		return defaultVal
	}
}

// Get an integer from env or use default
func IfromEnv(envName string, defaultVal int) int {
	i := fromEnv(envName, func(s string) interface{} {
		i, err := strconv.Atoi(s)
		if err != nil {
			return nil
		}
		return i
	})
	switch i.(type) {
	case int:
		return i.(int)
	default:
		return defaultVal
	}
}
