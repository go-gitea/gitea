GO ?= go
HAS_GO = $(shell hash $(GO) > /dev/null 2>&1 && echo "GO" || echo "NOGO" )
ifeq ($(HAS_GO), GO)
	GOPATH ?= $(shell $(GO) env GOPATH)
	export PATH := $(GOPATH)/bin:$(PATH)
endif

GOFMT ?= gofmt -s

ifneq ($(RACE_ENABLED),)
	GOTESTFLAGS ?= -race
endif

GO_SOURCES := $(wildcard *.go)
GO_SOURCES_OWN := $(filter-out vendor/%, $(GO_SOURCES))
GO_PACKAGES ?= $(shell $(GO) list ./... | grep -v /vendor/)

.PHONY: revive
revive:
	@hash revive > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/mgechev/revive; \
	fi
	revive -config .revive.toml -exclude=./vendor/... ./... || exit 1

.PHONY: golangci-lint
golangci-lint:
	@hash golangci-lint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		export BINARY="golangci-lint"; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(GOPATH)/bin v1.26.0; \
	fi
	golangci-lint run --timeout 5m

.PHONY: lint
lint: golangci-lint revive

.PHONY: fmt
fmt:
	$(GOFMT) -w $(GO_SOURCES_OWN)

.PHONY: fmt-check
fmt-check:
	# get all go files and run go fmt on them
	@diff=$$($(GOFMT) -d $(GO_SOURCES_OWN)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

.PHONY: misspell-check
misspell-check:
	@hash misspell > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		$(GO) get -u github.com/client9/misspell/cmd/misspell; \
	fi
	misspell -error $(GO_SOURCES_OWN)

.PHONY: test
test:
	$(GO) test -cover -coverprofile coverage.out $(GOTESTFLAGS) $(GO_PACKAGES)
