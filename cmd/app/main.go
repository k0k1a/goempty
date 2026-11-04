package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"goempty/config"
	"goempty/internal/app"
	"goempty/pkg/httpserver"
	"goempty/pkg/log"

	"go.uber.org/zap"
)

// @title goempty API
// @version 1.0
// @description goempty HTTP API. Ordinary JSON responses use code/message/data; code 0 means success. Request IDs are returned in the X-Request-ID header. Health endpoints return an empty body.
// @BasePath /
func main() {
	ctx := context.Background()
	exitCode := 0
	if err := run(); err != nil {
		log.Error(ctx, "Application exited with an error", zap.Error(err))
		exitCode = 1
	} else {
		log.Info(ctx, "Application stopped")
	}
	if err := log.Shutdown(); err != nil {
		fmt.Fprintf(os.Stderr, "Close logger: %v\n", err)
		exitCode = 1
	}
	if exitCode != 0 {
		os.Exit(exitCode)
	}
}

func run() error {
	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("PORT must be an integer between 1 and 65535")
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	addr := ":" + strconv.Itoa(portNumber)
	log.Info(ctx, "Application starting", zap.String("addr", addr))

	cfg.HTTP = httpserver.Config{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	cfg.ShutdownTimeout = 10 * time.Second
	application, err := app.Initialize(ctx, cfg)
	if err != nil {
		return fmt.Errorf("initialize application: %w", err)
	}
	defer application.Close()
	return application.Run(ctx)
}
