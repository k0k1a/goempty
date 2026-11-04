// Command generate-api-docs generates importable Swagger files from Go annotations.
// Run it from the project root with go run ./scripts/generate-api-docs.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("go", "run", "github.com/swaggo/swag/cmd/swag@v1.16.6", "init",
		"--generalInfo", "main.go",
		"--dir", "cmd/app,internal/app,internal/controller,internal/entity",
		"--parseInternal",
		"--output", "docs/swagger",
		"--outputTypes", "json,yaml",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Generate API documentation: %v\n", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
