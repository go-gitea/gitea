THEME := themes/gitea
PUBLIC := public
ARCHIVE := https://dl.gitea.io/theme/master.tar.gz

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
	hugo --cleanDestinationDir

.PHONY: build-offline
build-offline: $(THEME)
	hugo --baseURL="/" --cleanDestinationDir

.PHONY: update
update: $(THEME)

$(THEME): $(THEME)/theme.toml
$(THEME)/theme.toml:
	mkdir -p $$(dirname $@)
	curl -L -s $(ARCHIVE) | tar xz -C $$(dirname $@)
