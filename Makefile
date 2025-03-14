ifeq ($(USE_REPO_TEST_DIR),1)

# This rule replaces the whole Makefile when we're trying to use /tmp repository temporary files
location = $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST))
self := $(location)

%:
	@tmpdir=`mktemp --tmpdir -d` ; \
	echo Using temporary directory $$tmpdir for test repositories ; \
	USE_REPO_TEST_DIR= $(MAKE) -f $(self) --no-print-directory REPO_TEST_DIR=$$tmpdir/ $@ ; \
	STATUS=$$? ; rm -r "$$tmpdir" ; exit $$STATUS

else

# This is the "normal" part of the Makefile

DIST := dist
DIST_DIRS := $(DIST)/binaries $(DIST)/release
IMPORT := code.gitea.io/gitea

GO ?= go
SHASUM ?= shasum -a 256
HAS_GO := $(shell hash $(GO) > /dev/null 2>&1 && echo yes)
COMMA := ,

XGO_VERSION := go-1.24.x

AIR_PACKAGE ?= github.com/air-verse/air@v1
EDITORCONFIG_CHECKER_PACKAGE ?= github.com/editorconfig-checker/editorconfig-checker/v3/cmd/editorconfig-checker@v3.1.2
GOFUMPT_PACKAGE ?= mvdan.cc/gofumpt@v0.7.0
GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.5
GXZ_PACKAGE ?= github.com/ulikunitz/xz/cmd/gxz@v0.5.12
MISSPELL_PACKAGE ?= github.com/golangci/misspell/cmd/misspell@v0.6.0
SWAGGER_PACKAGE ?= github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0
XGO_PACKAGE ?= src.techknowlogick.com/xgo@latest
GO_LICENSES_PACKAGE ?= github.com/google/go-licenses@v1
GOVULNCHECK_PACKAGE ?= golang.org/x/vuln/cmd/govulncheck@v1
ACTIONLINT_PACKAGE ?= github.com/rhysd/actionlint/cmd/actionlint@v1
GOPLS_PACKAGE ?= golang.org/x/tools/gopls@v0.17.1

DOCKER_IMAGE ?= gitea/gitea
DOCKER_TAG ?= latest
DOCKER_REF := $(DOCKER_IMAGE):$(DOCKER_TAG)

ifeq ($(HAS_GO), yes)
	CGO_EXTRA_CFLAGS := -DSQLITE_MAX_VARIABLE_NUMBER=32766
	CGO_CFLAGS ?= $(shell $(GO) env CGO_CFLAGS) $(CGO_EXTRA_CFLAGS)
endif

ifeq ($(GOOS),windows)
	IS_WINDOWS := yes
else ifeq ($(patsubst Windows%,Windows,$(OS)),Windows)
	ifeq ($(GOOS),)
		IS_WINDOWS := yes
	endif
endif
ifeq ($(IS_WINDOWS),yes)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= gitea.exe
else
	GOFLAGS := -v
	EXECUTABLE ?= gitea
endif

ifeq ($(shell sed --version 2>/dev/null | grep -q GNU && echo gnu),gnu)
	SED_INPLACE := sed -i
else
	SED_INPLACE := sed -i ''
endif

EXTRA_GOFLAGS ?=

MAKE_VERSION := $(shell "$(MAKE)" -v | cat | head -n 1)
MAKE_EVIDENCE_DIR := .make_evidence

GOTESTFLAGS ?=
ifeq ($(RACE_ENABLED),true)
	GOFLAGS += -race
	GOTESTFLAGS += -race
endif

STORED_VERSION_FILE := VERSION
HUGO_VERSION ?= 0.111.3

GITHUB_REF_TYPE ?= branch
GITHUB_REF_NAME ?= $(shell git rev-parse --abbrev-ref HEAD)

ifneq ($(GITHUB_REF_TYPE),branch)
	VERSION ?= $(subst v,,$(GITHUB_REF_NAME))
	GITEA_VERSION ?= $(VERSION)
else
	ifneq ($(GITHUB_REF_NAME),)
		VERSION ?= $(subst release/v,,$(GITHUB_REF_NAME))-nightly
	else
		VERSION ?= main
	endif

	STORED_VERSION=$(shell cat $(STORED_VERSION_FILE) 2>/dev/null)
	ifneq ($(STORED_VERSION),)
		GITEA_VERSION ?= $(STORED_VERSION)
	else
		GITEA_VERSION ?= $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
	endif
endif

# if version = "main" then update version to "nightly"
ifeq ($(VERSION),main)
	VERSION := main-nightly
endif

LDFLAGS := $(LDFLAGS) -X "main.MakeVersion=$(MAKE_VERSION)" -X "main.Version=$(GITEA_VERSION)" -X "main.Tags=$(TAGS)"

LINUX_ARCHS ?= linux/amd64,linux/386,linux/arm-5,linux/arm-6,linux/arm64

GO_TEST_PACKAGES ?= $(filter-out $(shell $(GO) list code.gitea.io/gitea/models/migrations/...) code.gitea.io/gitea/tests/integration/migration-test code.gitea.io/gitea/tests code.gitea.io/gitea/tests/integration code.gitea.io/gitea/tests/e2e,$(shell $(GO) list ./... | grep -v /vendor/))
MIGRATE_TEST_PACKAGES ?= $(shell $(GO) list code.gitea.io/gitea/models/migrations/...)

WEBPACK_SOURCES := $(shell find web_src/js web_src/css -type f)
WEBPACK_CONFIGS := webpack.config.js tailwind.config.js
WEBPACK_DEST := public/assets/js/index.js public/assets/css/index.css
WEBPACK_DEST_ENTRIES := public/assets/js public/assets/css public/assets/fonts

