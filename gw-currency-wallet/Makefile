WIN_USERPROFILE := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('UserProfile')"))
WIN_LOCALAPPDATA := $(strip $(shell powershell.exe -NoProfile -Command "[Environment]::GetFolderPath('LocalApplicationData')"))

override export GO111MODULE := on
override export USERPROFILE := $(WIN_USERPROFILE)
override export LOCALAPPDATA := $(WIN_LOCALAPPDATA)

.PHONY: build lint swagger mocks mocks-debug test test-all test-integration test-skip-integration-pg tidy test-debug

test-debug:
	which go
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list ./...

mocks-debug:
	which go
	which mockery
	go env GOMOD GOPATH GOMODCACHE GOCACHE GOROOT
	go list github.com/paxaf/itkFinal/gw-currency-wallet/internal/storages
	go list github.com/paxaf/itkFinal/gw-currency-wallet/internal/usecase
	go list github.com/paxaf/itkFinal/gw-currency-wallet/internal/transport/http
	mockery --config .mockery.yaml --log-level debug

mocks:
	mockery --config .mockery.yaml

build:
	go build -o main ./cmd

lint:
	golangci-lint run

swagger:
	go run github.com/swaggo/swag/cmd/swag@v1.8.12 init --dir ./cmd,./internal/transport/http --generalInfo main.go --output docs --parseInternal

test: mocks
	go test ./... -skip Integration

test-all:
	go test ./...

test-integration:
	go test ./internal/storages/postgres

test-skip-integration-pg:
	go test ./internal/storages/postgres -skip-integration

tidy:
	go mod tidy
