.PHONY: build test test-all test-integration test-skip-integration-mongo tidy test-debug

build:
	go build -o main ./cmd

test:
	go test ./... -skip Integration

test-all:
	go test ./...

test-integration:
	go test ./internal/storages/mongo

test-skip-integration-mongo:
	go test ./internal/storages/mongo -skip-integration

tidy:
	go mod tidy

test-debug:
	which go
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list ./...