BINDATA_DEST := modules/public/bindata.go modules/options/bindata.go modules/templates/bindata.go
BINDATA_HASH := $(addsuffix .hash,$(BINDATA_DEST))

GENERATED_GO_DEST := modules/charset/invisible_gen.go modules/charset/ambiguous_gen.go

SVG_DEST_DIR := public/assets/img/svg

AIR_TMP_DIR := .air

GO_LICENSE_TMP_DIR := .go-licenses
GO_LICENSE_FILE := assets/go-licenses.json

TAGS ?=
TAGS_SPLIT := $(subst $(COMMA), ,$(TAGS))
TAGS_EVIDENCE := $(MAKE_EVIDENCE_DIR)/tags

TEST_TAGS ?= $(TAGS_SPLIT) sqlite sqlite_unlock_notify

TAR_EXCLUDES := .git data indexers queues log node_modules $(EXECUTABLE) $(DIST) $(MAKE_EVIDENCE_DIR) $(AIR_TMP_DIR) $(GO_LICENSE_TMP_DIR)

GO_DIRS := build cmd models modules routers services tests
WEB_DIRS := web_src/js web_src/css

ESLINT_FILES := web_src/js tools *.js *.ts *.cjs tests/e2e
STYLELINT_FILES := web_src/css web_src/js/components/*.vue
SPELLCHECK_FILES := $(GO_DIRS) $(WEB_DIRS) templates options/locale/locale_en-US.ini .github $(filter-out CHANGELOG.md, $(wildcard *.go *.js *.md *.yml *.yaml *.toml)) $(filter-out tools/misspellings.csv, $(wildcard tools/*))
EDITORCONFIG_FILES := templates .github/workflows options/locale/locale_en-US.ini

GO_SOURCES := $(wildcard *.go)
GO_SOURCES += $(shell find $(GO_DIRS) -type f -name "*.go" ! -path modules/options/bindata.go ! -path modules/public/bindata.go ! -path modules/templates/bindata.go)
GO_SOURCES += $(GENERATED_GO_DEST)
GO_SOURCES_NO_BINDATA := $(GO_SOURCES)

ifeq ($(filter $(TAGS_SPLIT),bindata),bindata)
	GO_SOURCES += $(BINDATA_DEST)
	GENERATED_GO_DEST += $(BINDATA_DEST)
endif

# Force installation of playwright dependencies by setting this flag
ifdef DEPS_PLAYWRIGHT
	PLAYWRIGHT_FLAGS += --with-deps
endif

SWAGGER_SPEC := templates/swagger/v1_json.tmpl
SWAGGER_SPEC_INPUT := templates/swagger/v1_input.json
SWAGGER_EXCLUDE := code.gitea.io/sdk

TEST_MYSQL_HOST ?= mysql:3306
TEST_MYSQL_DBNAME ?= testgitea
TEST_MYSQL_USERNAME ?= root
TEST_MYSQL_PASSWORD ?=
TEST_PGSQL_HOST ?= pgsql:5432
TEST_PGSQL_DBNAME ?= testgitea
TEST_PGSQL_USERNAME ?= postgres
TEST_PGSQL_PASSWORD ?= postgres
TEST_PGSQL_SCHEMA ?= gtestschema
TEST_MINIO_ENDPOINT ?= minio:9000
TEST_MSSQL_HOST ?= mssql:1433
TEST_MSSQL_DBNAME ?= gitea
TEST_MSSQL_USERNAME ?= sa
TEST_MSSQL_PASSWORD ?= MwantsaSecurePassword1

.PHONY: all
all: build

.PHONY: help
help: Makefile ## print Makefile help information.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m[TARGETS] default target: build\033[0m\n\n\033[35mTargets:\033[0m\n"} /^[0-9A-Za-z._-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 }' Makefile #$(MAKEFILE_LIST)
	@printf "  \033[36m%-46s\033[0m %s\n" "test-e2e[#TestSpecificName]" "test end to end using playwright"
	@printf "  \033[36m%-46s\033[0m %s\n" "test[#TestSpecificName]" "run unit test"
	@printf "  \033[36m%-46s\033[0m %s\n" "test-sqlite[#TestSpecificName]" "run integration test for sqlite"

.PHONY: go-check
go-check:
	$(eval MIN_GO_VERSION_STR := $(shell grep -Eo '^go\s+[0-9]+\.[0-9]+' go.mod | cut -d' ' -f2))
	$(eval MIN_GO_VERSION := $(shell printf "%03d%03d" $(shell echo '$(MIN_GO_VERSION_STR)' | tr '.' ' ')))
	$(eval GO_VERSION := $(shell printf "%03d%03d" $(shell $(GO) version | grep -Eo '[0-9]+\.[0-9]+' | tr '.' ' ');))
	@if [ "$(GO_VERSION)" -lt "$(MIN_GO_VERSION)" ]; then \
		echo "Gitea requires Go $(MIN_GO_VERSION_STR) or greater to build. You can get it at https://go.dev/dl/"; \
		exit 1; \
	fi

.PHONY: git-check
git-check:
	@if git lfs >/dev/null 2>&1 ; then : ; else \
		echo "Gitea requires git with lfs support to run tests." ; \
		exit 1; \
	fi

.PHONY: node-check
node-check:
	$(eval MIN_NODE_VERSION_STR := $(shell grep -Eo '"node":.*[0-9.]+"' package.json | sed -n 's/.*[^0-9.]\([0-9.]*\)"/\1/p'))
	$(eval MIN_NODE_VERSION := $(shell printf "%03d%03d%03d" $(shell echo '$(MIN_NODE_VERSION_STR)' | tr '.' ' ')))
	$(eval NODE_VERSION := $(shell printf "%03d%03d%03d" $(shell node -v | cut -c2- | tr '.' ' ');))
	$(eval NPM_MISSING := $(shell hash npm > /dev/null 2>&1 || echo 1))
	@if [ "$(NODE_VERSION)" -lt "$(MIN_NODE_VERSION)" -o "$(NPM_MISSING)" = "1" ]; then \
		echo "Gitea requires Node.js $(MIN_NODE_VERSION_STR) or greater and npm to build. You can get it at https://nodejs.org/en/download/"; \
		exit 1; \
	fi

.PHONY: clean-all
clean-all: clean ## delete backend, frontend and integration files
	rm -rf $(WEBPACK_DEST_ENTRIES) node_modules

.PHONY: clean
clean: ## delete backend and integration files
	rm -rf $(EXECUTABLE) $(DIST) $(BINDATA_DEST) $(BINDATA_HASH) \
		integrations*.test \
		e2e*.test \
		tests/integration/gitea-integration-* \
		tests/integration/indexers-* \
		tests/mysql.ini tests/pgsql.ini tests/mssql.ini man/ \
		tests/e2e/gitea-e2e-*/ \
		tests/e2e/indexers-*/ \
		tests/e2e/reports/ tests/e2e/test-artifacts/ tests/e2e/test-snapshots/

