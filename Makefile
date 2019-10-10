DIST := dist
IMPORT := code.gitea.io/gitea
export GO111MODULE=off

GO ?= go
SED_INPLACE := sed -i
SHASUM ?= shasum -a 256

export PATH := $($(GO) env GOPATH)/bin:$(PATH)

ifeq ($(OS), Windows_NT)
	EXECUTABLE ?= gitea.exe
else
	EXECUTABLE ?= gitea
	UNAME_S := $(shell uname -s)
	ifeq ($(UNAME_S),Darwin)
		SED_INPLACE := sed -i ''
	endif
endif

BINDATA := modules/{options,public,templates}/bindata.go
GOFILES := $(shell find . -name "*.go" -type f ! -path "./vendor/*" ! -path "*/bindata.go")
GOFMT ?= gofmt -s

GOFLAGS := -v
EXTRA_GOFLAGS ?=

MAKE_VERSION := $(shell make -v | head -n 1)

ifneq ($(DRONE_TAG),)
	VERSION ?= $(subst v,,$(DRONE_TAG))
	GITEA_VERSION ?= $(VERSION)
else
	ifneq ($(DRONE_BRANCH),)
		VERSION ?= $(subst release/v,,$(DRONE_BRANCH))
	else
		VERSION ?= master
	endif
	GITEA_VERSION ?= $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
endif

LDFLAGS := $(LDFLAGS) -X "main.MakeVersion=$(MAKE_VERSION)" -X "main.Version=$(GITEA_VERSION)" -X "main.Tags=$(TAGS)"

PACKAGES ?= $(filter-out code.gitea.io/gitea/integrations/migration-test,$(filter-out code.gitea.io/gitea/integrations,$(shell GO111MODULE=on $(GO) list -mod=vendor ./... | grep -v /vendor/)))
SOURCES ?= $(shell find . -name "*.go" -type f)

TAGS ?=

TMPDIR := $(shell mktemp -d 2>/dev/null || mktemp -d -t 'gitea-temp')

#To update swagger use: GO111MODULE=on go get -u github.com/go-swagger/go-swagger/cmd/swagger@v0.20.1
SWAGGER := GO111MODULE=on $(GO) run -mod=vendor github.com/go-swagger/go-swagger/cmd/swagger
SWAGGER_SPEC := templates/swagger/v1_json.tmpl
SWAGGER_SPEC_S_TMPL := s|"basePath": *"/api/v1"|"basePath": "{{AppSubUrl}}/api/v1"|g
SWAGGER_SPEC_S_JSON := s|"basePath": *"{{AppSubUrl}}/api/v1"|"basePath": "/api/v1"|g
SWAGGER_NEWLINE_COMMAND := -e '$$a\'

TEST_MYSQL_HOST ?= mysql:3306
TEST_MYSQL_DBNAME ?= testgitea
TEST_MYSQL_USERNAME ?= root
TEST_MYSQL_PASSWORD ?=
TEST_MYSQL8_HOST ?= mysql8:3306
TEST_MYSQL8_DBNAME ?= testgitea
TEST_MYSQL8_USERNAME ?= root
TEST_MYSQL8_PASSWORD ?=
TEST_PGSQL_HOST ?= pgsql:5432
TEST_PGSQL_DBNAME ?= testgitea
TEST_PGSQL_USERNAME ?= postgres
TEST_PGSQL_PASSWORD ?= postgres
TEST_MSSQL_HOST ?= mssql:1433
TEST_MSSQL_DBNAME ?= gitea
TEST_MSSQL_USERNAME ?= sa
TEST_MSSQL_PASSWORD ?= MwantsaSecurePassword1

# $(call strip-suffix,filename)
strip-suffix = $(firstword $(subst ., ,$(1)))

.PHONY: all
all: build

include docker/Makefile

.PHONY: clean
clean:
	$(GO) clean -i ./...
	rm -rf $(EXECUTABLE) $(DIST) $(BINDATA) \
		integrations*.test \
		integrations/gitea-integration-pgsql/ integrations/gitea-integration-mysql/ integrations/gitea-integration-mysql8/ integrations/gitea-integration-sqlite/ \
		integrations/gitea-integration-mssql/ integrations/indexers-mysql/ integrations/indexers-mysql8/ integrations/indexers-pgsql integrations/indexers-sqlite \
		integrations/indexers-mssql integrations/mysql.ini integrations/mysql8.ini integrations/pgsql.ini integrations/mssql.ini

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GOFILES)

