#!/bin/bash
set -uo pipefail

cd "$(dirname -- "${BASH_SOURCE[0]}")" && cd ..

IGNORE_PATTERNS=(
  "is deprecated" # TODO: fix these
)

# lint all go files with 'gopls check' and look for lines starting with the
# current absolute path, indicating a error was found. This is neccessary
# because the tool does not set non-zero exit code when errors are found.
# ref: https://github.com/golang/go/issues/67078
ERROR_LINES=$("$GO" run "$GOPLS_PACKAGE" check "$@" 2>/dev/null | grep -E "^$PWD" | grep -vFf <(printf '%s\n' "${IGNORE_PATTERNS[@]}"));
NUM_ERRORS=$(echo -n "$ERROR_LINES" | wc -l)

if [ "$NUM_ERRORS" -eq "0" ]; then
  exit 0;
else
  echo "$ERROR_LINES"
  echo "Found $NUM_ERRORS 'gopls check' errors"
  exit 1;
fi