.PHONY: fmt
fmt: ## format the Go code
	@GOFUMPT_PACKAGE=$(GOFUMPT_PACKAGE) $(GO) run build/code-batch-process.go gitea-fmt -w '{file-list}'
	$(eval TEMPLATES := $(shell find templates -type f -name '*.tmpl'))
	@# strip whitespace after '{{' or '(' and before '}}' or ')' unless there is only
	@# whitespace before it
	@$(SED_INPLACE) \
		-e 's/{{[ 	]\{1,\}/{{/g' -e '/^[ 	]\{1,\}}}/! s/[ 	]\{1,\}}}/}}/g' \
	  -e 's/([ 	]\{1,\}/(/g' -e '/^[ 	]\{1,\})/! s/[ 	]\{1,\})/)/g' \
	  $(TEMPLATES)

.PHONY: fmt-check
fmt-check: fmt
	@diff=$$(git diff --color=always $(GO_SOURCES) templates $(WEB_DIRS)); \
	if [ -n "$$diff" ]; then \
	  echo "Please run 'make fmt' and commit the result:"; \
	  printf "%s" "$${diff}"; \
	  exit 1; \
	fi

.PHONY: $(TAGS_EVIDENCE)
$(TAGS_EVIDENCE):
	@mkdir -p $(MAKE_EVIDENCE_DIR)
	@echo "$(TAGS)" > $(TAGS_EVIDENCE)

ifneq "$(TAGS)" "$(shell cat $(TAGS_EVIDENCE) 2>/dev/null)"
TAGS_PREREQ := $(TAGS_EVIDENCE)
endif

.PHONY: generate-swagger
generate-swagger: $(SWAGGER_SPEC) ## generate the swagger spec from code comments

$(SWAGGER_SPEC): $(GO_SOURCES_NO_BINDATA) $(SWAGGER_SPEC_INPUT)
	$(GO) run $(SWAGGER_PACKAGE) generate spec --exclude "$(SWAGGER_EXCLUDE)" --input "$(SWAGGER_SPEC_INPUT)" --output './$(SWAGGER_SPEC)'

.PHONY: swagger-check
swagger-check: generate-swagger
	@diff=$$(git diff --color=always '$(SWAGGER_SPEC)'); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make generate-swagger' and commit the result:"; \
		printf "%s" "$${diff}"; \
		exit 1; \
	fi

.PHONY: swagger-validate
swagger-validate: ## check if the swagger spec is valid
	@# swagger "validate" requires that the "basePath" must start with a slash, but we are using Golang template "{{...}}"
	@$(SED_INPLACE) -E -e 's|"basePath":( *)"(.*)"|"basePath":\1"/\2"|g' './$(SWAGGER_SPEC)' # add a prefix slash to basePath
	@# FIXME: there are some warnings
	$(GO) run $(SWAGGER_PACKAGE) validate './$(SWAGGER_SPEC)'
	@$(SED_INPLACE) -E -e 's|"basePath":( *)"/(.*)"|"basePath":\1"\2"|g' './$(SWAGGER_SPEC)' # remove the prefix slash from basePath

.PHONY: checks
checks: checks-frontend checks-backend ## run various consistency checks

.PHONY: checks-frontend
checks-frontend: lockfile-check svg-check ## check frontend files

.PHONY: checks-backend
checks-backend: tidy-check swagger-check fmt-check swagger-validate security-check ## check backend files

.PHONY: lint
lint: lint-frontend lint-backend lint-spell ## lint everything

.PHONY: lint-fix
lint-fix: lint-frontend-fix lint-backend-fix lint-spell-fix ## lint everything and fix issues

.PHONY: lint-frontend
lint-frontend: lint-js lint-css ## lint frontend files

.PHONY: lint-frontend-fix
lint-frontend-fix: lint-js-fix lint-css-fix ## lint frontend files and fix issues

.PHONY: lint-backend
lint-backend: lint-go lint-go-gitea-vet lint-go-gopls lint-editorconfig ## lint backend files

.PHONY: lint-backend-fix
lint-backend-fix: lint-go-fix lint-go-gitea-vet lint-editorconfig ## lint backend files and fix issues

