#!/bin/bash
set -euo pipefail

# Print packages with tests whose own code or test code imports any of the
# gogit-affected modules (modules/git, modules/gitrepo, modules/lfs). These
# are the packages whose tests can observe behavioral differences between
# the bindata and bindata+gogit tag sets.
#
# Packages without tests are intentionally skipped — they're compiled
# transitively by their consumers, so any tag-related compile error would
# already surface in those consumers' test builds.

TAGS=${1:?usage: $0 TAGS}

# Exclusions mirror the Makefile's GO_TEST_PACKAGES filter — these packages
# need a real database / dedicated harness and are tested separately.
OUT=$(go list -tags "$TAGS" -f '{{if or .TestGoFiles .XTestGoFiles}}{{.ImportPath}}|{{range .Imports}}{{.}};{{end}}{{range .TestImports}}{{.}};{{end}}{{range .XTestImports}}{{.}};{{end}}{{end}}' ./... \
  | awk -F'|' '
      $2 ~ /code\.gitea\.io\/gitea\/modules\/(git|gitrepo|lfs)[\/;]/ &&
      $1 !~ /^code\.gitea\.io\/gitea\/(models\/migrations(\/|$)|tests(\/integration(\/migration-test)?)?$)/ {
        print $1
      }' \
  | sort -u)
if [ -z "$OUT" ]; then
  echo "no gogit-affected packages found" >&2
  exit 1
fi
echo "$OUT"
