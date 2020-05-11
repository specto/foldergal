package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

var (
	Global            Configuration
	envPrefix         = "FOLDERGAL_"
	RegisteredEnvVars = make(map[string]interface{})
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
	Log               *log.Logger `json:"-"`
}

func (c *Configuration) FromJson(configFile string) (err error) {
	var file *os.File
	/* #nosec */
	if file, err = os.Open(configFile); err != nil {
		return
	}
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&c)
	return
}

type JsonDuration time.Duration

func (d JsonDuration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).String())
}

func (d *JsonDuration) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
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

func fromEnv(envName string,
	defaultVal interface{},
	convertString func(string) interface{}) (property interface{}) {
	RegisteredEnvVars[envPrefix+envName] = nil
	if env := os.Getenv(envPrefix + envName); env != "" {
		property = convertString(env)
	} else if defaultVal != nil {
		RegisteredEnvVars[envPrefix+envName] = defaultVal
		property = defaultVal
	}
	return
}

func envToInt(s string) interface{} {
	i, _ := strconv.Atoi(s)
	return i
}

func envToBool(s string) interface{} {
	b, _ := strconv.ParseBool(s)
	return b
}

func envToString(s string) interface{} { return s }

func envToDuration(s string) interface{} {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return JsonDuration(d)
}

////////////////////////////////////////////////////////////////////////////////

// Get a string from env or use default
func SfromEnv(envName string, defaultVal string) string {
	var s interface{}
	if defaultVal == "" {
		s = fromEnv(envName, nil, envToString)
	} else {
		s = fromEnv(envName, defaultVal, envToString)
	}
	if s == nil {
		return ""
	}
	return fmt.Sprint(s)
}

// Get a boolean from env or use default
func BfromEnv(envName string, defaultVal bool) bool {
	b := fromEnv(envName, defaultVal, envToBool)
	if b == nil {
		return false
	}
	return b.(bool)
}

// Get a (json)Duration from env or use default
func DfromEnv(envName string, defaultVal JsonDuration) JsonDuration {
	d := fromEnv(envName, defaultVal, envToDuration)
	if d == nil {
		return 0
	}
	return d.(JsonDuration)
}

// Get an integer from env or use default
func IfromEnv(envName string, defaultVal int) int {
	i := fromEnv(envName, defaultVal, envToInt)
	if i == nil {
		return 0
	}
	return i.(int)
}
