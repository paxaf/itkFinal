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

.PHONY: build install-tools lint mocks mocks-debug test test-all test-integration test-skip-integration-pg tidy

mocks-debug:
	which go
	command -v mockery || true
	echo $(MOCKERY)
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list github.com/paxaf/itkFinal/gw-exchanger/internal/storages
	go list github.com/paxaf/itkFinal/gw-exchanger/internal/usecase
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
