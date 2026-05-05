DIST := dist
DIST_DIRS := $(DIST)/binaries $(DIST)/release

# By default use go's 1.25 experimental json v2 library when building
# TODO: remove when no longer experimental
export GOEXPERIMENT ?= jsonv2

GO ?= go
SHASUM ?= shasum -a 256
COMMA := ,

XGO_VERSION := go-1.26.x

AIR_PACKAGE ?= github.com/air-verse/air@v1 # renovate: datasource=go
EDITORCONFIG_CHECKER_PACKAGE ?= github.com/editorconfig-checker/editorconfig-checker/v3/cmd/editorconfig-checker@v3 # renovate: datasource=go
GOLANGCI_LINT_PACKAGE ?= github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4 # renovate: datasource=go
GXZ_PACKAGE ?= github.com/ulikunitz/xz/cmd/gxz@v0.5.15 # renovate: datasource=go
MISSPELL_PACKAGE ?= github.com/golangci/misspell/cmd/misspell@v0.8.0 # renovate: datasource=go
SWAGGER_PACKAGE ?= github.com/go-swagger/go-swagger/cmd/swagger@v0.33.1 # renovate: datasource=go
XGO_PACKAGE ?= src.techknowlogick.com/xgo@latest
GOVULNCHECK_PACKAGE ?= golang.org/x/vuln/cmd/govulncheck@v1 # renovate: datasource=go
ACTIONLINT_PACKAGE ?= github.com/rhysd/actionlint/cmd/actionlint@v1.7.11 # renovate: datasource=go

HAS_GO := $(shell hash $(GO) > /dev/null 2>&1 && echo yes)
ifeq ($(HAS_GO), yes)
	CGO_EXTRA_CFLAGS := -DSQLITE_MAX_VARIABLE_NUMBER=32766
	CGO_CFLAGS ?= $(shell $(GO) env CGO_CFLAGS) $(CGO_EXTRA_CFLAGS)
endif

MAKE_EVIDENCE_DIR := .make_evidence

# Use sqlite as default database if running tests, only do so for local tests, not in CI.
# CI should explicitly set the database to avoid unexpected results.
ifneq ($(findstring test-,$(MAKECMDGOALS)),)
	ifeq ($(CI),)
		GITEA_TEST_DATABASE ?= sqlite
	endif
endif

TAGS ?=
ifeq ($(GITEA_TEST_DATABASE),sqlite)
	TAGS += sqlite sqlite_unlock_notify
endif
TAGS_EVIDENCE := $(MAKE_EVIDENCE_DIR)/tags

CGO_ENABLED ?= 0
ifneq (,$(findstring sqlite,$(TAGS))$(findstring pam,$(TAGS)))
	CGO_ENABLED = 1
endif

STATIC ?=
EXTLDFLAGS ?=
ifneq ($(STATIC),)
	EXTLDFLAGS = -extldflags "-static"
endif

ifeq ($(GOOS),windows)
	IS_WINDOWS := yes
else ifeq ($(patsubst Windows%,Windows,$(OS)),Windows)
	ifeq ($(GOOS),)
		IS_WINDOWS := yes
	endif
endif

# GOFLAGS and EXTRA_GOFLAGS are for the 'go build' command only
ifeq ($(IS_WINDOWS),yes)
	GOFLAGS := -v -buildmode=exe
	EXECUTABLE ?= gitea.exe
else
	GOFLAGS := -v
	EXECUTABLE ?= gitea
endif
EXTRA_GOFLAGS ?=

ifeq ($(shell sed --version 2>/dev/null | grep -q GNU && echo gnu),gnu)
	SED_INPLACE := sed -i
else
	SED_INPLACE := sed -i ''
endif

# GOTEST_FLAGS is for unit test and integration test
GOTEST_FLAGS ?= -timeout 40m

STORED_VERSION_FILE := VERSION

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

LDFLAGS := $(LDFLAGS) -X "main.Version=$(GITEA_VERSION)" -X "main.Tags=$(TAGS)"

LINUX_ARCHS ?= linux/amd64,linux/386,linux/arm-5,linux/arm-6,linux/arm64,linux/riscv64

GO_TEST_PACKAGES ?= $(filter-out $(shell $(GO) list code.gitea.io/gitea/models/migrations/...) code.gitea.io/gitea/tests/integration/migration-test code.gitea.io/gitea/tests code.gitea.io/gitea/tests/integration,$(shell $(GO) list ./... | grep -v /vendor/))
MIGRATE_TEST_PACKAGES ?= $(shell $(GO) list code.gitea.io/gitea/models/migrations/...)

