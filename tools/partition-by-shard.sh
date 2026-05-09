#!/bin/bash
set -euo pipefail

# Print the TEST_SHARD/TEST_TOTAL_SHARDS slice of stdin (newline-separated
# items), partitioned round-robin by line number after a deterministic sort.
# Required env: TEST_SHARD (1..TEST_TOTAL_SHARDS), TEST_TOTAL_SHARDS (>= 1).

shard=${TEST_SHARD:?missing TEST_SHARD}
total=${TEST_TOTAL_SHARDS:?missing TEST_TOTAL_SHARDS}

if ! [[ "$total" =~ ^[1-9][0-9]*$ ]]; then
  echo "TEST_TOTAL_SHARDS must be a positive integer, got: $total" >&2
  exit 2
fi
if ! [[ "$shard" =~ ^[1-9][0-9]*$ ]] || [ "$shard" -gt "$total" ]; then
  echo "TEST_SHARD must be in [1, $total], got: $shard" >&2
  exit 2
fi

LC_ALL=C sort -u | awk -v r=$((shard - 1)) -v t="$total" 'NR % t == r'
