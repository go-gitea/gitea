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

GOFLAGS := -i -v
EXTRA_GOFLAGS ?=

LDFLAGS := -X "main.Version=$(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')" -X "main.Tags=$(TAGS)"

PACKAGES ?= $(filter-out code.gitea.io/gitea/integrations,$(shell go list ./... | grep -v /vendor/))
SOURCES ?= $(shell find . -name "*.go" -type f)

TAGS ?=

TMPDIR := $(shell mktemp -d)

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
	@hash swagger > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/go-swagger/go-swagger/cmd/swagger; \
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
	go test $(PACKAGES)

.PHONY: test-coverage
test-coverage:
	for PKG in $(PACKAGES); do go test -cover -coverprofile $$GOPATH/src/$$PKG/coverage.out $$PKG || exit 1; done;

.PHONY: test-vendor
test-vendor:
	@hash govendor > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/kardianos/govendor; \
	fi
	govendor list +unused | tee "$(TMPDIR)/wc-gitea-unused"
	[ $$(cat "$(TMPDIR)/wc-gitea-unused" | wc -l) -eq 0 ] || echo "Warning: /!\\ Some vendor are not used /!\\"

	govendor list +outside | tee "$(TMPDIR)/wc-gitea-outside"
	[ $$(cat "$(TMPDIR)/wc-gitea-outside" | wc -l) -eq 0 ] || exit 1

	govendor status || exit 1

.PHONY: test-sqlite
test-sqlite:
	go test -c code.gitea.io/gitea/integrations -tags 'sqlite'
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.test

.PHONY: test-mysql
test-mysql: integrations.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.test

.PHONY: test-pgsql
test-pgsql: integrations.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.test

integrations.test: $(SOURCES)
	go test -c code.gitea.io/gitea/integrations

.PHONY: check
check: test

.PHONY: install
install: $(wildcard *.go)
	go install -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)'

.PHONY: build
build: $(EXECUTABLE)

$(EXECUTABLE): $(SOURCES)
	go build $(GOFLAGS) $(EXTRA_GOFLAGS) -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -o $@

.PHONY: docker
docker:
	docker run -ti --rm -v $(CURDIR):/srv/app/src/code.gitea.io/gitea -w /srv/app/src/code.gitea.io/gitea -e TAGS="bindata $(TAGS)" webhippie/golang:edge make clean generate build
	docker build -t gitea/gitea:latest .

.PHONY: release
release: release-dirs release-windows release-linux release-darwin release-copy release-check

.PHONY: release-dirs
release-dirs:
	mkdir -p $(DIST)/binaries $(DIST)/release

.PHONY: release-windows
release-windows:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'windows/*' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	mv /build/* $(DIST)/binaries
endif

.PHONY: release-linux
release-linux:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'linux/*' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	mv /build/* $(DIST)/binaries
endif

.PHONY: release-darwin
release-darwin:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/karalabe/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo $(TAGS)' -ldflags '$(LDFLAGS)' -targets 'darwin/*' -out gitea-$(VERSION) .
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

.PHONY: swagger-ui
swagger-ui:
	rm -Rf public/assets/swagger-ui
	git clone --depth=10 -b v3.0.7 --single-branch https://github.com/swagger-api/swagger-ui.git /tmp/swagger-ui
	mv /tmp/swagger-ui/dist public/assets/swagger-ui
	rm -Rf /tmp/swagger-ui
	sed -i "s;http://petstore.swagger.io/v2/swagger.json;../../swagger.v1.json;g" public/assets/swagger-ui/index.html

.PHONY: assets
assets: javascripts stylesheets
