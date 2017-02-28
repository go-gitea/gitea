DIST := dist
IMPORT := code.gitea.io/gitea

ifeq ($(OS), Windows_NT)
	EXECUTABLE := gitea.exe
else
	EXECUTABLE := gitea
endif

BINDATA := modules/{options,public,templates}/bindata.go
STYLESHEETS := $(wildcard public/less/index.less public/less/_*.less)
JAVASCRIPTS :=

LDFLAGS := -X "main.Version=$(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')" -X "main.Tags=$(TAGS)"

TARGETS ?= linux/*,darwin/*,windows/*
PACKAGES ?= $(shell go list ./... | grep -v /vendor/)
SOURCES ?= $(shell find . -name "*.go" -type f)

TAGS ?=

ifneq ($(DRONE_TAG),)
	VERSION ?= $(subst v,,$(DRONE_TAG))
else
	ifneq ($(DRONE_BRANCH),)
		VERSION ?= $(subst release/v,,$(DRONE_BRANCH))
	else
		VERSION ?= master
	endif
endif

.PHONY: all
all: build

.PHONY: clean
clean:
	go clean -i ./...
	rm -rf $(EXECUTABLE) $(DIST) $(BINDATA)

.PHONY: fmt
fmt:
	find . -name "*.go" -type f -not -path "./vendor/*" | xargs gofmt -s -w

.PHONY: vet
vet:
	go vet $(PACKAGES)

.PHONY: generate
generate:
	@hash go-bindata > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/jteeuwen/go-bindata/...; \
	fi
	go generate $(PACKAGES)

.PHONY: errcheck
errcheck:
	@hash errcheck > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/kisielk/errcheck; \
	fi
	errcheck $(PACKAGES)

.PHONY: lint
lint:
	@hash golint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/golang/lint/golint; \
	fi
	for PKG in $(PACKAGES); do golint -set_exit_status $$PKG || exit 1; done;

.PHONY: test
test:
	for PKG in $(PACKAGES); do go test -cover -coverprofile $$GOPATH/src/$$PKG/coverage.out $$PKG || exit 1; done;

.PHONY: test-mysql
test-mysql:
	@echo "Not integrated yet!"

.PHONY: test-pgsql
test-pgsql:
	@echo "Not integrated yet!"

.PHONY: check
check: test

.PHONY: install
install: $(wildcard *.go)
	go install -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)'

.PHONY: build
build: $(EXECUTABLE)

$(EXECUTABLE): $(SOURCES)
	go build -i -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -o $@

.PHONY: docker
docker:
	docker run -ti --rm -v $(CURDIR):/srv/app/src/code.gitea.io/gitea -w /srv/app/src/code.gitea.io/gitea -e TAGS="bindata $(TAGS)" webhippie/golang:edge make clean generate build
	docker build -t gitea/gitea:latest .

.PHONY: release
release: release-dirs release-build release-copy release-check

.PHONY: release-dirs
release-dirs:
	mkdir -p $(DIST)/binaries $(DIST)/release

.PHONY: release-build
release-build:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -targets '$(TARGETS)' -out $(EXECUTABLE)-$(VERSION) .
ifeq ($(CI),drone)
	mv /build/* $(DIST)/binaries
endif

.PHONY: release-copy
release-copy:
	$(foreach file,$(wildcard $(DIST)/binaries/$(EXECUTABLE)-*),cp $(file) $(DIST)/release/$(notdir $(file));)

.PHONY: release-check
release-check:
	cd $(DIST)/release; $(foreach file,$(wildcard $(DIST)/release/$(EXECUTABLE)-*),sha256sum $(notdir $(file)) > $(notdir $(file)).sha256;)

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

.PHONY: assets
assets: javascripts stylesheets
