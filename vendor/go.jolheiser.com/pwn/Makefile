GO ?= go
VERSION ?= $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: test
test:
	$(GO) test -race ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: build
build:
	$(GO) build -ldflags '-s -w -X "main.Version=$(VERSION)"' go.jolheiser.com/pwn/cmd/pwn