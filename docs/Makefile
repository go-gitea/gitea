THEME := themes/gitea
PUBLIC := public
ARCHIVE := https://dl.gitea.com/theme/main.tar.gz

HUGO_PACKAGE := github.com/gohugoio/hugo@v0.111.3

.PHONY: all
all: build

.PHONY: clean
clean:
	rm -rf $(PUBLIC) $(THEME)

.PHONY: trans-copy
trans-copy:
	bash scripts/trans-copy.sh

.PHONY: server
server: $(THEME)
	go run $(HUGO_PACKAGE) server

.PHONY: build
build: $(THEME)
	go run $(HUGO_PACKAGE) --cleanDestinationDir

.PHONY: build-offline
build-offline: $(THEME)
	go run $(HUGO_PACKAGE) --baseURL="/" --cleanDestinationDir

.PHONY: update
update: $(THEME)

$(THEME): $(THEME)/theme.toml
$(THEME)/theme.toml:
	mkdir -p $$(dirname $@)
	curl -L -s $(ARCHIVE) | tar xz -C $$(dirname $@)
