DIST := dist
BIN := bin

EXECUTABLE := gitea
IMPORT := github.com/go-gitea/gitea

SHA := $(shell git rev-parse --short HEAD)
DATE := $(shell date -u '+%Y-%m-%d %I:%M:%S %Z')

BINDATA := $(shell find conf | sed 's/ /\\ /g')
STYLESHEETS := $(wildcard public/less/index.less public/less/_*.less)
JAVASCRIPTS :=

LDFLAGS += -X "github.com/go-gitea/gitea/modules/setting.BuildTime=$(DATE)"
LDFLAGS += -X "github.com/go-gitea/gitea/modules/setting.BuildGitHash=$(SHA)"

TARGETS ?= linux/*,darwin/*,windows/*
PACKAGES ?= $(shell go list ./... | grep -v /vendor/)

TAGS ?=

ifneq ($(TRAVIS_TAG),)
	VERSION ?= $(TRAVIS_TAG)
else
	ifneq ($(TRAVIS_BRANCH),)
		VERSION ?= $(TRAVIS_BRANCH)
	else
		VERSION ?= master
	endif
endif

.PHONY: all
all: clean test build

.PHONY: clean
clean:
	go clean -i ./...
	rm -rf $(BIN) $(DIST)

.PHONY: deps
deps:
	@which go-bindata > /dev/null; if [ $$? -ne 0 ]; then \
		go get -u github.com/jteeuwen/go-bindata/...; \
	fi

.PHONY: fmt
fmt:
	go fmt $(PACKAGES)

.PHONY: vet
vet:
	go vet $(PACKAGES)

.PHONY: lint
lint:
	@which golint > /dev/null; if [ $$? -ne 0 ]; then \
		go get -u github.com/golang/lint/golint; \
	fi
	for PKG in $(PACKAGES); do golint -set_exit_status $$PKG || exit 1; done;

.PHONY: test
test:
	for PKG in $(PACKAGES); do go test -cover -coverprofile $$GOPATH/src/$$PKG/coverage.out $$PKG || exit 1; done;

# As for GNU Coding Standard
check: test

.PHONY: install
install: $(BIN)/$(EXECUTABLE)
	cp $< $(GOPATH)/bin/

.PHONY: build
build: $(BIN)/$(EXECUTABLE)

$(BIN)/$(EXECUTABLE): $(wildcard *.go)
	go build -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -o $@

.PHONY: release
release: release-build release-copy release-check

.PHONY: release-build
release-build:
	@which xgo > /dev/null; if [ $$? -ne 0 ]; then \
		go get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(BIN) -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -targets '$(TARGETS)' -out $(EXECUTABLE)-$(VERSION) $(IMPORT)

.PHONY: release-copy
release-copy:
	mkdir -p $(DIST)/release
	$(foreach file,$(wildcard $(BIN)/$(EXECUTABLE)-*),cp $(file) $(DIST)/release/$(notdir $(file));)

.PHONY: release-check
release-check:
	cd $(DIST)/release; $(foreach file,$(wildcard $(DIST)/release/$(EXECUTABLE)-*),sha256sum $(notdir $(file)) > $(notdir $(file)).sha256;)

.PHONY: latest
latest: release-build latest-copy latest-check

.PHONY: latest-copy
latest-copy:
	mkdir -p $(DIST)/latest
	$(foreach file,$(wildcard $(BIN)/$(EXECUTABLE)-*),cp $(file) $(DIST)/latest/$(subst $(EXECUTABLE)-$(VERSION),$(EXECUTABLE)-latest,$(notdir $(file)));)

.PHONY: latest-check
latest-check:
	cd $(DIST)/latest; $(foreach file,$(wildcard $(DIST)/latest/$(EXECUTABLE)-*),sha256sum $(notdir $(file)) > $(notdir $(file)).sha256;)

.PHONY: publish
publish: release latest

.PHONY: bindata
bindata: modules/bindata/bindata.go

.IGNORE: modules/bindata/bindata.go
modules/bindata/bindata.go: $(BINDATA)
	go-bindata -o=$@ -ignore="\\.go|README.md|TRANSLATORS" -pkg=bindata conf/...
	go fmt $@

.PHONY: javascripts
javascripts: public/js/index.js

.IGNORE: public/js/index.js
public/js/index.js: $(JAVASCRIPTS)
	cat $< >| $@

.PHONY: stylesheets
stylesheets: public/css/index.css

.IGNORE: public/css/index.css
public/css/index.css: $(STYLESHEETS)
	lessc $< $@

.PHONY: generate
generate: bindata javascripts stylesheets
