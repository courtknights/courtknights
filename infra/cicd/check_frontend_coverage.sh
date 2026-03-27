#!/usr/bin/env bash
set -euo pipefail

# check_frontend_coverage.sh — Frontend coverage check
#
# Runs Angular (Jest) tests with coverage and verifies two thresholds
# defined in docs/testing/strategy.md:
#
#   1. Global statement coverage for web/src/app/: >= 75%
#   2. Function coverage for every *.service.ts file: 100%
#
# Requirements:
#   - Node.js and npm available in PATH
#   - jq available in PATH
#   - web/ dependencies installed (npm install)

REPO_ROOT=$(git rev-parse --show-toplevel)
WEB_DIR="${REPO_ROOT}/web"
COVERAGE_SUMMARY="${WEB_DIR}/coverage/coverage-summary.json"

THRESHOLD_GLOBAL=75
THRESHOLD_SERVICE=100

FAILED=0

# ---------------------------------------------------------------------------
# Run Jest with coverage
# ---------------------------------------------------------------------------
echo "=== Running frontend tests with coverage ==="
make -C "$WEB_DIR" test -- --coverage --watchAll=false
echo ""

# ---------------------------------------------------------------------------
# Verify coverage report was emitted
# ---------------------------------------------------------------------------
if [ ! -f "$COVERAGE_SUMMARY" ]; then
    echo "ERROR: Coverage report not found at: coverage/coverage-summary.json"
    echo "Ensure Jest is configured with coverageReporters: [\"json-summary\"] in jest.config.js"
    exit 1
fi

# ---------------------------------------------------------------------------
# Check 1: global statement coverage for web/src/app/
# ---------------------------------------------------------------------------
echo "=== Checking global statement coverage ==="

# Istanbul json-summary uses the key "total" for the aggregate entry
actual_global=$(jq '.total.statements.pct' "$COVERAGE_SUMMARY")

ok=$(awk -v a="$actual_global" -v t="$THRESHOLD_GLOBAL" \
    'BEGIN { print (a + 0 >= t + 0) ? "1" : "0" }')

if [ "$ok" -eq 1 ]; then
    printf "OK   global statements: %s%% >= %s%%\n" "$actual_global" "$THRESHOLD_GLOBAL"
else
    printf "FAIL global statements: %s%% < %s%%\n" "$actual_global" "$THRESHOLD_GLOBAL"
    FAILED=1
fi
echo ""

# ---------------------------------------------------------------------------
# Check 2: 100% function coverage for every *.service.ts file
# ---------------------------------------------------------------------------
echo "=== Checking service function coverage ==="

# jq iterates all keys in the summary; keys ending in .service.ts are services
while IFS= read -r entry; do
    file=$(echo "$entry" | jq -r '.key')
    pct=$(echo "$entry"  | jq -r '.pct')

    ok=$(awk -v a="$pct" -v t="$THRESHOLD_SERVICE" \
        'BEGIN { print (a + 0 >= t + 0) ? "1" : "0" }')

    if [ "$ok" -eq 1 ]; then
        printf "OK   %s: %s%%\n" "$file" "$pct"
    else
        printf "FAIL %s: %s%% < %s%%\n" "$file" "$pct" "$THRESHOLD_SERVICE"
        FAILED=1
    fi
done < <(jq -c '
    to_entries[]
    | select(.key | endswith(".service.ts"))
    | { key: .key, pct: .value.functions.pct }
' "$COVERAGE_SUMMARY")

echo ""

if [ "$FAILED" -ne 0 ]; then
    echo "Coverage check FAILED — one or more thresholds not met."
    exit 1
fi

echo "Coverage check PASSED — all thresholds met."
exit 0