.PHONY: vet
vet:
	$(GO) vet $(PACKAGES)

.PHONY: generate
generate:
	GO111MODULE=on $(GO) generate -mod=vendor $(PACKAGES)

.PHONY: generate-swagger
generate-swagger:
	$(SWAGGER) generate spec -o './$(SWAGGER_SPEC)'
	$(SED_INPLACE) '$(SWAGGER_SPEC_S_TMPL)' './$(SWAGGER_SPEC)'
	$(SED_INPLACE) $(SWAGGER_NEWLINE_COMMAND) './$(SWAGGER_SPEC)'

.PHONY: swagger-check
swagger-check: generate-swagger
	@diff=$$(git diff '$(SWAGGER_SPEC)'); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make generate-swagger' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: swagger-validate
swagger-validate:
	$(SED_INPLACE) '$(SWAGGER_SPEC_S_JSON)' './$(SWAGGER_SPEC)'
	$(SWAGGER) validate './$(SWAGGER_SPEC)'
	$(SED_INPLACE) '$(SWAGGER_SPEC_S_TMPL)' './$(SWAGGER_SPEC)'

.PHONY: errcheck
errcheck:
	@hash errcheck > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/kisielk/errcheck; \
	fi
	errcheck $(PACKAGES)

.PHONY: lint
lint:
	@echo 'make lint is depricated. Use "make revive" if you want to use the old lint tool, or "make golangci-lint" to run a complete code check.'

.PHONY: revive
revive:
	@hash revive > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/mgechev/revive; \
	fi
	revive -config .revive.toml -exclude=./vendor/... ./... || exit 1

.PHONY: misspell-check
misspell-check:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -error -i unknwon,destory $(GOFILES)

.PHONY: misspell
misspell:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -w -i unknwon $(GOFILES)

