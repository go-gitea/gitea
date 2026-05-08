#!/bin/bash
# Run the gitea Go header check and golangci-lint. Returns golangci-lint's
# exit code on lint failure, otherwise the header check's.
#
# Set LINT_GO_INSTALL=1 in cross-compile envs (e.g. GOOS=windows); `go run`
# would build a target-platform binary that can't exec on the host.

if [ -n "$LINT_GO_INSTALL" ]; then
	GOOS= GOARCH= go install "$GOLANGCI_LINT_PACKAGE"
	host_env=(env GOOS= GOARCH=)
	linter=(golangci-lint run)
else
	host_env=()
	linter=(go run "$GOLANGCI_LINT_PACKAGE" run)
fi

"${host_env[@]}" go run tools/lint-go-header.go
header=$?
"${linter[@]}" "$@"
lint=$?

exit $((lint ? lint : header))
