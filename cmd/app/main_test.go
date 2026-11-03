package main

import (
	"strings"
	"testing"
)

func TestRunRejectsInvalidPort(t *testing.T) {
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
