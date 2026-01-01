.PHONY: build test lint clean install run fmt check

BINARY=lokl
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X github.com/shahin-bayat/lokl/internal/version.Version=$(VERSION)"
LOCAL_PREFIX=github.com/shahin-bayat/lokl

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/lokl

test:
	go test -race -v ./...

lint:
	go vet ./...

fmt:
	goimports -w -local $(LOCAL_PREFIX) .
	go fmt ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

install:
	go install $(LDFLAGS) ./cmd/lokl

run:
	go run ./cmd/lokl $(ARGS)

check: fmt build test lint
