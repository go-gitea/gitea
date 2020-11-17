PROJECT_ROOT_DIR := $(CURDIR)
SRC := $(shell git ls-files *.go */*.go)

.PHONY: bin test test-go test-core test-skipped submodule

test: test-go test-core

submodule:
	git submodule update --init

editorconfig: $(SRC)
	go build \
		-ldflags "-X main.version=1.99.99" \
		github.com/editorconfig/editorconfig-core-go/v2/cmd/editorconfig

test-go:
	go test -v ./...

test-core: editorconfig
	cd core-test; \
		cmake ..
	cd core-test; \
		ctest \
		-E "^octothorpe_in_value$$" \
		--output-on-failure \
		.

test-skipped: editorconfig
	cd core-test; \
		cmake ..
	cd core-test; \
		ctest \
		-R "^octothorpe_in_value$$" \
		--show-only \
		.
