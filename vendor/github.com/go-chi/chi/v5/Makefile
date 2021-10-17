all:
	@echo "**********************************************************"
	@echo "**                    chi build tool                    **"
	@echo "**********************************************************"


test:
	go clean -testcache && $(MAKE) test-router && $(MAKE) test-middleware

test-router:
	go test -race -v .

test-middleware:
	go test -race -v ./middleware

.PHONY: docs
docs:
	npx docsify-cli serve ./docs
