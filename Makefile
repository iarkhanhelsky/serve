.PHONY: build run test

BINARY ?= serve
GO ?= go
ARGS ?=

build:
	$(GO) build -o ./bin/$(BINARY) ./cmd/serve

run:
	$(GO) run ./cmd/serve $(ARGS)

test:
	$(GO) test ./...
