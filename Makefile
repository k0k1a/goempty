APP_NAME ?= app
DIST_DIR ?= dist
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
CGO_ENABLED ?= 0
LDFLAGS ?= -s -w

SHELL := /bin/sh
.DEFAULT_GOAL := help

.PHONY: help run build fmt test vet check test-integration wire docs

help:
	@printf '%s\n' \
	  'make run               Run the application' \
	  'make build             Build $(DIST_DIR)/$(APP_NAME) (GOOS/GOARCH configurable)' \
	  'make fmt               Format Go source files' \
	  'make test              Run all tests' \
	  'make vet               Run go vet' \
	  'make check             Run tests, vet and build in order' \
	  'make test-integration  Run service tests (requires isolated services)' \
	  'make wire              Generate Wire code using the latest version' \
	  'make docs              Generate Swagger documentation'

run:
	go run ./cmd/app

build:
	mkdir -p "$(DIST_DIR)"
	CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags="$(LDFLAGS)" -o "$(DIST_DIR)/$(APP_NAME)" ./cmd/app

fmt:
	go fmt ./...

test:
	go test ./...

vet:
	go vet ./...

check:
	$(MAKE) test
	$(MAKE) vet
	go build ./...

test-integration:
	go test -count=1 ./internal/app ./integration-test

wire:
	go run github.com/google/wire/cmd/wire@latest ./internal/app

docs:
	go run ./scripts/generate-api-docs
