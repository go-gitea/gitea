# Package configuration
PROJECT = enry
COMMANDS = cmd/enry

# Including ci Makefile
CI_REPOSITORY ?= https://github.com/src-d/ci.git
CI_BRANCH ?= v1
CI_PATH ?= .ci
MAKEFILE := $(CI_PATH)/Makefile.main
$(MAKEFILE):
	git clone --quiet --depth 1 -b $(CI_BRANCH) $(CI_REPOSITORY) $(CI_PATH);
-include $(MAKEFILE)

# Docsrv: configure the languages whose api-doc can be auto generated
LANGUAGES = go
# Docs: do not edit this
DOCS_REPOSITORY := https://github.com/src-d/docs
SHARED_PATH ?= $(shell pwd)/.docsrv-resources
DOCS_PATH ?= $(SHARED_PATH)/.docs
$(DOCS_PATH)/Makefile.inc:
	git clone --quiet --depth 1 $(DOCS_REPOSITORY) $(DOCS_PATH);
-include $(DOCS_PATH)/Makefile.inc

LINGUIST_PATH = .linguist

# shared objects
RESOURCES_DIR=./.shared
LINUX_DIR=$(RESOURCES_DIR)/linux-x86-64
LINUX_SHARED_LIB=$(LINUX_DIR)/libenry.so
DARWIN_DIR=$(RESOURCES_DIR)/darwin
DARWIN_SHARED_LIB=$(DARWIN_DIR)/libenry.dylib
HEADER_FILE=libenry.h
NATIVE_LIB=./shared/enry.go

$(LINGUIST_PATH):
	git clone https://github.com/github/linguist.git $@

clean-linguist:
	rm -rf $(LINGUIST_PATH)

clean-shared:
	rm -rf $(RESOURCES_DIR)

clean: clean-linguist clean-shared

code-generate: $(LINGUIST_PATH)
	mkdir -p data && \
	go run internal/code-generator/main.go
	ENRY_TEST_REPO="$${PWD}/.linguist" go test  -v \
		-run Test_GeneratorTestSuite \
		./internal/code-generator/generator \
		-testify.m TestUpdateGeneratorTestSuiteGold \
		-update_gold

benchmarks: $(LINGUIST_PATH)
	go test -run=NONE -bench=. && \
	benchmarks/linguist-total.rb

benchmarks-samples: $(LINGUIST_PATH)
	go test -run=NONE -bench=. -benchtime=5us && \
	benchmarks/linguist-samples.rb

benchmarks-slow: $(LINGUIST_PATH)
	mkdir -p benchmarks/output && \
	go test -run=NONE -bench=. -slow -benchtime=100ms -timeout=100h > benchmarks/output/enry_samples.bench && \
	benchmarks/linguist-samples.rb 5 > benchmarks/output/linguist_samples.bench

linux-shared: $(LINUX_SHARED_LIB)

darwin-shared: $(DARWIN_SHARED_LIB)

$(DARWIN_SHARED_LIB):
	mkdir -p $(DARWIN_DIR) && \
	CC="o64-clang" CXX="o64-clang++" CGO_ENABLED=1 GOOS=darwin go build -buildmode=c-shared -o $(DARWIN_SHARED_LIB) $(NATIVE_LIB) && \
	mv $(DARWIN_DIR)/$(HEADER_FILE) $(RESOURCES_DIR)/$(HEADER_FILE)

$(LINUX_SHARED_LIB):
	mkdir -p $(LINUX_DIR) && \
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildmode=c-shared -o $(LINUX_SHARED_LIB) $(NATIVE_LIB) && \
	mv $(LINUX_DIR)/$(HEADER_FILE) $(RESOURCES_DIR)/$(HEADER_FILE)

.PHONY: benchmarks benchmarks-samples benchmarks-slow
