PACKAGES ?= $(shell go list ./...)

.PHONY: check
check: lint
	go test

.PHONY: lint
lint:
	@which golint > /dev/null; if [ $$? -ne 0 ]; then \
		go get -u github.com/golang/lint/golint; \
	fi
	@for PKG in $(PACKAGES); do golint -set_exit_status $$PKG || exit 1; done;
