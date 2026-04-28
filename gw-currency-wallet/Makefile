WIN_USERPROFILE := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('UserProfile')"))
WIN_LOCALAPPDATA := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('LocalApplicationData')"))

override export GO111MODULE := on
override export USERPROFILE := $(WIN_USERPROFILE)
override export LOCALAPPDATA := $(WIN_LOCALAPPDATA)

.PHONY: build test test-all test-integration test-skip-integration-pg tidy test-debug

test-debug:
	which go
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list ./...

build:
	go build -o main ./cmd

test:
	go test ./... -skip Integration

test-all:
	go test ./...

test-integration:
	go test ./internal/storages/postgres

test-skip-integration-pg:
	go test ./internal/storages/postgres -skip-integration

tidy:
	go mod tidy