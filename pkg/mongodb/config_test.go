package mongodb

import (
	"strings"
	"testing"
	"time"
)

func TestConfigValidate(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"empty URI", Config{}},
		{"invalid scheme", Config{URI: "https://localhost:27017"}},
		{"invalid URI option", Config{URI: "mongodb://localhost/?maxPoolSize=secret-value"}},
		{"negative connect timeout", Config{URI: "mongodb://localhost", ConnectTimeout: -time.Second}},
		{"negative selection timeout", Config{URI: "mongodb://localhost", ServerSelectionTimeout: -time.Second}},
		{"negative operation timeout", Config{URI: "mongodb://localhost", Timeout: -time.Second}},
		{"negative idle time", Config{URI: "mongodb://localhost", MaxConnIdleTime: -time.Second}},
		{"invalid pool sizes", Config{URI: "mongodb://localhost", MaxPoolSize: 1, MinPoolSize: 2}},
		{"negative max pool size", Config{URI: "mongodb://localhost", MaxPoolSize: -1}},
		{"negative min pool size", Config{URI: "mongodb://localhost", MinPoolSize: -1}},
		{"default max exceeded", Config{URI: "mongodb://localhost", MinPoolSize: defaultMaxPoolSize + 1}},
		{"URI pool conflict", Config{URI: "mongodb://localhost/?maxPoolSize=1", MinPoolSize: 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatal("Validate() succeeded for invalid settings")
			}
		})
	}
}

func TestConfigErrorsDoNotExposeURI(t *testing.T) {
	cfg := Config{URI: "mongodb://private-user:private-password@localhost/?maxPoolSize=private-value"}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() succeeded for invalid URI")
	}
	for _, secret := range []string{cfg.URI, "private-user", "private-password", "private-value"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatal("validation error exposed URI contents")
		}
	}
}

func TestConfigOptionPrecedence(t *testing.T) {
	cfg := Config{
		URI: "mongodb://localhost/?maxPoolSize=8&minPoolSize=2&maxIdleTimeMS=1000&connectTimeoutMS=2000&serverSelectionTimeoutMS=3000&timeoutMS=4000",
	}
	opts, err := cfg.clientOptions()
	if err != nil {
		t.Fatal(err)
	}
	if *opts.MaxPoolSize != 8 || *opts.MinPoolSize != 2 || *opts.MaxConnIdleTime != time.Second ||
		*opts.ConnectTimeout != 2*time.Second || *opts.ServerSelectionTimeout != 3*time.Second || *opts.Timeout != 4*time.Second {
		t.Fatal("zero fields did not retain URI options")
	}
	cfg.MaxPoolSize, cfg.MinPoolSize = 10, 3
	cfg.MaxConnIdleTime, cfg.ConnectTimeout = 5*time.Second, 6*time.Second
	cfg.ServerSelectionTimeout, cfg.Timeout = 7*time.Second, 8*time.Second
	opts, err = cfg.clientOptions()
	if err != nil {
		t.Fatal(err)
	}
	if *opts.MaxPoolSize != 10 || *opts.MinPoolSize != 3 || *opts.MaxConnIdleTime != 5*time.Second ||
		*opts.ConnectTimeout != 6*time.Second || *opts.ServerSelectionTimeout != 7*time.Second || *opts.Timeout != 8*time.Second {
		t.Fatal("positive fields did not override URI options")
	}
}

func TestConfigDefaultsAndUnlimitedPool(t *testing.T) {
	for _, cfg := range []Config{
		{URI: "mongodb://localhost"},
		{URI: "mongodb://localhost/?maxPoolSize=0", MinPoolSize: defaultMaxPoolSize + 1},
	} {
		if err := cfg.Validate(); err != nil {
			t.Fatal(err)
		}
	}
}
