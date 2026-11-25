package postgres

import "testing"

func TestValidateDSN(t *testing.T) {
	// pgx applies PG* environment variables when a DSN omits a setting.
	t.Setenv("PGDATABASE", "")
	t.Setenv("PGCONNECT_TIMEOUT", "")

	for _, tc := range []struct {
		name  string
		dsn   string
		valid bool
	}{
		{"valid", "postgres://user:pass@127.0.0.1:5432/test?sslmode=disable&connect_timeout=5", true},
		{"keyword", "host=127.0.0.1 port=5432 user=test dbname=test connect_timeout=5", true},
		{"missing database", "postgres://user:pass@127.0.0.1:5432?sslmode=disable&connect_timeout=5", false},
		{"missing connect timeout", "postgres://user:pass@127.0.0.1:5432/test?sslmode=disable", false},
		{"zero connect timeout", "postgres://user:pass@127.0.0.1:5432/test?sslmode=disable&connect_timeout=0", false},
		{"invalid connect timeout", "postgres://user:pass@127.0.0.1:5432/test?sslmode=disable&connect_timeout=abc", false},
		{"malformed", "postgres://user:pass@127.0.0.1:5432/test?%zz", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{DSN: tc.dsn, MaxOpenConns: 1}
			if err := cfg.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, want valid=%v", err, tc.valid)
			}
		})
	}
}

func TestValidatePoolLimits(t *testing.T) {
	valid := Config{
		DSN:             "postgres://user:pass@127.0.0.1:5432/test?sslmode=disable&connect_timeout=5",
		MaxOpenConns:    2,
		MaxIdleConns:    1,
		ConnMaxLifetime: 0,
		ConnMaxIdleTime: 0,
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Config)
		valid  bool
	}{
		{"default", func(*Config) {}, true},
		{"zero open", func(c *Config) { c.MaxOpenConns = 0 }, false},
		{"negative idle", func(c *Config) { c.MaxIdleConns = -1 }, false},
		{"idle exceeds open", func(c *Config) { c.MaxIdleConns = 3 }, false},
		{"negative lifetime", func(c *Config) { c.ConnMaxLifetime = -1 }, false},
		{"negative idle time", func(c *Config) { c.ConnMaxIdleTime = -1 }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := valid
			tc.mutate(&cfg)
			if err := cfg.Validate(); (err == nil) != tc.valid {
				t.Fatalf("Validate() = %v, want valid=%v", err, tc.valid)
			}
		})
	}
}
