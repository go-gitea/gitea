#!/bin/bash
set -euo pipefail

# Run a deterministic shard of the integration test binary. Test names are
# enumerated from source — running the binary with -test.list isn't viable
# because TestMain boots the full Gitea environment and would panic without
# a configured database.

binary=$1
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

# match `func Test*(t *testing.T|TB)` only — excludes TestMain (takes *testing.M)
names=$(grep -hE '^func Test[A-Z][A-Za-z0-9_]*\([a-zA-Z_][a-zA-Z0-9_]* \*testing\.(T|TB)\)' tests/integration/*.go \
  | sed -E 's/^func (Test[A-Z][A-Za-z0-9_]*).*/\1/' \
  | LC_ALL=C sort -u \
  | awk -v r=$((shard - 1)) -v t="$total" 'NR % t == r')

if [ -z "$names" ]; then
  echo "no tests assigned to shard $shard/$total — likely a misconfiguration" >&2
  exit 1
fi

pattern=$(echo "$names" | paste -sd '|' -)
echo "Running shard $shard/$total ($(echo "$names" | wc -l | tr -d ' ') tests)"
exec "$binary" -test.run "^($pattern)\$"
