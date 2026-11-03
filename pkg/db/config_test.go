package db

import "testing"

func TestValidateDSNTimeouts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		params string
		valid  bool
	}{
		{"positive", "timeout=5s&readTimeout=5s&writeTimeout=5s", true},
		{"missing", "", false},
		{"missing connect", "readTimeout=5s&writeTimeout=5s", false},
		{"missing read", "timeout=5s&writeTimeout=5s", false},
		{"missing write", "timeout=5s&readTimeout=5s", false},
		{"zero", "timeout=0s&readTimeout=5s&writeTimeout=5s", false},
		{"negative", "timeout=5s&readTimeout=-1s&writeTimeout=5s", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{
				DSN:          "user@tcp(127.0.0.1:3306)/test?parseTime=true&" + tc.params,
				MaxOpenConns: 1,
			}
			if err := cfg.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, want valid=%v", err, tc.valid)
			}
		})
	}
}
