.PHONY: build test test-integration test-skip-integration test-skip-integration-pg tidy

build:
	go build -o main ./cmd

test:
	go test ./...

test-integration:
	go test ./internal/storages/postgres

test-skip-integration:
	go test ./... -skip Integration

test-skip-integration-pg:
	go test ./internal/storages/postgres -skip-integration

tidy:
	go mod tidy
