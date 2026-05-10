#!/bin/bash
set -euo pipefail

# Run a deterministic shard of the integration test binary. Test names are
# enumerated by the binary's own -test.list flag; TestMain skips environment
# setup in that mode so listing works without a configured database.

BINARY=$1

NAMES=$("$BINARY" -test.list='^Test' | "$(dirname "$0")/partition-by-shard.sh")
PATTERN=$(echo "$NAMES" | paste -sd '|' -)
echo "Running shard $TEST_SHARD/$TEST_TOTAL_SHARDS ($(echo "$NAMES" | wc -l | tr -d ' ') tests)"
exec "$BINARY" -test.run "^($PATTERN)\$"
