#!/bin/sh

[ -z "$GO" -o -z "$GOLANGCI_LINT_PACKAGE" ] && {
  echo "GO and GOLANGCI_LINT_PACKAGE environment variables must be set"
  exit 1
}

# 'go run' can not have distinct GOOS/GOARCH for its build and run steps
# so pre-compile a binary and run it for different target platforms
echo "installing golangci-lint ..."
GOOS= GOARCH= "$GO" install "$GOLANGCI_LINT_PACKAGE" || exit $?

# always run all linters and report all issues, even if some of them fail
status=0

echo "lint SPDX header ..."
GOOS= "$GO" run tools/lint-go-header.go || status=$?

echo "lint for linux ..."
GOOS=linux TAGS=bindata golangci-lint run || status=$?

echo "lint for windows ..."
GOOS=windows TAGS=gogit golangci-lint run || status=$?

exit $status;