.PHONY: fmt-check
fmt-check:
	# get all go files and run go fmt on them
	@diff=$$($(GOFMT) -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: test
test:
	GO111MODULE=on $(GO) test -mod=vendor -tags='sqlite sqlite_unlock_notify' $(PACKAGES)

.PHONY: coverage
coverage:
	@hash gocovmerge > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/wadey/gocovmerge; \
	fi
	gocovmerge integration.coverage.out $(shell find . -type f -name "coverage.out") > coverage.all;\

.PHONY: unit-test-coverage
unit-test-coverage:
	$(GO) test -tags='sqlite sqlite_unlock_notify' -cover -coverprofile coverage.out $(PACKAGES) && echo "\n==>\033[32m Ok\033[m\n" || exit 1

.PHONY: vendor
vendor:
	GO111MODULE=on $(GO) mod tidy && GO111MODULE=on $(GO) mod vendor

.PHONY: test-vendor
test-vendor: vendor
	@diff=$$(git diff vendor/); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make vendor' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: test-sqlite
test-sqlite: integrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test

.PHONY: test-sqlite\#%
test-sqlite\#%: integrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test -test.run $*

.PHONY: test-sqlite-migration
test-sqlite-migration:  migrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./migrations.sqlite.test

generate-ini-mysql:
	sed -e 's|{{TEST_MYSQL_HOST}}|${TEST_MYSQL_HOST}|g' \
		-e 's|{{TEST_MYSQL_DBNAME}}|${TEST_MYSQL_DBNAME}|g' \
		-e 's|{{TEST_MYSQL_USERNAME}}|${TEST_MYSQL_USERNAME}|g' \
		-e 's|{{TEST_MYSQL_PASSWORD}}|${TEST_MYSQL_PASSWORD}|g' \
			integrations/mysql.ini.tmpl > integrations/mysql.ini

.PHONY: test-mysql
test-mysql: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.mysql.test

.PHONY: test-mysql\#%
test-mysql\#%: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.mysql.test -test.run $*

.PHONY: test-mysql-migration
test-mysql-migration: migrations.mysql.test generate-ini-mysql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./migrations.mysql.test


generate-ini-mysql8:
	sed -e 's|{{TEST_MYSQL8_HOST}}|${TEST_MYSQL8_HOST}|g' \
		-e 's|{{TEST_MYSQL8_DBNAME}}|${TEST_MYSQL8_DBNAME}|g' \
		-e 's|{{TEST_MYSQL8_USERNAME}}|${TEST_MYSQL8_USERNAME}|g' \
		-e 's|{{TEST_MYSQL8_PASSWORD}}|${TEST_MYSQL8_PASSWORD}|g' \
			integrations/mysql8.ini.tmpl > integrations/mysql8.ini

.PHONY: test-mysql8
test-mysql8: integrations.mysql8.test generate-ini-mysql8
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql8.ini ./integrations.mysql8.test

.PHONY: test-mysql8\#%
test-mysql8\#%: integrations.mysql8.test generate-ini-mysql8
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql8.ini ./integrations.mysql8.test -test.run $*

.PHONY: test-mysql8-migration
test-mysql8-migration: migrations.mysql8.test generate-ini-mysql8
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql8.ini ./migrations.mysql8.test


generate-ini-pgsql:
	sed -e 's|{{TEST_PGSQL_HOST}}|${TEST_PGSQL_HOST}|g' \
		-e 's|{{TEST_PGSQL_DBNAME}}|${TEST_PGSQL_DBNAME}|g' \
		-e 's|{{TEST_PGSQL_USERNAME}}|${TEST_PGSQL_USERNAME}|g' \
		-e 's|{{TEST_PGSQL_PASSWORD}}|${TEST_PGSQL_PASSWORD}|g' \
			integrations/pgsql.ini.tmpl > integrations/pgsql.ini

.PHONY: test-pgsql
test-pgsql: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.pgsql.test

.PHONY: test-pgsql\#%
test-pgsql\#%: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.pgsql.test -test.run $*

.PHONY: test-pgsql-migration
test-pgsql-migration: migrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./migrations.pgsql.test


generate-ini-mssql:
	sed -e 's|{{TEST_MSSQL_HOST}}|${TEST_MSSQL_HOST}|g' \
		-e 's|{{TEST_MSSQL_DBNAME}}|${TEST_MSSQL_DBNAME}|g' \
		-e 's|{{TEST_MSSQL_USERNAME}}|${TEST_MSSQL_USERNAME}|g' \
		-e 's|{{TEST_MSSQL_PASSWORD}}|${TEST_MSSQL_PASSWORD}|g' \
			integrations/mssql.ini.tmpl > integrations/mssql.ini

.PHONY: test-mssql
test-mssql: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mssql.ini ./integrations.mssql.test

.PHONY: test-mssql\#%
test-mssql\#%: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mssql.ini ./integrations.mssql.test -test.run $*

.PHONY: test-mssql-migration
test-mssql-migration: migrations.mssql.test generate-ini-mssql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mssql.ini ./migrations.mssql.test


.PHONY: bench-sqlite
bench-sqlite: integrations.sqlite.test
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/sqlite.ini ./integrations.sqlite.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-mysql
bench-mysql: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.mysql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-mssql
bench-mssql: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mssql.ini ./integrations.mssql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-pgsql
bench-pgsql: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/pgsql.ini ./integrations.pgsql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .


.PHONY: integration-test-coverage
integration-test-coverage: integrations.cover.test generate-ini-mysql
	GITEA_ROOT=${CURDIR} GITEA_CONF=integrations/mysql.ini ./integrations.cover.test -test.coverprofile=integration.coverage.out

integrations.mysql.test: $(SOURCES)
	GO111MODULE=on $(GO) test -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.mysql.test

integrations.mysql8.test: $(SOURCES)
	GO111MODULE=on $(GO) test -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.mysql8.test

integrations.pgsql.test: $(SOURCES)
	GO111MODULE=on $(GO) test -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.pgsql.test

integrations.mssql.test: $(SOURCES)
	GO111MODULE=on $(GO) test -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.mssql.test

integrations.sqlite.test: $(SOURCES)
	GO111MODULE=on $(GO) test -mod=vendor -c code.gitea.io/gitea/integrations -o integrations.sqlite.test -tags 'sqlite sqlite_unlock_notify'

integrations.cover.test: $(SOURCES)
	GO111MODULE=on $(GO) test -mod=vendor -c code.gitea.io/gitea/integrations -coverpkg $(shell echo $(PACKAGES) | tr ' ' ',') -o integrations.cover.test

.PHONY: migrations.mysql.test
migrations.mysql.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations/migration-test -o migrations.mysql.test

.PHONY: migrations.mysql8.test
migrations.mysql8.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations/migration-test -o migrations.mysql8.test

.PHONY: migrations.pgsql.test
migrations.pgsql.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations/migration-test -o migrations.pgsql.test

.PHONY: migrations.mssql.test
migrations.mssql.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations/migration-test -o migrations.mssql.test

.PHONY: migrations.sqlite.test
migrations.sqlite.test: $(SOURCES)
	$(GO) test -c code.gitea.io/gitea/integrations/migration-test -o migrations.sqlite.test -tags 'sqlite sqlite_unlock_notify'

.PHONY: check
check: test

.PHONY: install
install: $(wildcard *.go)
	$(GO) install -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)'

