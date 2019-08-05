PROJECT_ROOT_DIR := $(CURDIR)
SRC := editorconfig.go cmd/editorconfig/main.go

.PHONY: bin test test-go test-core submodule installdeps

test: test-go test-core

submodule:
	git submodule update --init

installdeps:
	go get -t ./...

editorconfig: $(SRC)
	go build ./cmd/editorconfig

test-go:
	go test -v

test-core: editorconfig
	cd $(PROJECT_ROOT_DIR)/core-test && \
		cmake -DEDITORCONFIG_CMD="$(PROJECT_ROOT_DIR)/editorconfig" .
# Temporarily disable core-test
	# cd $(PROJECT_ROOT_DIR)/core-test && \
	# 	ctest --output-on-failure .