.PHONY: lint-js
lint-js: node_modules ## lint js files
	npx eslint --color --max-warnings=0 --ext js,ts,vue $(ESLINT_FILES)
	npx vue-tsc

.PHONY: lint-js-fix
lint-js-fix: node_modules ## lint js files and fix issues
	npx eslint --color --max-warnings=0 --ext js,ts,vue $(ESLINT_FILES) --fix
	npx vue-tsc

.PHONY: lint-css
lint-css: node_modules ## lint css files
	npx stylelint --color --max-warnings=0 $(STYLELINT_FILES)

.PHONY: lint-css-fix
lint-css-fix: node_modules ## lint css files and fix issues
	npx stylelint --color --max-warnings=0 $(STYLELINT_FILES) --fix

.PHONY: lint-swagger
lint-swagger: node_modules ## lint swagger files
	npx spectral lint -q -F hint $(SWAGGER_SPEC)

.PHONY: lint-md
lint-md: node_modules ## lint markdown files
	npx markdownlint *.md

.PHONY: lint-spell
lint-spell: ## lint spelling
	@go run $(MISSPELL_PACKAGE) -dict tools/misspellings.csv -error $(SPELLCHECK_FILES)

.PHONY: lint-spell-fix
lint-spell-fix: ## lint spelling and fix issues
	@go run $(MISSPELL_PACKAGE) -dict tools/misspellings.csv -w $(SPELLCHECK_FILES)

.PHONY: lint-go
lint-go: ## lint go files
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run

.PHONY: lint-go-fix
lint-go-fix: ## lint go files and fix issues
	$(GO) run $(GOLANGCI_LINT_PACKAGE) run --fix

# workaround step for the lint-go-windows CI task because 'go run' can not
# have distinct GOOS/GOARCH for its build and run steps
.PHONY: lint-go-windows
lint-go-windows:
	@GOOS= GOARCH= $(GO) install $(GOLANGCI_LINT_PACKAGE)
	golangci-lint run

.PHONY: lint-go-gitea-vet
lint-go-gitea-vet: ## lint go files with gitea-vet
	@echo "Running gitea-vet..."
	@GOOS= GOARCH= $(GO) build code.gitea.io/gitea-vet
	@$(GO) vet -vettool=gitea-vet ./...

.PHONY: lint-go-gopls
lint-go-gopls: ## lint go files with gopls
	@echo "Running gopls check..."
	@GO=$(GO) GOPLS_PACKAGE=$(GOPLS_PACKAGE) tools/lint-go-gopls.sh $(GO_SOURCES_NO_BINDATA)

.PHONY: lint-editorconfig
lint-editorconfig:
	@echo "Running editorconfig check..."
	@$(GO) run $(EDITORCONFIG_CHECKER_PACKAGE) $(EDITORCONFIG_FILES)

.PHONY: lint-actions
lint-actions: ## lint action workflow files
	$(GO) run $(ACTIONLINT_PACKAGE)

.PHONY: lint-templates
lint-templates: .venv node_modules ## lint template files
	@node tools/lint-templates-svg.js
	@poetry run djlint $(shell find templates -type f -iname '*.tmpl')

.PHONY: lint-yaml
lint-yaml: .venv ## lint yaml files
	@poetry run yamllint -s .

.PHONY: watch
watch: ## watch everything and continuously rebuild
	@bash tools/watch.sh

.PHONY: watch-frontend
watch-frontend: node-check node_modules ## watch frontend files and continuously rebuild
	@rm -rf $(WEBPACK_DEST_ENTRIES)
	NODE_ENV=development npx webpack --watch --progress

.PHONY: watch-backend
watch-backend: go-check ## watch backend files and continuously rebuild
	GITEA_RUN_MODE=dev $(GO) run $(AIR_PACKAGE) -c .air.toml

.PHONY: test
test: test-frontend test-backend ## test everything

.PHONY: test-backend
test-backend: ## test frontend files
	@echo "Running go test with $(GOTESTFLAGS) -tags '$(TEST_TAGS)'..."
	@$(GO) test $(GOTESTFLAGS) -tags='$(TEST_TAGS)' $(GO_TEST_PACKAGES)

.PHONY: test-frontend
test-frontend: node_modules ## test backend files
	npx vitest

.PHONY: test-check
test-check:
	@echo "Running test-check...";
	@diff=$$(git status -s); \
	if [ -n "$$diff" ]; then \
		echo "make test-backend has changed files in the source tree:"; \
		printf "%s" "$${diff}"; \
		echo "You should change the tests to create these files in a temporary directory."; \
		echo "Do not simply add these files to .gitignore"; \
		exit 1; \
	fi

.PHONY: test\#%
test\#%:
	@echo "Running go test with -tags '$(TEST_TAGS)'..."
	@$(GO) test $(GOTESTFLAGS) -tags='$(TEST_TAGS)' -run $(subst .,/,$*) $(GO_TEST_PACKAGES)

.PHONY: coverage
coverage:
	grep '^\(mode: .*\)\|\(.*:[0-9]\+\.[0-9]\+,[0-9]\+\.[0-9]\+ [0-9]\+ [0-9]\+\)$$' coverage.out > coverage-bodged.out
	grep '^\(mode: .*\)\|\(.*:[0-9]\+\.[0-9]\+,[0-9]\+\.[0-9]\+ [0-9]\+ [0-9]\+\)$$' integration.coverage.out > integration.coverage-bodged.out
	$(GO) run build/gocovmerge.go integration.coverage-bodged.out coverage-bodged.out > coverage.all

