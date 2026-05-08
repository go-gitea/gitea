#!/bin/bash
# Run the gitea Go header check together with the golangci-lint command
# given in $@. If both fail, golangci-lint's exit code wins so callers
# can distinguish its codes (1, 2, 5, ...) from the header check's.

GOOS= GOARCH= go run tools/lint-go-header.go
header=$?

"$@"
lint=$?

exit $((lint ? lint : header))
