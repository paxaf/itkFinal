override export GO111MODULE := on

ifeq ($(OS),Windows_NT)
WIN_USERPROFILE := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('UserProfile')"))
WIN_LOCALAPPDATA := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('LocalApplicationData')"))
override export USERPROFILE := $(WIN_USERPROFILE)
override export LOCALAPPDATA := $(WIN_LOCALAPPDATA)
endif

.PHONY: build lint mocks mocks-debug test test-all test-integration test-skip-integration-pg tidy

mocks-debug:
	which go
	which mockery
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list github.com/paxaf/itkFinal/gw-exchanger/internal/storages
	go list github.com/paxaf/itkFinal/gw-exchanger/internal/usecase
	mockery --config .mockery.yaml --log-level debug

mocks:
	mockery --config .mockery.yaml

build:
	go build -o main ./cmd

lint:
	golangci-lint run

test: mocks
	go test ./... -skip Integration

test-all: mocks
	go test ./...

test-integration:
	go test ./internal/storages/postgres

test-skip-integration-pg:
	go test ./internal/storages/postgres -skip-integration

tidy:
	go mod tidy
