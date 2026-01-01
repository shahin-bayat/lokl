.PHONY: build test lint clean install run fmt check

BINARY=devenv
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/shahin-bayat/devenv/internal/version.Version=$(VERSION)"
LOCAL_PREFIX=github.com/shahin-bayat/devenv

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/devenv

test:
	go test -race -v ./...

lint:
	golangci-lint run

fmt:
	goimports -w -local $(LOCAL_PREFIX) .
	go fmt ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

install:
	go install $(LDFLAGS) ./cmd/devenv

run:
	go run ./cmd/devenv $(ARGS)

check: fmt
	go build ./...
	go test -race ./...
	golangci-lint run
