.PHONY: build test lint fmt cover

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/hood ./cmd/hood

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofumpt -l -w .

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1
