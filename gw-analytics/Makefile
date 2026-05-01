WIN_USERPROFILE := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('UserProfile')"))
WIN_LOCALAPPDATA := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('LocalApplicationData')"))

override export GO111MODULE := on
override export USERPROFILE := $(WIN_USERPROFILE)
override export LOCALAPPDATA := $(WIN_LOCALAPPDATA)

.PHONY: build lint mocks mocks-debug test test-all tidy test-debug

build:
	go build -o main ./cmd

lint:
	golangci-lint run

mocks-debug:
	which go
	which mockery
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list github.com/paxaf/itkFinal/gw-analytics/internal/storages
	go list github.com/paxaf/itkFinal/gw-analytics/internal/usecase
	go list github.com/paxaf/itkFinal/gw-analytics/internal/transport/kafka
	go list github.com/paxaf/itkFinal/gw-analytics/internal/logger
	mockery --config .mockery.yaml --log-level debug

mocks:
	mockery --config .mockery.yaml

test: mocks
	go test ./...

test-all: mocks
	go test ./...

tidy:
	go mod tidy

test-debug:
	which go
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list ./...
