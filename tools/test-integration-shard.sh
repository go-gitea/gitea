#!/bin/bash
set -euo pipefail

# Run a deterministic shard of the integration test binary. Test names are
# enumerated from source — running the binary with -test.list isn't viable
# because TestMain boots the full Gitea environment and would panic without
# a configured database.

binary=$1

# match `func Test*(t *testing.T|TB)` only — excludes TestMain (takes *testing.M)
names=$(grep -hE '^func Test[A-Z][A-Za-z0-9_]*\([a-zA-Z_][a-zA-Z0-9_]* \*testing\.(T|TB)\)' tests/integration/*.go \
  | sed -E 's/^func (Test[A-Z][A-Za-z0-9_]*).*/\1/' \
  | ./tools/partition-by-shard.sh)

if [ -z "$names" ]; then
  echo "no tests assigned to shard $TEST_SHARD/$TEST_TOTAL_SHARDS — likely a misconfiguration" >&2
  exit 1
fi

pattern=$(echo "$names" | paste -sd '|' -)
echo "Running shard $TEST_SHARD/$TEST_TOTAL_SHARDS ($(echo "$names" | wc -l | tr -d ' ') tests)"
exec "$binary" -test.run "^($pattern)\$"