FRONTEND_SOURCES := $(shell find web_src/js web_src/css -type f)
FRONTEND_CONFIGS := vite.config.ts tailwind.config.ts
FRONTEND_DEST := public/assets/.vite/manifest.json
FRONTEND_DEST_ENTRIES := public/assets/js public/assets/css public/assets/fonts public/assets/.vite
FRONTEND_DEV_LOG_LEVEL ?= warn

BINDATA_DEST_WILDCARD := modules/migration/bindata.* modules/public/bindata.* modules/options/bindata.* modules/templates/bindata.*

GENERATED_GO_DEST := modules/charset/invisible_gen.go modules/charset/ambiguous_gen.go

SVG_DEST_DIR := public/assets/img/svg

AIR_TMP_DIR := .air

GO_LICENSE_FILE := assets/go-licenses.json

TAR_EXCLUDES := .git data indexers queues log node_modules $(EXECUTABLE) $(DIST) $(MAKE_EVIDENCE_DIR) $(AIR_TMP_DIR)

GO_DIRS := build cmd models modules routers services tests tools
WEB_DIRS := web_src/js web_src/css

ESLINT_FILES := web_src/js tools *.ts tests/e2e
STYLELINT_FILES := web_src/css web_src/js/components/*.vue
SPELLCHECK_FILES := $(GO_DIRS) $(WEB_DIRS) templates options/locale/locale_en-US.json .github $(filter-out CHANGELOG.md, $(wildcard *.go *.md *.yml *.yaml *.toml))
EDITORCONFIG_FILES := templates .github/workflows options/locale/locale_en-US.json

GO_SOURCES := $(wildcard *.go)
GO_SOURCES += $(shell find $(GO_DIRS) -type f -name "*.go")
GO_SOURCES += $(GENERATED_GO_DEST)

ESLINT_CONCURRENCY ?= 2

SWAGGER_SPEC := templates/swagger/v1_json.tmpl
SWAGGER_SPEC_INPUT := templates/swagger/v1_input.json
SWAGGER_EXCLUDE := code.gitea.io/sdk
OPENAPI3_SPEC := templates/swagger/v1_openapi3_json.tmpl

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
TEST_MSSQL_DBNAME ?= testgitea
TEST_MSSQL_USERNAME ?= sa
TEST_MSSQL_PASSWORD ?= MwantsaSecurePassword1

# Include local Makefile
# Makefile.local is listed in .gitignore
ifneq ("$(wildcard Makefile.local)","")
	include Makefile.local
endif

$(foreach v, $(filter TEST_%, $(.VARIABLES)), $(eval MAKEFILE_VARS+=$v=$($v)))
$(foreach v, $(filter GITEA_TEST_%, $(.VARIABLES)), $(eval MAKEFILE_VARS+=$v=$($v)))
export MAKEFILE_VARS

.PHONY: all
all: build

.PHONY: help
help: Makefile ## print Makefile help information.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m[TARGETS] default target: build\033[0m\n\n\033[35mTargets:\033[0m\n"} /^[0-9A-Za-z._-]+:.*?##/ { printf "  \033[36m%-45s\033[0m %s\n", $$1, $$2 }' Makefile #$(MAKEFILE_LIST)
	@printf "  \033[36m%-46s\033[0m %s\n" "test-e2e" "test end to end using playwright"
	@printf "  \033[36m%-46s\033[0m %s\n" "test-backend[#TestSpecificName]" "run unit test (sqlite only)"
	@printf "  \033[36m%-46s\033[0m %s\n" "test-integration[#TestSpecificName]" "run integration test for GITEA_TEST_DATABASE (sqlite, mysql, pgsql, mssql)"

.PHONY: clean-all
clean-all: clean ## delete backend, frontend and integration files
	rm -rf $(FRONTEND_DEST_ENTRIES) node_modules

.PHONY: clean
clean: ## delete backend and integration files
	rm -f $(EXECUTABLE) test-*.test tests/*.ini
	rm -rf  $(DIST) $(BINDATA_DEST_WILDCARD) man tests/integration/gitea-integration-*

.PHONY: fmt
fmt: ## format the Go and template code
	$(GO) run $(GOLANGCI_LINT_PACKAGE) fmt
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
generate-swagger: $(SWAGGER_SPEC) $(OPENAPI3_SPEC) ## generate the swagger spec from code comments

$(SWAGGER_SPEC): $(GO_SOURCES) $(SWAGGER_SPEC_INPUT)
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

.PHONY: generate-openapi3
generate-openapi3: $(OPENAPI3_SPEC) ## generate the OpenAPI 3.0 spec from the Swagger 2.0 spec

$(OPENAPI3_SPEC): $(SWAGGER_SPEC) build/generate-openapi.go $(wildcard build/openapi3gen/*.go)
	$(GO) run build/generate-openapi.go

.PHONY: openapi3-check
openapi3-check: generate-openapi3
	@diff=$$(git diff --color=always '$(OPENAPI3_SPEC)'); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make generate-openapi3' and commit the result:"; \
		printf "%s" "$${diff}"; \
		exit 1; \
	fi

.PHONY: checks
checks: checks-frontend checks-backend ## run various consistency checks

.PHONY: checks-frontend
checks-frontend: lockfile-check svg-check ## check frontend files

.PHONY: checks-backend
checks-backend: tidy-check swagger-check openapi3-check fmt-check swagger-validate security-check ## check backend files

.PHONY: lint
lint: lint-frontend lint-backend lint-templates lint-swagger lint-spell lint-md lint-actions lint-json lint-yaml ## lint everything

.PHONY: lint-fix
lint-fix: lint-frontend-fix lint-backend-fix lint-spell-fix ## lint everything and fix issues

.PHONY: lint-frontend
lint-frontend: lint-js lint-css ## lint frontend files

.PHONY: lint-frontend-fix
lint-frontend-fix: lint-js-fix lint-css-fix ## lint frontend files and fix issues

.PHONY: lint-backend
lint-backend: lint-go lint-editorconfig ## lint backend files

.PHONY: lint-backend-fix
lint-backend-fix: lint-go-fix lint-editorconfig ## lint backend files and fix issues

.PHONY: lint-js
lint-js: node_modules ## lint js and ts files
	pnpm exec eslint --color --max-warnings=0 --concurrency $(ESLINT_CONCURRENCY) $(ESLINT_FILES)
	pnpm exec vue-tsc

.PHONY: lint-js-fix
lint-js-fix: node_modules ## lint js and ts files and fix issues
	pnpm exec eslint --color --max-warnings=0 --concurrency $(ESLINT_CONCURRENCY) $(ESLINT_FILES) --fix
	pnpm exec vue-tsc

.PHONY: lint-css
lint-css: node_modules ## lint css files
	pnpm exec stylelint --color --max-warnings=0 $(STYLELINT_FILES)

.PHONY: lint-css-fix
lint-css-fix: node_modules ## lint css files and fix issues
	pnpm exec stylelint --color --max-warnings=0 $(STYLELINT_FILES) --fix

.PHONY: lint-swagger
lint-swagger: node_modules ## lint swagger files
	pnpm exec spectral lint -q -F hint $(SWAGGER_SPEC)

.PHONY: lint-md
lint-md: node_modules ## lint markdown files
	pnpm exec markdownlint *.md

.PHONY: lint-md-fix
lint-md-fix: node_modules ## lint markdown files and fix issues
	pnpm exec markdownlint --fix *.md

.PHONY: lint-spell
lint-spell: ## lint spelling
	@git ls-files $(SPELLCHECK_FILES) | xargs go run $(MISSPELL_PACKAGE) -dict assets/misspellings.csv -error

.PHONY: lint-spell-fix
lint-spell-fix: ## lint spelling and fix issues
	@git ls-files $(SPELLCHECK_FILES) | xargs go run $(MISSPELL_PACKAGE) -dict assets/misspellings.csv -w

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

.PHONY: lint-editorconfig
lint-editorconfig:
	@echo "Running editorconfig check..."
	@$(GO) run $(EDITORCONFIG_CHECKER_PACKAGE) $(EDITORCONFIG_FILES)

.PHONY: lint-actions
lint-actions: ## lint action workflow files
	$(GO) run $(ACTIONLINT_PACKAGE)

.PHONY: lint-templates
lint-templates: .venv node_modules ## lint template files
	@node tools/lint-templates-svg.ts
	@uv run --frozen djlint $(shell find templates -type f -iname '*.tmpl')

.PHONY: lint-yaml
lint-yaml: .venv ## lint yaml files
	@uv run --frozen yamllint -s .

.PHONY: lint-json
lint-json: node_modules ## lint json files
	pnpm exec eslint -c eslint.json.config.ts --color --max-warnings=0 --concurrency $(ESLINT_CONCURRENCY)

.PHONY: lint-json-fix
lint-json-fix: node_modules ## lint and fix json files
	pnpm exec eslint -c eslint.json.config.ts --color --max-warnings=0 --concurrency $(ESLINT_CONCURRENCY) --fix

.PHONY: watch
watch: ## watch everything and continuously rebuild
	@bash tools/watch.sh

.PHONY: watch-frontend
watch-frontend: node_modules ## start vite dev server for frontend
	NODE_ENV=development pnpm exec vite --logLevel $(FRONTEND_DEV_LOG_LEVEL)

.PHONY: watch-backend
watch-backend: ## watch backend files and continuously rebuild
	GITEA_RUN_MODE=dev $(GO) run $(AIR_PACKAGE) -c .air.toml

.PHONY: test-backend
test-backend: ## test backend files
	@echo "Running go test with $(GOTEST_FLAGS) -tags '$(TAGS)'..."
	@$(GO) test $(GOTEST_FLAGS) -tags='$(TAGS)' $(GO_TEST_PACKAGES)

.PHONY: test-frontend
test-frontend: node_modules ## test frontend files
	pnpm exec vitest

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

.PHONY: test-backend\#%
test-backend\#%:
	@echo "Running go test with -tags '$(TAGS)'..."
	@$(GO) test $(GOTEST_FLAGS) -tags='$(TAGS)' -run $(subst .,/,$*) $(GO_TEST_PACKAGES)

.PHONY: coverage
coverage:
	grep '^\(mode: .*\)\|\(.*:[0-9]\+\.[0-9]\+,[0-9]\+\.[0-9]\+ [0-9]\+ [0-9]\+\)$$' coverage.out > coverage-bodged.out
	grep '^\(mode: .*\)\|\(.*:[0-9]\+\.[0-9]\+,[0-9]\+\.[0-9]\+ [0-9]\+ [0-9]\+\)$$' integration.coverage.out > integration.coverage-bodged.out
	$(GO) run tools/gocovmerge.go integration.coverage-bodged.out coverage-bodged.out > coverage.all

.PHONY: unit-test-coverage
unit-test-coverage:
	@echo "Running unit-test-coverage $(GOTEST_FLAGS) -tags '$(TAGS)'..."
	@$(GO) test $(GOTEST_FLAGS) -tags='$(TAGS)' -cover -coverprofile coverage.out $(GO_TEST_PACKAGES) && echo "\n==>\033[32m Ok\033[m\n" || exit 1

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
	GO=$(GO) $(GO) run build/generate-go-licenses.go $(GO_LICENSE_FILE)

.PHONY: test-integration
test-integration:
	@# Use a compiled binary: testlogger forwards gitea logs to t.Log, so `go test -v`
	@# would flood output per passing test. testcache can't help these tests anyway —
	@# they mutate the work directory, so cache inputs change between runs.
	$(GO) test $(GOTEST_FLAGS) -tags '$(TAGS)' -c code.gitea.io/gitea/tests/integration -o ./test-integration-$(GITEA_TEST_DATABASE).test
	./test-integration-$(GITEA_TEST_DATABASE).test

.PHONY: test-integration\#%
test-integration\#%:
	$(GO) test $(GOTEST_FLAGS) -tags '$(TAGS)' -run $(subst .,/,$*) code.gitea.io/gitea/tests/integration

.PHONY: test-migration
test-migration: migrations.integration.test migrations.individual.test

.PHONY: migrations.integration.test
migrations.integration.test:
	$(GO) test $(GOTEST_FLAGS) -tags '$(TAGS)' code.gitea.io/gitea/tests/integration/migration-test

.PHONY: migrations.individual.test
migrations.individual.test:
	@# tests of multiple packages use the same database, don't run in parallel
	$(GO) test $(GOTEST_FLAGS) -tags '$(TAGS)' -p 1 $(MIGRATE_TEST_PACKAGES)

.PHONY: migrations.individual.test\#%
migrations.individual.test\#%:
	$(GO) test $(GOTEST_FLAGS) -tags '$(TAGS)' code.gitea.io/gitea/models/migrations/$*

.PHONY: playwright
playwright: deps-frontend
	@# on GitHub Actions VMs, playwright's system deps are pre-installed
	@pnpm exec playwright install $(if $(GITHUB_ACTIONS),,--with-deps) chromium firefox $(PLAYWRIGHT_FLAGS)

.PHONY: test-e2e
test-e2e: playwright frontend backend
	@EXECUTABLE=$(EXECUTABLE) ./tools/test-e2e.sh $(GITEA_TEST_E2E_FLAGS)

.PHONY: build
build: frontend backend ## build everything

.PHONY: frontend
frontend: $(FRONTEND_DEST) ## build frontend files

.PHONY: backend
backend: generate-backend $(EXECUTABLE) ## build backend files

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
	GOEXPERIMENT= go run $(GOVULNCHECK_PACKAGE) -show color ./... || true

$(EXECUTABLE): $(GO_SOURCES) $(TAGS_PREREQ)
ifneq ($(and $(STATIC),$(findstring pam,$(TAGS))),)
  $(error pam support set via TAGS does not support static builds)
endif
	CGO_ENABLED="$(CGO_ENABLED)" CGO_CFLAGS="$(CGO_CFLAGS)" $(GO) build $(GOFLAGS) $(EXTRA_GOFLAGS) -tags '$(TAGS)' -ldflags '-s -w $(EXTLDFLAGS) $(LDFLAGS)' -o $@

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
	$(GO) install $(GOLANGCI_LINT_PACKAGE) & \
	$(GO) install $(GXZ_PACKAGE) & \
	$(GO) install $(MISSPELL_PACKAGE) & \
	$(GO) install $(SWAGGER_PACKAGE) & \
	$(GO) install $(XGO_PACKAGE) & \
	$(GO) install $(GOVULNCHECK_PACKAGE) & \
	$(GO) install $(ACTIONLINT_PACKAGE) & \
	wait

node_modules: pnpm-lock.yaml
	pnpm install --frozen-lockfile
	@touch node_modules

.venv: uv.lock
	uv sync
	@touch .venv

.PHONY: update
update: update-go update-js update-py ## update dependencies

.PHONY: update-go
update-go: ## update go dependencies
	$(GO) get -u ./...
	$(MAKE) tidy

.PHONY: update-js
update-js: node_modules ## update js dependencies
	pnpm exec updates -u -f package.json
	rm -rf node_modules pnpm-lock.yaml
	pnpm install
	@touch node_modules
	$(MAKE) --no-print-directory nolyfill

.PHONY: nolyfill
nolyfill: node_modules ## apply nolyfill overrides to package.json and relock
	pnpm exec nolyfill install
	pnpm install
	@touch node_modules

.PHONY: update-py
update-py: node_modules ## update py dependencies
	pnpm exec updates -u -f pyproject.toml
	rm -rf .venv uv.lock
	uv sync
	@touch .venv

.PHONY: vite
vite: $(FRONTEND_DEST) ## build vite files

$(FRONTEND_DEST): $(FRONTEND_SOURCES) $(FRONTEND_CONFIGS) pnpm-lock.yaml
	@$(MAKE) -s node_modules
	@rm -rf $(FRONTEND_DEST_ENTRIES)
	@echo "Running vite build..."
	@pnpm exec vite build
	@touch $(FRONTEND_DEST)

.PHONY: svg
svg: node_modules ## build svg files
	rm -rf $(SVG_DEST_DIR)
	node tools/generate-svg.ts

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
	pnpm install --frozen-lockfile
	@diff=$$(git diff --color=always pnpm-lock.yaml); \
	if [ -n "$$diff" ]; then \
		echo "pnpm-lock.yaml is inconsistent with package.json"; \
		echo "Please run 'pnpm install --frozen-lockfile' and commit the result:"; \
		printf "%s" "$${diff}"; \
		exit 1; \
	fi

.PHONY: generate-gitignore
generate-gitignore: ## update gitignore files
	$(GO) run build/generate-gitignores.go

.PHONY: generate-images
generate-images: | node_modules ## generate images
	cd tools && node generate-images.ts $(TAGS)

.PHONY: generate-manpage
generate-manpage: ## generate manpage
	@[ -f gitea ] || make backend
	@mkdir -p man/man1/ man/man5
	@./gitea docs --man > man/man1/gitea.1
	@gzip -9 man/man1/gitea.1 && echo man/man1/gitea.1.gz created
	@#TODO A small script that formats config-cheat-sheet.en-us.md nicely for use as a config man page

# Disable parallel execution because it would break some targets that don't
# specify exact dependencies like 'backend' which does currently not depend
# on 'frontend' to enable Node.js-less builds from source tarballs.
.NOTPARALLEL:
