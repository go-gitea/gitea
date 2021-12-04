THEME := themes/gitea
ARCHIVE := https://dl.gitea.io/theme/master.tar.gz
BRANCH := $(shell git rev-parse --abbrev-ref HEAD)
DOC_VERSION := $(shell git describe --tags --always | sed 's/-/+/' | sed 's/^v//')
PUBLIC := public/$(DOC_VERSION)
BASE_PATH := /$(DOC_VERSION)/
MAIN_BRANCH := main

ifeq ($(BRANCH), $(MAIN_BRANCH))
	DOC_VERSION := $(MAIN_BRANCH)
	PUBLIC := public
	BASE_PATH := /
endif

.PHONY: all
all: build

.PHONY: clean
clean:
	rm -rf $(PUBLIC) $(THEME)

.PHONY: trans-copy
trans-copy:
	@bash scripts/trans-copy

.PHONY: server
server: $(THEME)
	hugo server

.PHONY: build
build: $(THEME)
	hugo --cleanDestinationDir --destination=$(PUBLIC)

.PHONY: build-offline
build-offline: $(THEME)
	hugo --baseURL="$(BASE_PATH)" --destination=$(PUBLIC) --cleanDestinationDir

.PHONY: update
update: $(THEME)

$(THEME): $(THEME)/theme.toml
$(THEME)/theme.toml:
	mkdir -p $$(dirname $@)
	curl -L -s $(ARCHIVE) | tar xz -C $$(dirname $@)
