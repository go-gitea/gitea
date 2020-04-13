PROJECT_ROOT_DIR := $(CURDIR)
SRC := $(shell git ls-files *.go */*.go)

.PHONY: bin test test-go test-core submodule

test: test-go test-core

submodule:
	git submodule update --init

editorconfig: $(SRC)
	go build ./cmd/editorconfig

test-go:
	go test -v ./...

test-core: editorconfig
	cd core-test; cmake ..
	cd core-test; ctest -E "(comments_after_section|octothorpe|unset_|_pre_0.9.0|max_|root_file_mixed_case)" --output-on-failure .