.PHONY: build
build: $(EXECUTABLE)

$(EXECUTABLE): $(SOURCES)
	GO111MODULE=on $(GO) build -mod=vendor $(GOFLAGS) $(EXTRA_GOFLAGS) -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -o $@

.PHONY: release
release: release-dirs release-windows release-linux release-darwin release-copy release-compress release-check

.PHONY: release-dirs
release-dirs:
	mkdir -p $(DIST)/binaries $(DIST)/release

.PHONY: release-windows
release-windows:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u src.techknowlogick.com/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'windows/*' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	cp /build/* $(DIST)/binaries
endif

.PHONY: release-linux
release-linux:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u src.techknowlogick.com/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'linux/amd64,linux/386,linux/arm-5,linux/arm-6,linux/arm64,linux/mips64le,linux/mips,linux/mipsle' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	cp /build/* $(DIST)/binaries
endif

.PHONY: release-darwin
release-darwin:
	@hash xgo > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u src.techknowlogick.com/xgo; \
	fi
	xgo -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '$(LDFLAGS)' -targets 'darwin/*' -out gitea-$(VERSION) .
ifeq ($(CI),drone)
	cp /build/* $(DIST)/binaries
endif

.PHONY: release-copy
release-copy:
	cd $(DIST); for file in `find /build -type f -name "*"`; do cp $${file} ./release/; done;

.PHONY: release-check
release-check:
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "checksumming $${file}" && $(SHASUM) `echo $${file} | sed 's/^..//'` > $${file}.sha256; done;

.PHONY: release-compress
release-compress:
	@hash gxz > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/ulikunitz/xz/cmd/gxz; \
	fi
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "compressing $${file}" && gxz -k -9 $${file}; done;

npm-check:
	@hash npm > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		echo "Please install Node.js 8.x or greater with npm"; \
		exit 1; \
	fi;
	@hash npx > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		echo "Please install Node.js 8.x or greater with npm"; \
		exit 1; \
	fi;

.PHONY: npm
npm: npm-check
	npm install --no-save

.PHONY: npm-update
npm-update: npm-check
	npx updates -cu
	rm -rf node_modules package-lock.json
	npm install --package-lock

.PHONY: js
js: npm
	npx eslint public/js

.PHONY: css
css: npm
	npx stylelint public/less
	npx lessc --clean-css="--s0 -b" public/less/index.less public/css/index.css
	$(foreach file, $(filter-out public/less/themes/_base.less, $(wildcard public/less/themes/*)),npx lessc --clean-css="--s0 -b" public/less/themes/$(notdir $(file)) > public/css/theme-$(notdir $(call strip-suffix,$(file))).css;)
	npx postcss --use autoprefixer --no-map --replace public/css/*

	@diff=$$(git diff public/css/*); \
	if ([ -n "$$CI" ] && [ -n "$$diff" ]); then \
		echo "Generated files in public/css have changed, please commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: javascripts
javascripts:
	echo "'make javascripts' is deprecated, please use 'make js'"
	$(MAKE) js

.PHONY: stylesheets-check
stylesheets-check:
	echo "'make stylesheets-check' is deprecated, please use 'make css'"
	$(MAKE) css

.PHONY: generate-stylesheets
generate-stylesheets:
	echo "'make generate-stylesheets' is deprecated, please use 'make css'"
	$(MAKE) css

.PHONY: swagger-ui
swagger-ui:
	rm -Rf public/vendor/assets/swagger-ui
	git clone --depth=10 -b v3.13.4 --single-branch https://github.com/swagger-api/swagger-ui.git $(TMPDIR)/swagger-ui
	mv $(TMPDIR)/swagger-ui/dist public/vendor/assets/swagger-ui
	rm -Rf $(TMPDIR)/swagger-ui
	$(SED_INPLACE) "s;http://petstore.swagger.io/v2/swagger.json;../../../swagger.v1.json;g" public/vendor/assets/swagger-ui/index.html

.PHONY: update-translations
update-translations:
	mkdir -p ./translations
	cd ./translations && curl -L https://crowdin.com/download/project/gitea.zip > gitea.zip && unzip gitea.zip
	rm ./translations/gitea.zip
	$(SED_INPLACE) -e 's/="/=/g' -e 's/"$$//g' ./translations/*.ini
	$(SED_INPLACE) -e 's/\\"/"/g' ./translations/*.ini
	mv ./translations/*.ini ./options/locale/
	rmdir ./translations

.PHONY: generate-images
generate-images:
	mkdir -p $(TMPDIR)/images
	inkscape -f $(PWD)/assets/logo.svg -w 880 -h 880 -e $(PWD)/public/img/gitea-lg.png
	inkscape -f $(PWD)/assets/logo.svg -w 512 -h 512 -e $(PWD)/public/img/gitea-512.png
	inkscape -f $(PWD)/assets/logo.svg -w 192 -h 192 -e $(PWD)/public/img/gitea-192.png
	inkscape -f $(PWD)/assets/logo.svg -w 120 -h 120 -jC -i layer1 -e $(TMPDIR)/images/sm-1.png
	inkscape -f $(PWD)/assets/logo.svg -w 120 -h 120 -jC -i layer2 -e $(TMPDIR)/images/sm-2.png
	composite -compose atop $(TMPDIR)/images/sm-2.png $(TMPDIR)/images/sm-1.png $(PWD)/public/img/gitea-sm.png
	inkscape -f $(PWD)/assets/logo.svg -w 200 -h 200 -e $(PWD)/public/img/avatar_default.png
	inkscape -f $(PWD)/assets/logo.svg -w 180 -h 180 -e $(PWD)/public/img/favicon.png
	inkscape -f $(PWD)/assets/logo.svg -w 128 -h 128 -e $(TMPDIR)/images/128-raw.png
	inkscape -f $(PWD)/assets/logo.svg -w 64 -h 64 -e $(TMPDIR)/images/64-raw.png
	inkscape -f $(PWD)/assets/logo.svg -w 32 -h 32 -jC -i layer1 -e $(TMPDIR)/images/32-1.png
	inkscape -f $(PWD)/assets/logo.svg -w 32 -h 32 -jC -i layer2 -e $(TMPDIR)/images/32-2.png
	composite -compose atop $(TMPDIR)/images/32-2.png $(TMPDIR)/images/32-1.png $(TMPDIR)/images/32-raw.png
	inkscape -f $(PWD)/assets/logo.svg -w 16 -h 16 -jC -i layer1 -e $(TMPDIR)/images/16-raw.png
	zopflipng -m -y $(TMPDIR)/images/128-raw.png $(TMPDIR)/images/128.png
	zopflipng -m -y $(TMPDIR)/images/64-raw.png $(TMPDIR)/images/64.png
	zopflipng -m -y $(TMPDIR)/images/32-raw.png $(TMPDIR)/images/32.png
	zopflipng -m -y $(TMPDIR)/images/16-raw.png $(TMPDIR)/images/16.png
	rm -f $(TMPDIR)/images/*-*.png
	convert $(TMPDIR)/images/16.png $(TMPDIR)/images/32.png \
					$(TMPDIR)/images/64.png $(TMPDIR)/images/128.png \
					$(PWD)/public/img/favicon.ico
	rm -rf $(TMPDIR)/images
	$(foreach file, $(shell find public/img -type f -name '*.png'),zopflipng -m -y $(file) $(file);)

.PHONY: pr
pr:
	$(GO) run contrib/pr/checkout.go $(PR)

.PHONY: golangci-lint
golangci-lint:
	@hash golangci-lint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		export BINARY="golangci-lint"; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.20.0; \
	fi
	golangci-lint run