.PHONY: unit-test-coverage
unit-test-coverage:
	@echo "Running unit-test-coverage $(GOTESTFLAGS) -tags '$(TEST_TAGS)'..."
	@$(GO) test $(GOTESTFLAGS) -timeout=20m -tags='$(TEST_TAGS)' -cover -coverprofile coverage.out $(GO_TEST_PACKAGES) && echo "\n==>\033[32m Ok\033[m\n" || exit 1

.PHONY: tidy
tidy: ## run go mod tidy
	$(eval MIN_GO_VERSION := $(shell grep -Eo '^go\s+[0-9]+\.[0-9.]+' go.mod | cut -d' ' -f2))
	$(GO) mod tidy -compat=$(MIN_GO_VERSION)
	@$(MAKE) --no-print-directory $(GO_LICENSE_FILE)

vendor: go.mod go.sum
	$(GO) mod vendor
	@touch vendor

.PHONY: tidy-check
tidy-check: tidy
	@diff=$$(git diff --color=always go.mod go.sum $(GO_LICENSE_FILE)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make tidy' and commit the result:"; \
		printf "%s" "$${diff}"; \
		exit 1; \
	fi

.PHONY: go-licenses
go-licenses: $(GO_LICENSE_FILE) ## regenerate go licenses

$(GO_LICENSE_FILE): go.mod go.sum
	@rm -rf $(GO_LICENSE_FILE)
	$(GO) install $(GO_LICENSES_PACKAGE)
	-GOOS=linux CGO_ENABLED=1 go-licenses save . --force --save_path=$(GO_LICENSE_TMP_DIR) 2>/dev/null
	$(GO) run build/generate-go-licenses.go $(GO_LICENSE_TMP_DIR) $(GO_LICENSE_FILE)
	@rm -rf $(GO_LICENSE_TMP_DIR)

generate-ini-sqlite:
	sed -e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
		-e 's|{{TEST_LOGGER}}|$(or $(TEST_LOGGER),test$(COMMA)file)|g' \
		-e 's|{{TEST_TYPE}}|$(or $(TEST_TYPE),integration)|g' \
			tests/sqlite.ini.tmpl > tests/sqlite.ini

.PHONY: test-sqlite
test-sqlite: integrations.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini ./integrations.sqlite.test

.PHONY: test-sqlite\#%
test-sqlite\#%: integrations.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini ./integrations.sqlite.test -test.run $(subst .,/,$*)

.PHONY: test-sqlite-migration
test-sqlite-migration:  migrations.sqlite.test migrations.individual.sqlite.test

generate-ini-mysql:
	sed -e 's|{{TEST_MYSQL_HOST}}|${TEST_MYSQL_HOST}|g' \
		-e 's|{{TEST_MYSQL_DBNAME}}|${TEST_MYSQL_DBNAME}|g' \
		-e 's|{{TEST_MYSQL_USERNAME}}|${TEST_MYSQL_USERNAME}|g' \
		-e 's|{{TEST_MYSQL_PASSWORD}}|${TEST_MYSQL_PASSWORD}|g' \
		-e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
		-e 's|{{TEST_LOGGER}}|$(or $(TEST_LOGGER),test$(COMMA)file)|g' \
		-e 's|{{TEST_TYPE}}|$(or $(TEST_TYPE),integration)|g' \
			tests/mysql.ini.tmpl > tests/mysql.ini

.PHONY: test-mysql
test-mysql: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini ./integrations.mysql.test

.PHONY: test-mysql\#%
test-mysql\#%: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini ./integrations.mysql.test -test.run $(subst .,/,$*)

.PHONY: test-mysql-migration
test-mysql-migration: migrations.mysql.test migrations.individual.mysql.test

generate-ini-pgsql:
	sed -e 's|{{TEST_PGSQL_HOST}}|${TEST_PGSQL_HOST}|g' \
		-e 's|{{TEST_PGSQL_DBNAME}}|${TEST_PGSQL_DBNAME}|g' \
		-e 's|{{TEST_PGSQL_USERNAME}}|${TEST_PGSQL_USERNAME}|g' \
		-e 's|{{TEST_PGSQL_PASSWORD}}|${TEST_PGSQL_PASSWORD}|g' \
		-e 's|{{TEST_PGSQL_SCHEMA}}|${TEST_PGSQL_SCHEMA}|g' \
		-e 's|{{TEST_MINIO_ENDPOINT}}|${TEST_MINIO_ENDPOINT}|g' \
		-e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
		-e 's|{{TEST_LOGGER}}|$(or $(TEST_LOGGER),test$(COMMA)file)|g' \
		-e 's|{{TEST_TYPE}}|$(or $(TEST_TYPE),integration)|g' \
			tests/pgsql.ini.tmpl > tests/pgsql.ini

.PHONY: test-pgsql
test-pgsql: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini ./integrations.pgsql.test

.PHONY: test-pgsql\#%
test-pgsql\#%: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini ./integrations.pgsql.test -test.run $(subst .,/,$*)

.PHONY: test-pgsql-migration
test-pgsql-migration: migrations.pgsql.test migrations.individual.pgsql.test

generate-ini-mssql:
	sed -e 's|{{TEST_MSSQL_HOST}}|${TEST_MSSQL_HOST}|g' \
		-e 's|{{TEST_MSSQL_DBNAME}}|${TEST_MSSQL_DBNAME}|g' \
		-e 's|{{TEST_MSSQL_USERNAME}}|${TEST_MSSQL_USERNAME}|g' \
		-e 's|{{TEST_MSSQL_PASSWORD}}|${TEST_MSSQL_PASSWORD}|g' \
		-e 's|{{REPO_TEST_DIR}}|${REPO_TEST_DIR}|g' \
		-e 's|{{TEST_LOGGER}}|$(or $(TEST_LOGGER),test$(COMMA)file)|g' \
		-e 's|{{TEST_TYPE}}|$(or $(TEST_TYPE),integration)|g' \
			tests/mssql.ini.tmpl > tests/mssql.ini

.PHONY: test-mssql
test-mssql: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini ./integrations.mssql.test

.PHONY: test-mssql\#%
test-mssql\#%: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini ./integrations.mssql.test -test.run $(subst .,/,$*)

.PHONY: test-mssql-migration
test-mssql-migration: migrations.mssql.test migrations.individual.mssql.test

.PHONY: playwright
playwright: deps-frontend
	npx playwright install $(PLAYWRIGHT_FLAGS)

.PHONY: test-e2e%
test-e2e%: TEST_TYPE ?= e2e
	# Clear display env variable. Otherwise, chromium tests can fail.
	DISPLAY=

.PHONY: test-e2e
test-e2e: test-e2e-sqlite

.PHONY: test-e2e-sqlite
test-e2e-sqlite: playwright e2e.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini ./e2e.sqlite.test

.PHONY: test-e2e-sqlite\#%
test-e2e-sqlite\#%: playwright e2e.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini ./e2e.sqlite.test -test.run TestE2e/$*

.PHONY: test-e2e-mysql
test-e2e-mysql: playwright e2e.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini ./e2e.mysql.test

.PHONY: test-e2e-mysql\#%
test-e2e-mysql\#%: playwright e2e.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini ./e2e.mysql.test -test.run TestE2e/$*

.PHONY: test-e2e-pgsql
test-e2e-pgsql: playwright e2e.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini ./e2e.pgsql.test

.PHONY: test-e2e-pgsql\#%
test-e2e-pgsql\#%: playwright e2e.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini ./e2e.pgsql.test -test.run TestE2e/$*

.PHONY: test-e2e-mssql
test-e2e-mssql: playwright e2e.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini ./e2e.mssql.test

.PHONY: test-e2e-mssql\#%
test-e2e-mssql\#%: playwright e2e.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini ./e2e.mssql.test -test.run TestE2e/$*

.PHONY: bench-sqlite
bench-sqlite: integrations.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini ./integrations.sqlite.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-mysql
bench-mysql: integrations.mysql.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini ./integrations.mysql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-mssql
bench-mssql: integrations.mssql.test generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini ./integrations.mssql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: bench-pgsql
bench-pgsql: integrations.pgsql.test generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini ./integrations.pgsql.test -test.cpuprofile=cpu.out -test.run DontRunTests -test.bench .

.PHONY: integration-test-coverage
integration-test-coverage: integrations.cover.test generate-ini-mysql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini ./integrations.cover.test -test.coverprofile=integration.coverage.out

.PHONY: integration-test-coverage-sqlite
integration-test-coverage-sqlite: integrations.cover.sqlite.test generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini ./integrations.cover.sqlite.test -test.coverprofile=integration.coverage.out

integrations.mysql.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration -o integrations.mysql.test

integrations.pgsql.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration -o integrations.pgsql.test

integrations.mssql.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration -o integrations.mssql.test

integrations.sqlite.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration -o integrations.sqlite.test -tags '$(TEST_TAGS)'

integrations.cover.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration -coverpkg $(shell echo $(GO_TEST_PACKAGES) | tr ' ' ',') -o integrations.cover.test

integrations.cover.sqlite.test: git-check $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration -coverpkg $(shell echo $(GO_TEST_PACKAGES) | tr ' ' ',') -o integrations.cover.sqlite.test -tags '$(TEST_TAGS)'

.PHONY: migrations.mysql.test
migrations.mysql.test: $(GO_SOURCES) generate-ini-mysql
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration/migration-test -o migrations.mysql.test
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini ./migrations.mysql.test

.PHONY: migrations.pgsql.test
migrations.pgsql.test: $(GO_SOURCES) generate-ini-pgsql
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration/migration-test -o migrations.pgsql.test
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini ./migrations.pgsql.test

.PHONY: migrations.mssql.test
migrations.mssql.test: $(GO_SOURCES) generate-ini-mssql
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration/migration-test -o migrations.mssql.test
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini ./migrations.mssql.test

.PHONY: migrations.sqlite.test
migrations.sqlite.test: $(GO_SOURCES) generate-ini-sqlite
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/integration/migration-test -o migrations.sqlite.test -tags '$(TEST_TAGS)'
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini ./migrations.sqlite.test

.PHONY: migrations.individual.mysql.test
migrations.individual.mysql.test: $(GO_SOURCES)
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mysql.ini $(GO) test $(GOTESTFLAGS) -tags='$(TEST_TAGS)' -p 1 $(MIGRATE_TEST_PACKAGES)

.PHONY: migrations.individual.sqlite.test\#%
migrations.individual.sqlite.test\#%: $(GO_SOURCES) generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini $(GO) test $(GOTESTFLAGS) -tags '$(TEST_TAGS)' code.gitea.io/gitea/models/migrations/$*

.PHONY: migrations.individual.pgsql.test
migrations.individual.pgsql.test: $(GO_SOURCES)
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini $(GO) test $(GOTESTFLAGS) -tags='$(TEST_TAGS)' -p 1 $(MIGRATE_TEST_PACKAGES)

.PHONY: migrations.individual.pgsql.test\#%
migrations.individual.pgsql.test\#%: $(GO_SOURCES) generate-ini-pgsql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/pgsql.ini $(GO) test $(GOTESTFLAGS) -tags '$(TEST_TAGS)' code.gitea.io/gitea/models/migrations/$*

.PHONY: migrations.individual.mssql.test
migrations.individual.mssql.test: $(GO_SOURCES) generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini $(GO) test $(GOTESTFLAGS) -tags='$(TEST_TAGS)' -p 1 $(MIGRATE_TEST_PACKAGES)

.PHONY: migrations.individual.mssql.test\#%
migrations.individual.mssql.test\#%: $(GO_SOURCES) generate-ini-mssql
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/mssql.ini $(GO) test $(GOTESTFLAGS) -tags '$(TEST_TAGS)' code.gitea.io/gitea/models/migrations/$*

.PHONY: migrations.individual.sqlite.test
migrations.individual.sqlite.test: $(GO_SOURCES) generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini $(GO) test $(GOTESTFLAGS) -tags='$(TEST_TAGS)' -p 1 $(MIGRATE_TEST_PACKAGES)

.PHONY: migrations.individual.sqlite.test\#%
migrations.individual.sqlite.test\#%: $(GO_SOURCES) generate-ini-sqlite
	GITEA_ROOT="$(CURDIR)" GITEA_CONF=tests/sqlite.ini $(GO) test $(GOTESTFLAGS) -tags '$(TEST_TAGS)' code.gitea.io/gitea/models/migrations/$*

e2e.mysql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/e2e -o e2e.mysql.test

e2e.pgsql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/e2e -o e2e.pgsql.test

e2e.mssql.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/e2e -o e2e.mssql.test

e2e.sqlite.test: $(GO_SOURCES)
	$(GO) test $(GOTESTFLAGS) -c code.gitea.io/gitea/tests/e2e -o e2e.sqlite.test -tags '$(TEST_TAGS)'

.PHONY: check
check: test

.PHONY: install $(TAGS_PREREQ)
install: $(wildcard *.go)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) install -v -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)'

