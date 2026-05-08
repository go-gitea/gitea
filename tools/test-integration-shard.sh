#!/bin/bash
set -euo pipefail

# Run a deterministic shard of the integration test binary. Test names are
# enumerated from source — running the binary with -test.list isn't viable
# because TestMain boots the full Gitea environment and would panic without
# a configured database.

binary=$1
shard=${TEST_SHARD:?missing TEST_SHARD}
total=${TEST_TOTAL_SHARDS:?missing TEST_TOTAL_SHARDS}

names=$(grep -hE '^func Test[A-Z][A-Za-z0-9_]*\(' tests/integration/*.go \
  | sed -E 's/^func (Test[A-Z][A-Za-z0-9_]*).*/\1/' \
  | sort -u \
  | awk -v s="$shard" -v t="$total" 'NR % t == (s - 1) % t')

if [ -z "$names" ]; then
  echo "shard $shard/$total has no tests assigned" >&2
  exit 0
fi

pattern=$(echo "$names" | paste -sd '|' -)
echo "Running shard $shard/$total ($(echo "$names" | wc -l | tr -d ' ') tests)"
exec "$binary" -test.run "^($pattern)\$"
