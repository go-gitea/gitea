#!/bin/bash
# Run the gitea Go header check and golangci-lint. Returns golangci-lint's
# exit code on lint failure, otherwise the header check's.
#
# Set LINT_GO_INSTALL=1 in cross-compile envs (e.g. GOOS=windows); `go run`
# would build a target-platform binary that can't exec on the host.

if [ -n "$LINT_GO_INSTALL" ]; then
	GOOS= GOARCH= "$GO" install "$GOLANGCI_LINT_PACKAGE"
	GOOS= GOARCH= "$GO" run tools/lint-go-header.go; header=$?
	golangci-lint run "$@"; lint=$?
else
	"$GO" run tools/lint-go-header.go; header=$?
	"$GO" run "$GOLANGCI_LINT_PACKAGE" run "$@"; lint=$?
fi

exit $((lint ? lint : header))
