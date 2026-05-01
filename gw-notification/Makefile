override export GO111MODULE := on

MOCKERY_VERSION := v2.53.5

ifeq ($(OS),Windows_NT)
WIN_USERPROFILE := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('UserProfile')"))
WIN_LOCALAPPDATA := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('LocalApplicationData')"))
override export USERPROFILE := $(WIN_USERPROFILE)
override export LOCALAPPDATA := $(WIN_LOCALAPPDATA)
GOPATH_BIN := $(shell cygpath -u "$$(go env GOPATH)")/bin
else
GOPATH_BIN := $(shell go env GOPATH)/bin
endif

MOCKERY := $(shell command -v mockery 2>/dev/null)
ifeq ($(MOCKERY),)
MOCKERY := $(GOPATH_BIN)/mockery
endif

.PHONY: build install-tools lint mocks mocks-debug test test-all test-integration test-skip-integration-mongo tidy test-debug

build:
	go build -o main ./cmd

lint:
	golangci-lint run

mocks-debug:
	which go
	command -v mockery || true
	echo $(MOCKERY)
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list github.com/paxaf/itkFinal/gw-notification/internal/storages
	go list github.com/paxaf/itkFinal/gw-notification/internal/usecase
	go list github.com/paxaf/itkFinal/gw-notification/internal/transport/kafka
	go list github.com/paxaf/itkFinal/gw-notification/internal/logger
	$(MOCKERY) --config .mockery.yaml --log-level debug

install-tools:
	go install github.com/vektra/mockery/v2@$(MOCKERY_VERSION)

mocks:
	@if [ ! -x "$(MOCKERY)" ]; then \
		echo "mockery not found: $(MOCKERY)"; \
		echo "Run: make install-tools"; \
		echo "If it still fails, add $$(go env GOPATH)/bin to PATH"; \
		exit 127; \
	fi
	$(MOCKERY) --config .mockery.yaml

test: mocks
	go test ./... -skip Integration

test-all: mocks
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