.PHONY: build
build: frontend backend ## build everything

.PHONY: frontend
frontend: $(WEBPACK_DEST) ## build frontend files

.PHONY: backend
backend: go-check generate-backend $(EXECUTABLE) ## build backend files

# We generate the backend before the frontend in case we in future we want to generate things in the frontend from generated files in backend
.PHONY: generate
generate: generate-backend ## run "go generate"

.PHONY: generate-backend
generate-backend: $(TAGS_PREREQ) generate-go

.PHONY: generate-go
generate-go: $(TAGS_PREREQ)
	@echo "Running go generate..."
	@CC= GOOS= GOARCH= CGO_ENABLED=0 $(GO) generate -tags '$(TAGS)' ./...

.PHONY: security-check
security-check:
	go run $(GOVULNCHECK_PACKAGE) ./...

$(EXECUTABLE): $(GO_SOURCES) $(TAGS_PREREQ)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) build $(GOFLAGS) $(EXTRA_GOFLAGS) -tags '$(TAGS)' -ldflags '-s -w $(LDFLAGS)' -o $@

.PHONY: release
release: frontend generate release-windows release-linux release-darwin release-freebsd release-copy release-compress vendor release-sources release-check

$(DIST_DIRS):
	mkdir -p $(DIST_DIRS)

