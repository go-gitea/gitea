#!/bin/bash
set -euo pipefail

# Run a deterministic shard of the integration test binary. Test names are
# enumerated from source — running the binary with -test.list isn't viable
# because TestMain boots the full Gitea environment and would panic without
# a configured database.

BINARY=$1

# match `func Test...(t *testing.T)` only — `*testing.M` excludes TestMain
NAMES=$(grep -hE '^func Test[A-Za-z0-9_]*\([a-zA-Z_][a-zA-Z0-9_]* \*testing\.T\)' tests/integration/*.go \
  | sed -E 's/^func (Test[A-Za-z0-9_]*).*/\1/' \
  | ./tools/partition-by-shard.sh)

PATTERN=$(echo "$NAMES" | paste -sd '|' -)
echo "Running shard $TEST_SHARD/$TEST_TOTAL_SHARDS ($(echo "$NAMES" | wc -l | tr -d ' ') tests)"
exec "$BINARY" -test.run "^($PATTERN)\$"
