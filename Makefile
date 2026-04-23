.PHONY: build build-release run test

BINARY ?= serve
GO ?= go
ARGS ?=

build:
	$(GO) build -o ./bin/$(BINARY) ./cmd/serve

build-release:
	$(GO) build -trimpath -ldflags="-s -w" -o ./bin/$(BINARY) ./cmd/serve

run:
	$(GO) run ./cmd/serve $(ARGS)

test:
	$(GO) test ./...