.PHONY: release-windows
release-windows: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -buildmode exe -dest $(DIST)/binaries -tags 'osusergo $(TAGS)' -ldflags '-s -w -linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'windows/*' -out gitea-$(VERSION) .
ifeq (,$(findstring gogit,$(TAGS)))
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -buildmode exe -dest $(DIST)/binaries -tags 'osusergo gogit $(TAGS)' -ldflags '-s -w -linkmode external -extldflags "-static" $(LDFLAGS)' -targets 'windows/*' -out gitea-$(VERSION)-gogit .
endif

.PHONY: release-linux
release-linux: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-s -w -linkmode external -extldflags "-static" $(LDFLAGS)' -targets '$(LINUX_ARCHS)' -out gitea-$(VERSION) .

.PHONY: release-darwin
release-darwin: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-s -w $(LDFLAGS)' -targets 'darwin-10.12/amd64,darwin-10.12/arm64' -out gitea-$(VERSION) .

.PHONY: release-freebsd
release-freebsd: | $(DIST_DIRS)
	CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) run $(XGO_PACKAGE) -go $(XGO_VERSION) -dest $(DIST)/binaries -tags 'netgo osusergo $(TAGS)' -ldflags '-s -w $(LDFLAGS)' -targets 'freebsd/amd64' -out gitea-$(VERSION) .

.PHONY: release-copy
release-copy: | $(DIST_DIRS)
	cd $(DIST); for file in `find . -type f -name "*"`; do cp $${file} ./release/; done;

.PHONY: release-check
release-check: | $(DIST_DIRS)
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "checksumming $${file}" && $(SHASUM) `echo $${file} | sed 's/^..//'` > $${file}.sha256; done;

.PHONY: release-compress
release-compress: | $(DIST_DIRS)
	cd $(DIST)/release/; for file in `find . -type f -name "*"`; do echo "compressing $${file}" && $(GO) run $(GXZ_PACKAGE) -k -9 $${file}; done;

.PHONY: release-sources
release-sources: | $(DIST_DIRS)
	echo $(VERSION) > $(STORED_VERSION_FILE)
# bsdtar needs a ^ to prevent matching subdirectories
	$(eval EXCL := --exclude=$(shell tar --help | grep -q bsdtar && echo "^")./)
# use transform to a add a release-folder prefix; in bsdtar the transform parameter equivalent is -s
	$(eval TRANSFORM := $(shell tar --help | grep -q bsdtar && echo "-s '/^./gitea-src-$(VERSION)/'" || echo "--transform 's|^./|gitea-src-$(VERSION)/|'"))
	tar $(addprefix $(EXCL),$(TAR_EXCLUDES)) $(TRANSFORM) -czf $(DIST)/release/gitea-src-$(VERSION).tar.gz .
	rm -f $(STORED_VERSION_FILE)

.PHONY: deps
deps: deps-frontend deps-backend deps-tools deps-py ## install dependencies

.PHONY: deps-py
deps-py: .venv ## install python dependencies

.PHONY: deps-frontend
deps-frontend: node_modules ## install frontend dependencies

.PHONY: deps-backend
deps-backend: ## install backend dependencies
	$(GO) mod download

.PHONY: deps-tools
deps-tools: ## install tool dependencies
	$(GO) install $(AIR_PACKAGE) & \
	$(GO) install $(EDITORCONFIG_CHECKER_PACKAGE) & \
	$(GO) install $(GOFUMPT_PACKAGE) & \
	$(GO) install $(GOLANGCI_LINT_PACKAGE) & \
	$(GO) install $(GXZ_PACKAGE) & \
	$(GO) install $(MISSPELL_PACKAGE) & \
	$(GO) install $(SWAGGER_PACKAGE) & \
	$(GO) install $(XGO_PACKAGE) & \
	$(GO) install $(GO_LICENSES_PACKAGE) & \
	$(GO) install $(GOVULNCHECK_PACKAGE) & \
	$(GO) install $(ACTIONLINT_PACKAGE) & \
	$(GO) install $(GOPLS_PACKAGE) & \
	wait

node_modules: package-lock.json
	npm install --no-save
	@touch node_modules

.venv: poetry.lock
	poetry install
	@touch .venv

.PHONY: update
update: update-js update-py ## update js and py dependencies

.PHONY: update-js
update-js: node-check | node_modules ## update js dependencies
	npx updates -u -f package.json
	rm -rf node_modules package-lock.json
	npm install --package-lock
	npx nolyfill install
	npm install --package-lock
	@touch node_modules

.PHONY: update-py
update-py: node-check | node_modules ## update py dependencies
	npx updates -u -f pyproject.toml
	rm -rf .venv poetry.lock
	poetry install
	@touch .venv

.PHONY: webpack
webpack: $(WEBPACK_DEST) ## build webpack files

$(WEBPACK_DEST): $(WEBPACK_SOURCES) $(WEBPACK_CONFIGS) package-lock.json
	@$(MAKE) -s node-check node_modules
	@rm -rf $(WEBPACK_DEST_ENTRIES)
	@echo "Running webpack..."
	@BROWSERSLIST_IGNORE_OLD_DATA=true npx webpack
	@touch $(WEBPACK_DEST)

.PHONY: svg
svg: node-check | node_modules ## build svg files
	rm -rf $(SVG_DEST_DIR)
	node tools/generate-svg.js

.PHONY: svg-check
svg-check: svg
	@git add $(SVG_DEST_DIR)
	@diff=$$(git diff --color=always --cached $(SVG_DEST_DIR)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make svg' and 'git add $(SVG_DEST_DIR)' and commit the result:"; \
		printf "%s" "$${diff}"; \
		exit 1; \
	fi

.PHONY: lockfile-check
lockfile-check:
	npm install --package-lock-only
	@diff=$$(git diff --color=always package-lock.json); \
	if [ -n "$$diff" ]; then \
		echo "package-lock.json is inconsistent with package.json"; \
		echo "Please run 'npm install --package-lock-only' and commit the result:"; \
		printf "%s" "$${diff}"; \
		exit 1; \
	fi

.PHONY: update-translations
update-translations:
	mkdir -p ./translations
	cd ./translations && curl -L https://crowdin.com/download/project/gitea.zip > gitea.zip && unzip gitea.zip
	rm ./translations/gitea.zip
	$(SED_INPLACE) -e 's/="/=/g' -e 's/"$$//g' ./translations/*.ini
	$(SED_INPLACE) -e 's/\\"/"/g' ./translations/*.ini
	mv ./translations/*.ini ./options/locale/
	rmdir ./translations

.PHONY: generate-gitignore
generate-gitignore: ## update gitignore files
	$(GO) run build/generate-gitignores.go

.PHONY: generate-images
generate-images: | node_modules
	npm install --no-save fabric@6 imagemin-zopfli@7
	node tools/generate-images.js $(TAGS)

.PHONY: generate-manpage
generate-manpage: ## generate manpage
	@[ -f gitea ] || make backend
	@mkdir -p man/man1/ man/man5
	@./gitea docs --man > man/man1/gitea.1
	@gzip -9 man/man1/gitea.1 && echo man/man1/gitea.1.gz created
	@#TODO A small script that formats config-cheat-sheet.en-us.md nicely for use as a config man page

.PHONY: docker
docker:
	docker build --disable-content-trust=false -t $(DOCKER_REF) .
# support also build args docker build --build-arg GITEA_VERSION=v1.2.3 --build-arg TAGS="bindata sqlite sqlite_unlock_notify"  .

# This endif closes the if at the top of the file
endif

# Disable parallel execution because it would break some targets that don't
# specify exact dependencies like 'backend' which does currently not depend
# on 'frontend' to enable Node.js-less builds from source tarballs.
.NOTPARALLEL:
