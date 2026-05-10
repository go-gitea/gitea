#!/bin/bash
set -euo pipefail

# Run a compiled *.test binary. When TEST_SHARD is set, enumerate top-level
# tests via -test.list and run only the shard's slice; TestMain skips
# environment setup in -test.list mode. Without TEST_SHARD, runs the binary
# directly.

BINARY=${1:?usage: $0 BINARY}

if [ -z "${TEST_SHARD:-}" ]; then
  exec "$BINARY"
fi

if ! [[ "${TEST_TOTAL_SHARDS:-}" =~ ^[1-9][0-9]*$ ]]; then
  echo "TEST_TOTAL_SHARDS must be a positive integer, got: ${TEST_TOTAL_SHARDS:-}" >&2
  exit 2
fi
if ! [[ "$TEST_SHARD" =~ ^[1-9][0-9]*$ ]] || [ "$TEST_SHARD" -gt "$TEST_TOTAL_SHARDS" ]; then
  echo "TEST_SHARD must be in [1, $TEST_TOTAL_SHARDS], got: $TEST_SHARD" >&2
  exit 2
fi

NAMES=$("$BINARY" -test.list='^Test' | LC_ALL=C sort -u | awk -v r=$((TEST_SHARD - 1)) -v t="$TEST_TOTAL_SHARDS" '(NR - 1) % t == r')
if [ -z "$NAMES" ]; then
  echo "shard $TEST_SHARD/$TEST_TOTAL_SHARDS has no tests assigned" >&2
  exit 1
fi
PATTERN=$(echo "$NAMES" | paste -sd '|' -)
echo "Running shard $TEST_SHARD/$TEST_TOTAL_SHARDS ($(echo "$NAMES" | wc -l | tr -d ' ') tests)"
exec "$BINARY" -test.run "^($PATTERN)\$"
