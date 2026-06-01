.PHONY: build test lint fmt cover

build:
	go build -o bin/hood ./cmd/hood

test:
	go test ./...

lint:
	golangci-lint run ./...

fmt:
	gofumpt -l -w .

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1
