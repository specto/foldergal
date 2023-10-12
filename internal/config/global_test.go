package config

import (
	"errors"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestFromJson(t *testing.T) {
	var testCfg Configuration

	if err := testCfg.FromJson("./testdata/non_existing.json"); err == nil || !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Expected ErrNotExist error, got '%v'", err)
	}

	badKeyErr := `json: unknown field "evil"`
	if err := testCfg.FromJson("./testdata/bad_key_config.json"); err == nil || err.Error() != badKeyErr {
		t.Errorf("Expected error '%v', got '%v'", badKeyErr, err)
	}

	if err := testCfg.FromJson("../../config.default.json"); err != nil {
		t.Errorf("config.default.json: '%v'", err)
	}

	if err := testCfg.FromJson("./testdata/valid.json"); err != nil {
		t.Errorf("valid.json: '%v'", err)
	}

	if err := testCfg.FromJson("./testdata/invalid.json"); err == nil {
		t.Errorf("invalid.json: '%v'", err)
	}

	invalidDurationStr := `time: invalid duration "blabla"`
	if err := testCfg.FromJson("./testdata/duration_invalid_string.json"); err == nil || err.Error() != invalidDurationStr {
		t.Errorf("Expected error '%v', got '%v'", invalidDurationStr, err)
	}

	invalidDuration := "invalid duration"
	if err := testCfg.FromJson("./testdata/duration_invalid.json"); err == nil || err.Error() != invalidDuration {
		t.Errorf("Expected error '%v', got '%v'", invalidDuration, err)
	}
}

func TestStrFromEnv(t *testing.T) {
	key := "stringval"
	val := "A web gallery"

	_ = os.Setenv(EnvPrefix+key, val)
	if res := strFromEnv(key, ""); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	val = "what"
	if res := strFromEnv("non_existing", val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}

func TestBoolFromEnv(t *testing.T) {
	key := "boolval"
	val := true

	_ = os.Setenv(EnvPrefix+key, "1")
	if res := boolFromEnv(key, false); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	val = true
	if res := boolFromEnv("non_existing", val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	_ = os.Setenv(EnvPrefix+key, "invalid")
	val = true
	if res := boolFromEnv(key, val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}

func TestIntFromEnv(t *testing.T) {
	key := "intval"
	val := 1234

	_ = os.Setenv(EnvPrefix+key, strconv.Itoa(val))
	if res := intFromEnv(key, 0); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	val = 9876
	if res := intFromEnv("non_existing", val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	_ = os.Setenv(EnvPrefix+key, "impossible")
	val = 10
	if res := intFromEnv(key, val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}

func TestDurationFromEnv(t *testing.T) {
	key := "durval"
	val := JsonDuration(30 * time.Second)

	_ = os.Setenv(EnvPrefix+key, "30s")
	if res := durationFromEnv(key, 0); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	_ = os.Setenv(EnvPrefix+key, "this is not a valid duration")
	if res := durationFromEnv(key, val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}

func TestLoadEnv(t *testing.T) {
	execFolder, _ := os.Getwd()
	Global.LoadEnv(execFolder)
}
