#!/bin/bash
set -euo pipefail

# lint all go files with 'gopls check' and look for lines starting with the
# current absolute path, indicating a error was found. This is neccessary
# because the tool does not set non-zero exit code when errors are found.
# ref: https://github.com/golang/go/issues/67078
ERROR_LINES=$("$GO" run "$GOPLS_PACKAGE" check $@ | grep -P "^$PWD");
NUM_ERRORS=$(echo "$ERROR_LINES" | wc -l)

if [ "$NUM_ERRORS" -eq "0" ]; then
  exit 0;
else
  echo "$ERROR_LINES"
  echo "Found $NUM_ERRORS 'gopls check' errors"
  exit 1;
fi
