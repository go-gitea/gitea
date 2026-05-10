#!/bin/bash
set -euo pipefail

# Two modes:
#   pkgs           Read packages from stdin, write the TEST_SHARD's slice to stdout.
#                  Pass-through when TEST_SHARD is unset.
#   tests BINARY   Run a compiled *.test binary. When TEST_SHARD is set, enumerate
#                  top-level tests via -test.list and run only the shard's slice;
#                  TestMain skips environment setup in -test.list mode.

usage() {
  echo "usage: $0 {pkgs | tests BINARY}" >&2
  exit 2
}

partition_stdin() {
  if ! [[ "$TEST_TOTAL_SHARDS" =~ ^[1-9][0-9]*$ ]]; then
    echo "TEST_TOTAL_SHARDS must be a positive integer, got: $TEST_TOTAL_SHARDS" >&2
    exit 2
  fi
  if ! [[ "$TEST_SHARD" =~ ^[1-9][0-9]*$ ]] || [ "$TEST_SHARD" -gt "$TEST_TOTAL_SHARDS" ]; then
    echo "TEST_SHARD must be in [1, $TEST_TOTAL_SHARDS], got: $TEST_SHARD" >&2
    exit 2
  fi
  OUT=$(LC_ALL=C sort -u | awk -v r=$((TEST_SHARD - 1)) -v t="$TEST_TOTAL_SHARDS" '(NR - 1) % t == r')
  if [ -z "$OUT" ]; then
    echo "shard $TEST_SHARD/$TEST_TOTAL_SHARDS has no items assigned" >&2
    exit 1
  fi
  echo "$OUT"
}

case "${1:-}" in
  pkgs)
    if [ -z "${TEST_SHARD:-}" ]; then
      exec cat
    fi
    partition_stdin
    ;;
  tests)
    BINARY=${2:-}
    [ -n "$BINARY" ] || usage
    if [ -z "${TEST_SHARD:-}" ]; then
      exec "$BINARY"
    fi
    NAMES=$("$BINARY" -test.list='^Test' | partition_stdin)
    PATTERN=$(echo "$NAMES" | paste -sd '|' -)
    echo "Running shard $TEST_SHARD/$TEST_TOTAL_SHARDS ($(echo "$NAMES" | wc -l | tr -d ' ') tests)"
    exec "$BINARY" -test.run "^($PATTERN)\$"
    ;;
  *)
    usage
    ;;
esac
