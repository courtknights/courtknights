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
# Thresholds have a warning zone of 10 percentage points below the minimum:
#   - actual >= threshold              → OK
#   - threshold-10 <= actual < threshold → WARNING (exits 0, posts PR comment)
#   - actual < threshold-10            → FAIL (exits 1)
#
# Requirements:
#   - Node.js and npm available in PATH
#   - jq available in PATH
#   - web/ dependencies installed (npm install)
#
# Optional environment variables (CI only):
#   PR_NUMBER          — pull request number; enables PR comment on warnings
#   GITHUB_REPOSITORY  — owner/repo (e.g. courtknights/courtknights)
#   GITHUB_TOKEN       — GitHub token (needed by gh to post comments)

REPO_ROOT=$(git rev-parse --show-toplevel)
WEB_DIR="${REPO_ROOT}/web"
COVERAGE_SUMMARY="${WEB_DIR}/coverage/coverage-summary.json"

THRESHOLD_GLOBAL=75
THRESHOLD_SERVICE=100

# Warning zone: 10 percentage points below each threshold
WARNING_MARGIN=10

FAILED=0
WARNED=0
# Accumulated markdown rows for the warning comment (pipe-separated)
WARN_ROWS=""

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
# Helper: three-way coverage comparison
#
# Arguments:
#   $1 — display label (e.g. "global statements")
#   $2 — actual percentage (float)
#   $3 — threshold (integer)
# Sets FAILED / WARNED and appends to WARN_ROWS.
# ---------------------------------------------------------------------------
check_coverage() {
    local label="$1"
    local actual="$2"
    local threshold="$3"
    local warn_threshold
    warn_threshold=$(awk -v t="$threshold" -v m="$WARNING_MARGIN" 'BEGIN { print t - m }')

    local status
    status=$(awk -v a="$actual" -v t="$threshold" -v w="$warn_threshold" 'BEGIN {
        if (a + 0 >= t + 0)       print "ok"
        else if (a + 0 >= w + 0)  print "warn"
        else                       print "fail"
    }')

    if [ "$status" = "ok" ]; then
        printf "OK   %s: %s%% >= %s%%\n" "$label" "$actual" "$threshold"
    elif [ "$status" = "warn" ]; then
        printf "WARN %s: %s%% (threshold %s%%, warning zone >= %s%%)\n" \
            "$label" "$actual" "$threshold" "$warn_threshold"
        WARNED=1
        WARN_ROWS="${WARN_ROWS}| \`${label}\` | ${actual}% | ${threshold}% | ${warn_threshold}% |\n"
    else
        printf "FAIL %s: %s%% < %s%% (warning zone >= %s%%)\n" \
            "$label" "$actual" "$threshold" "$warn_threshold"
        FAILED=1
    fi
}

# ---------------------------------------------------------------------------
# Check 1: global statement coverage for web/src/app/
# ---------------------------------------------------------------------------
echo "=== Checking global statement coverage ==="
actual_global=$(jq '.total.statements.pct' "$COVERAGE_SUMMARY")
check_coverage "global statements" "$actual_global" "$THRESHOLD_GLOBAL"
echo ""

# ---------------------------------------------------------------------------
# Check 2: function coverage for every *.service.ts file
# ---------------------------------------------------------------------------
echo "=== Checking service function coverage ==="

while IFS= read -r entry; do
    file=$(echo "$entry" | jq -r '.key')
    pct=$(echo "$entry"  | jq -r '.pct')
    check_coverage "$file" "$pct" "$THRESHOLD_SERVICE"
done < <(jq -c '
    to_entries[]
    | select(.key | endswith(".service.ts"))
    | { key: .key, pct: .value.functions.pct }
' "$COVERAGE_SUMMARY")

echo ""

# ---------------------------------------------------------------------------
# Post PR comment for warnings (CI only — requires PR_NUMBER and gh CLI)
# ---------------------------------------------------------------------------
if [ "$WARNED" -ne 0 ] && [ -n "${PR_NUMBER:-}" ]; then
    COMMENT_BODY="## Frontend Coverage Warning

One or more coverage checks are in the warning zone (within ${WARNING_MARGIN}% of their threshold).
Consider opening a task to improve coverage before it drops below the minimum.

| Check | Actual | Threshold | Warning zone |
|-------|--------|-----------|--------------|
$(printf '%b' "$WARN_ROWS")
> A check enters **error** state when it falls below the warning zone."

    gh pr comment "$PR_NUMBER" \
        --repo "${GITHUB_REPOSITORY:-}" \
        --body "$COMMENT_BODY"
    echo "Warning comment posted to PR #${PR_NUMBER}."
fi

# ---------------------------------------------------------------------------
# Final result
# ---------------------------------------------------------------------------
if [ "$FAILED" -ne 0 ]; then
    echo "Coverage check FAILED — one or more thresholds not met."
    exit 1
fi

if [ "$WARNED" -ne 0 ]; then
    echo "Coverage check PASSED with warnings — some checks are in the warning zone."
else
    echo "Coverage check PASSED — all thresholds met."
fi
exit 0
