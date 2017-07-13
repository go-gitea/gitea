DIST := dist
IMPORT := code.gitea.io/gitea

SED_INPLACE := sed -i

ifeq ($(OS), Windows_NT)
	EXECUTABLE := gitea.exe
else
	EXECUTABLE := gitea
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Darwin)
		SED_INPLACE := sed -i ''
	endif
endif

BINDATA := modules/{options,public,templates}/bindata.go
STYLESHEETS := $(wildcard public/less/index.less public/less/_*.less)
DOCKER_TAG := gitea/gitea:latest
GOFILES := $(shell find . -name "*.go" -type f ! -path "./vendor/*" ! -path "*/bindata.go")
GOFMT ?= gofmt -s

GOFLAGS := -i -v
EXTRA_GOFLAGS ?=

LDFLAGS := -X "main.Version=$(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')" -X "main.Tags=$(TAGS)"

PACKAGES ?= $(filter-out code.gitea.io/gitea/integrations,$(shell go list ./... | grep -v /vendor/))
SOURCES ?= $(shell find . -name "*.go" -type f)

TAGS ?=

TMPDIR := $(shell mktemp -d 2>/dev/null || mktemp -d -t 'gitea-temp')

ifeq ($(OS), Windows_NT)
	EXECUTABLE := gitea.exe
else
	EXECUTABLE := gitea
endif

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

required-gofmt-version:
	@go version  | grep -q '\(1.7\|1.8\)' || { echo "We require go version 1.7 or 1.8 to format code" >&2 && exit 1; }

.PHONY: fmt
fmt: required-gofmt-version
	$(GOFMT) -w $(GOFILES)

.PHONY: vet
vet:
	go vet $(PACKAGES)

.PHONY: generate
generate:
	@hash go-bindata > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/jteeuwen/go-bindata/...; \
	fi
	go generate $(PACKAGES)

.PHONY: generate-swagger
generate-swagger:
	@hash swagger > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/go-swagger/go-swagger/cmd/swagger; \
	fi
	swagger generate spec -o ./public/swagger.v1.json
	$(SED_INPLACE) "s;\".ref\": \"#/definitions/GPGKey\";\"type\": \"object\";g" ./public/swagger.v1.json
	$(SED_INPLACE) "s;^          \".ref\": \"#/definitions/Repository\";          \"type\": \"object\";g" ./public/swagger.v1.json

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

.PHONY: misspell-check
misspell-check:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -error -i unknwon $(GOFILES)

.PHONY: misspell
misspell:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		go get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -w -i unknwon $(GOFILES)

.PHONY: fmt-check
fmt-check: required-gofmt-version
	# get all go files and run go fmt on them
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: test
test: fmt-check
	go test $(PACKAGES)

.PHONY: test-coverage
test-coverage: unit-test-coverage integration-test-coverage
	for PKG in $(PACKAGES); do\
	  touch $$GOPATH/src/$$PKG/coverage.out;\
	  egrep "$$PKG[^/]*\.go" integration.coverage.out > int.coverage.out;\
	  gocovmerge $$GOPATH/src/$$PKG/coverage.out int.coverage.out > pkg.coverage.out;\
	  mv pkg.coverage.out $$GOPATH/src/$$PKG/coverage.out;\
	  rm int.coverage.out;\
	done;

.PHONY: unit-test-coverage
unit-test-coverage:
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
test-sqlite: integrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test

.PHONY: test-mysql
test-mysql: integrations.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.test

.PHONY: test-pgsql
test-pgsql: integrations.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.test


.PHONY: bench-sqlite
bench-sqlite: integrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test -test.bench .

.PHONY: bench-mysql
bench-mysql: integrations.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.test -test.bench .

.PHONY: bench-pgsql
bench-pgsql: integrations.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.test -test.bench .


.PHONY: integration-test-coverage
integration-test-coverage: integrations.cover.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.cover.test -test.coverprofile=integration.coverage.out

integrations.test: $(SOURCES)
	go test -c code.gitea.io/gitea/integrations

integrations.sqlite.test: $(SOURCES)
	go test -c code.gitea.io/gitea/integrations -o integrations.sqlite.test -tags 'sqlite'

integrations.cover.test: $(SOURCES)
	go test -c code.gitea.io/gitea/integrations -coverpkg $(shell echo $(PACKAGES) | tr ' ' ',') -o integrations.cover.test

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
	docker build -t $(DOCKER_TAG) .

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

.PHONY: stylesheets-check
stylesheets-check: stylesheets
	@diff=$$(git diff public/css/index.css); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make stylesheets' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: stylesheets
stylesheets: public/css/index.css

.IGNORE: public/css/index.css
public/css/index.css: $(STYLESHEETS)
	@which lessc > /dev/null; if [ $$? -ne 0 ]; then \
		go get -u github.com/kib357/less-go/lessc; \
	fi
	lessc -i $< -o $@

.PHONY: swagger-ui
swagger-ui:
	rm -Rf public/assets/swagger-ui
	git clone --depth=10 -b v3.0.7 --single-branch https://github.com/swagger-api/swagger-ui.git $(TMPDIR)/swagger-ui
	mv $(TMPDIR)/swagger-ui/dist public/assets/swagger-ui
	rm -Rf $(TMPDIR)/swagger-ui
	$(SED_INPLACE) "s;http://petstore.swagger.io/v2/swagger.json;../../swagger.v1.json;g" public/assets/swagger-ui/index.html

.PHONY: update-translations
update-translations:
	mkdir -p ./translations
	cd ./translations && curl -L https://crowdin.com/download/project/gitea.zip > gitea.zip && unzip gitea.zip
	rm ./translations/gitea.zip
	$(SED_INPLACE) -e 's/="/=/g' -e 's/"$$//g' ./translations/*.ini
	$(SED_INPLACE) -e 's/\\"/"/g' ./translations/*.ini
	mv ./translations/*.ini ./options/locale/
	rmdir ./translations
