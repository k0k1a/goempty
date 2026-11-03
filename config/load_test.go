package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setupConfig(t *testing.T, name, content string) {
	t.Helper()
	t.Setenv("APP_NAME", "")
	t.Setenv("ENV", "")
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "configs"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "configs", name), []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

func TestLoadDatabaseConfiguration(t *testing.T) {
	setupConfig(t, "app_dev.yaml", `mysql:
  dsn: "local:local_password@tcp(127.0.0.1:3306)/goempty?parseTime=true&timeout=5s&readTimeout=5s&writeTimeout=5s"
  max_open_conns: 10
  max_idle_conns: 5
  conn_max_lifetime: 30m
  conn_max_idle_time: 5m
`)
	t.Setenv("GOEMPTY_MYSQL_DSN", "ignored")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.MySQL.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.MySQL.MaxOpenConns != 10 || cfg.MySQL.MaxIdleConns != 5 ||
		cfg.MySQL.ConnMaxLifetime != 30*time.Minute || cfg.MySQL.ConnMaxIdleTime != 5*time.Minute {
		t.Fatal("database pool configuration was not decoded")
	}
	if Get().MySQL != cfg.MySQL {
		t.Fatal("database configuration snapshot differs from startup configuration")
	}
	// Entry-point settings belong only to the startup copy, not the file snapshot.
	cfg.HTTP.Addr = ":8080"
	cfg.ShutdownTimeout = 10 * time.Second
	if snapshot := Get(); snapshot.HTTP.Addr != "" || snapshot.ShutdownTimeout != 0 {
		t.Fatal("startup settings leaked into the file configuration snapshot")
	}
}

func TestLoadRedisConfiguration(t *testing.T) {
	setupConfig(t, "app_dev.yaml", `redis:
  addr: "127.0.0.1:6379"
  username: "local"
  password: "local_password"
  db: 2
  pool_size: 10
  min_idle_conns: 2
  dial_timeout: 5s
  read_timeout: 3s
  write_timeout: 3s
  pool_timeout: 4s
`)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.Redis
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if r.Addr != "127.0.0.1:6379" || r.Username != "local" || r.Password != "local_password" ||
		r.DB != 2 || r.PoolSize != 10 || r.MinIdleConns != 2 || r.DialTimeout != 5*time.Second ||
		r.ReadTimeout != 3*time.Second || r.WriteTimeout != 3*time.Second || r.PoolTimeout != 4*time.Second {
		t.Fatal("Redis configuration was not decoded")
	}
	if Get().Redis != r {
		t.Fatal("Redis snapshot differs from startup configuration")
	}
}

func TestLoadEnvironmentSelection(t *testing.T) {
	for _, tt := range []struct {
		name, appName, env, file string
	}{
		{name: "defaults", file: "app_dev.yaml"},
		{name: "production", appName: "app", env: "prod", file: "app_prod.yaml"},
		{name: "another service", appName: "worker", env: "test", file: "worker_test.yaml"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupConfig(t, tt.file, "{}\n")
			if output, err := loadConfigProcess(t, tt.appName, tt.env); err != nil {
				t.Fatalf("load process: %v\n%s", err, output)
			}
		})
	}
}

func TestLoadUnsetEnvironmentAndRemovedOverrides(t *testing.T) {
	setupConfig(t, "app_dev.yaml", "{}\n")
	for _, key := range []string{"APP_NAME", "ENV"} {
		if err := os.Unsetenv(key); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PORT", "invalid") // PORT is exclusively the entry point's concern.
	t.Setenv("GOEMPTY_HTTP_ADDR", "invalid")
	t.Setenv("GOEMPTY_HTTP_READ_TIMEOUT", "invalid")
	t.Setenv("GOEMPTY_SHUTDOWN_TIMEOUT", "invalid")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}

func TestLoadInvalidConfiguration(t *testing.T) {
	for _, tt := range []struct {
		name, content, want string
	}{
		{name: "malformed yaml", content: "setting: [", want: "config: read"},
		{name: "unknown field", content: "unknown: true", want: "config: decode"},
		{name: "removed http section", content: "http:\n  addr: ':9090'", want: "config: decode"},
		{name: "removed shutdown timeout", content: "shutdown_timeout: 20s", want: "config: decode"},
		{name: "invalid database duration", content: "mysql:\n  conn_max_lifetime: invalid", want: "config: decode"},
		{name: "unknown database option", content: "mysql:\n  unknown: true", want: "config: decode"},
		{name: "invalid Redis duration", content: "redis:\n  dial_timeout: invalid", want: "config: decode"},
		{name: "unknown Redis option", content: "redis:\n  unknown: true", want: "config: decode"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setupConfig(t, "app_dev.yaml", tt.content)
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Load error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestLoadMissingEnvironmentFile(t *testing.T) {
	setupConfig(t, "app_dev.yaml", "{}\n")
	if output, err := loadConfigProcess(t, "app", "missing"); err == nil || !strings.Contains(string(output), "app_missing.yaml") {
		t.Fatalf("load process = %v, %s; want missing selected file without fallback", err, output)
	}
}

func TestEnvironmentFiles(t *testing.T) {
	t.Chdir("..")
	for _, environment := range []string{"dev", "test", "prod"} {
		if output, err := loadConfigProcess(t, "app", environment); err != nil {
			t.Errorf("%s configuration: %v\n%s", environment, err, output)
		}
	}
}

// A fresh process reads APP_NAME and ENV before package initialization.
func loadConfigProcess(t *testing.T, appName, environment string) ([]byte, error) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestLoadConfigProcess$")
	cmd.Env = append(os.Environ(), "GOEMPTY_CONFIG_LOAD_TEST=1", "APP_NAME="+appName, "ENV="+environment, "LEVEL=")
	return cmd.CombinedOutput()
}

func TestLoadConfigProcess(t *testing.T) {
	if os.Getenv("GOEMPTY_CONFIG_LOAD_TEST") != "1" {
		t.Skip("subprocess helper")
	}
	// Changing the OS environment after initialization must not select another file.
	t.Setenv("APP_NAME", "changed")
	t.Setenv("ENV", "changed")
	if _, err := Load(); err != nil {
		t.Fatal(err)
	}
}
