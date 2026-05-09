#!/bin/bash
set -euo pipefail

# Print the TEST_SHARD/TEST_TOTAL_SHARDS slice of stdin (newline-separated
# items), partitioned round-robin by line number after a deterministic sort.
# Required env: TEST_SHARD (1..TEST_TOTAL_SHARDS), TEST_TOTAL_SHARDS (>= 1).

SHARD=${TEST_SHARD:?missing TEST_SHARD}
TOTAL=${TEST_TOTAL_SHARDS:?missing TEST_TOTAL_SHARDS}

if ! [[ "$TOTAL" =~ ^[1-9][0-9]*$ ]]; then
  echo "TEST_TOTAL_SHARDS must be a positive integer, got: $TOTAL" >&2
  exit 2
fi
if ! [[ "$SHARD" =~ ^[1-9][0-9]*$ ]] || [ "$SHARD" -gt "$TOTAL" ]; then
  echo "TEST_SHARD must be in [1, $TOTAL], got: $SHARD" >&2
  exit 2
fi

OUT=$(LC_ALL=C sort -u | awk -v r=$((SHARD - 1)) -v t="$TOTAL" '(NR - 1) % t == r')
if [ -z "$OUT" ]; then
  echo "shard $SHARD/$TOTAL has no items assigned" >&2
  exit 1
fi
echo "$OUT"
