JSONNET_FMT := jsonnetfmt -n 2 --max-blank-lines 1 --string-style s --comment-style s

default: vendor build dashboards_out

all: fmt build

vendor: 
	jb install

fmt:
	find . -name 'vendor' -prune -o -name '*.libsonnet' -print -o -name '*.jsonnet' -print | \
		xargs -n 1 -- $(JSONNET_FMT) -i

lint: build
	find . -name 'vendor' -prune -o -name '*.libsonnet' -print -o -name '*.jsonnet' -print | \
		while read f; do \
			$(JSONNET_FMT) "$$f" | diff -u "$$f" -; \
		done
	mixtool lint mixin.libsonnet

dashboards_out: mixin.libsonnet config.libsonnet $(wildcard dashboards/*)
	@mkdir -p dashboards_out
	jsonnet -J vendor -m dashboards_out lib/dashboards.jsonnet

build: vendor
