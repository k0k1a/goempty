package env_test

import (
	"os"
	"os/exec"
	"testing"

	"goempty/pkg/env"
)

func TestApplicationEnvironment(t *testing.T) {
	for _, tc := range []struct{ name, appName, environment, wantApp, wantEnv string }{
		{"defaults", "", "", "app", "dev"},
		{"custom", "order", "prod", "order", "prod"},
		{"unchanged", "../custom", "test", "../custom", "test"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestApplicationEnvironmentProcess$")
			cmd.Env = append(os.Environ(), "GOEMPTY_ENV_TEST=1", "APP_NAME="+tc.appName,
				"ENV="+tc.environment, "WANT_APP_NAME="+tc.wantApp, "WANT_ENV="+tc.wantEnv)
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("environment process: %v\n%s", err, output)
			}
		})
	}
}

func TestApplicationEnvironmentProcess(t *testing.T) {
	if os.Getenv("GOEMPTY_ENV_TEST") != "1" {
		t.Skip("subprocess helper")
	}
	wantApp, wantEnv := os.Getenv("WANT_APP_NAME"), os.Getenv("WANT_ENV")
	if env.AppName() != wantApp || env.Env() != wantEnv {
		t.Fatalf("got %q/%q, want %q/%q", env.AppName(), env.Env(), wantApp, wantEnv)
	}
	t.Setenv("APP_NAME", "changed")
	t.Setenv("ENV", "changed")
	if env.AppName() != wantApp || env.Env() != wantEnv {
		t.Fatal("application identity must remain fixed after initialization")
	}
}
