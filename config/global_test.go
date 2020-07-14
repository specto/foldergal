package config

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestFromJson(t *testing.T) {
	var testCfg Configuration

	fileNotFound := "open ./testdata/non_existing.json: no such file or directory"
	if err := testCfg.FromJson("./testdata/non_existing.json"); err == nil || err.Error() != fileNotFound {
		t.Errorf("Expected error '%v', got '%v'", fileNotFound, err)
	}

	badKeyErr := `json: unknown field "evil"`
	if err := testCfg.FromJson("./testdata/bad_key_config.json"); err == nil || err.Error() != badKeyErr {
		t.Errorf("Expected error '%v', got '%v'", badKeyErr, err)
	}

	if err := testCfg.FromJson("../config.default.json"); err != nil {
		t.Errorf("config.default.json: '%v'", err)
	}

	if err := testCfg.FromJson("./testdata/valid.json"); err != nil {
		t.Errorf("valid.json: '%v'", err)
	}

	if err := testCfg.FromJson("./testdata/invalid.json"); err == nil {
		t.Errorf("invalid.json: '%v'", err)
	}

	invalidDurationStr := "time: invalid duration blabla"
	if err := testCfg.FromJson("./testdata/duration_invalid_string.json"); err == nil || err.Error() != invalidDurationStr {
		t.Errorf("Expected error '%v', got '%v'", invalidDurationStr, err)
	}

	invalidDuration := "invalid duration"
	if err := testCfg.FromJson("./testdata/duration_invalid.json"); err == nil || err.Error() != invalidDuration {
		t.Errorf("Expected error '%v', got '%v'", invalidDuration, err)
	}
}

func TestSfromEnv(t *testing.T) {
	key := "stringval"
	val := "A web gallery"

	_ = os.Setenv(envPrefix+key, val)
	if res := SfromEnv(key, ""); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	val = "what"
	if res := SfromEnv("non_existing", val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}

func TestBfromEnv(t *testing.T) {
	key := "boolval"
	val := true

	_ = os.Setenv(envPrefix+key, "1")
	if res := BfromEnv(key, false); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	val = true
	if res := BfromEnv("non_existing", val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	_ = os.Setenv(envPrefix+key, "invalid")
	val = true
	if res := BfromEnv(key, val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}

func TestIfromEnv(t *testing.T) {
	key := "intval"
	val := 1234

	_ = os.Setenv(envPrefix+key, strconv.Itoa(val))
	if res := IfromEnv(key, 0); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	val = 9876
	if res := IfromEnv("non_existing", val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	_ = os.Setenv(envPrefix+key, "impossible")
	val = 10
	if res := IfromEnv(key, val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}

func TestDfromEnv(t *testing.T) {
	key := "durval"
	val := JsonDuration(30 * time.Second)

	_ = os.Setenv(envPrefix+key, "30s")
	if res := DfromEnv(key, 0); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}

	_ = os.Setenv(envPrefix+key, "this is not a valid duration")
	if res := DfromEnv(key, val); res != val {
		t.Errorf("Expected %v, got %v", val, res)
	}
}
