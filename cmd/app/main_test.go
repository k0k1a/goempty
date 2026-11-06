package main

import (
	"strings"
	"testing"
	"time"
)

func TestRunRejectsInvalidPort(t *testing.T) {
	previous := time.Local
	t.Cleanup(func() { time.Local = previous })
	// The removed address override must not hide an invalid PORT.
	t.Setenv("GOEMPTY_HTTP_ADDR", ":7070")
	for _, port := range []string{"", "abc", "0", "-1", "65536"} {
		t.Run("port="+port, func(t *testing.T) {
			t.Setenv("PORT", port)
			if err := run(); err == nil || !strings.Contains(err.Error(), "PORT must be") {
				t.Fatalf("run = %v, want invalid PORT error", err)
			}
		})
	}
}

func TestInitTimezone(t *testing.T) {
	previous := time.Local
	t.Cleanup(func() { time.Local = previous })
	time.Local = time.UTC
	if err := initTimezone(); err != nil {
		t.Fatal(err)
	}
	if time.Now().Location() != time.Local || time.Local.String() != "Asia/Shanghai" {
		t.Fatalf("application timezone = %s", time.Local)
	}
	local := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.Local)
	_, offset := local.Zone()
	if offset != 8*60*60 {
		t.Fatalf("application UTC offset = %d, want +08:00", offset)
	}
}
