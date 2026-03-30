#!/usr/bin/env bash
set -euo pipefail

# check_backend_coverage.sh — Backend per-layer coverage check
#
# Runs Go unit and integration tests with coverage, then verifies that each
# layer meets its minimum threshold defined in docs/testing/strategy.md:
#
#   Domain:         >= 90%  (unit)
#   Application:    >= 80%  (unit)
#   Infrastructure: >= 70%  (integration)
#   API (handlers): >= 80%  (unit + integration merged)
#
# Thresholds have a warning zone of 10 percentage points below the minimum:
#   - actual >= threshold              → OK
#   - threshold-10 <= actual < threshold → WARNING (exits 0, posts PR comment)
#   - actual < threshold-10            → FAIL (exits 1)
#
# Requirements:
#   - Go toolchain available in PATH
#   - Docker available (required by Testcontainers for integration tests)
#
# Optional environment variables (CI only):
#   PR_NUMBER          — pull request number; enables PR comment on warnings
#   GITHUB_REPOSITORY  — owner/repo (e.g. courtknights/courtknights)
#   GITHUB_TOKEN       — GitHub token (needed by gh to post comments)

REPO_ROOT=$(git rev-parse --show-toplevel)
API_DIR="${REPO_ROOT}/api"

# Layer thresholds (from docs/testing/strategy.md)
THRESHOLD_DOMAIN=90
THRESHOLD_APPLICATION=80
THRESHOLD_INFRASTRUCTURE=70
THRESHOLD_API=80

# Warning zone: 10 percentage points below each threshold
WARNING_MARGIN=10

# Package prefixes as they appear in coverage profiles
MODULE="github.com/courtknights/courtknights"
PREFIX_DOMAIN="${MODULE}/internal/domain/"
PREFIX_APPLICATION="${MODULE}/internal/application/"
PREFIX_INFRASTRUCTURE="${MODULE}/internal/infrastructure/"
PREFIX_API="${MODULE}/internal/api/"

# Temporary directory for coverage files (cleaned up on exit)
COV_TMPDIR=$(mktemp -d)
trap 'rm -rf "$COV_TMPDIR"' EXIT

COV_UNIT="${COV_TMPDIR}/coverage_unit.out"
COV_INT="${COV_TMPDIR}/coverage_int.out"
COV_API="${COV_TMPDIR}/coverage_api.out"

FAILED=0
WARNED=0
# Accumulated markdown rows for the warning comment (pipe-separated)
WARN_ROWS=""

# ---------------------------------------------------------------------------
# Phase 1: Unit tests
# ---------------------------------------------------------------------------
echo "=== Phase 1: Running unit tests ==="
make -C "$REPO_ROOT" test-cov COV_OUT="$COV_UNIT"
echo ""

# ---------------------------------------------------------------------------
# Phase 2: Integration tests
# ---------------------------------------------------------------------------
echo "=== Phase 2: Running integration tests ==="
make -C "$REPO_ROOT" test-int-cov COV_OUT="$COV_INT"
echo ""

# ---------------------------------------------------------------------------
# Merge unit + integration profiles for the API handlers layer
# ---------------------------------------------------------------------------
echo "=== Merging profiles for API layer ==="
head -1 "$COV_UNIT" > "$COV_API"
grep -v "^mode:" "$COV_UNIT" >> "$COV_API"
grep -v "^mode:" "$COV_INT" >> "$COV_API"
echo "Merged unit + integration into API profile."
echo ""

# ---------------------------------------------------------------------------
# Helper: check coverage for a single layer
#
# Arguments:
#   $1 — layer display name (e.g. "domain")
#   $2 — package prefix to filter by
#   $3 — coverage file to use
#   $4 — minimum threshold (integer percentage)
# ---------------------------------------------------------------------------
check_layer() {
    local layer_name="$1"
    local prefix="$2"
    local coverage_file="$3"
    local threshold="$4"
    local warn_threshold
    warn_threshold=$(awk -v t="$threshold" -v m="$WARNING_MARGIN" 'BEGIN { print t - m }')
    local filtered="${COV_TMPDIR}/filtered_${layer_name}.out"

    # Build a filtered profile: mode line + lines matching this layer's prefix
    head -1 "$coverage_file" > "$filtered"
    grep -F "$prefix" "$coverage_file" >> "$filtered" || true

    # If the filtered file has only the mode line, there is no coverage data
    if [ "$(wc -l < "$filtered")" -le 1 ]; then
        echo "FAIL ${layer_name}: 0.0% < ${threshold}% (no coverage data found for prefix '${prefix}')"
        FAILED=1
        return
    fi

    # Extract total coverage percentage (e.g. "87.5%") from go tool cover output
    local actual
    actual=$(go tool cover -func="$filtered" | grep "^total:" | awk '{print $3}' | tr -d '%')

    # Three-way comparison via awk: ok / warn / fail
    local status
    status=$(awk -v a="$actual" -v t="$threshold" -v w="$warn_threshold" 'BEGIN {
        if (a + 0 >= t + 0)       print "ok"
        else if (a + 0 >= w + 0)  print "warn"
        else                       print "fail"
    }')

    if [ "$status" = "ok" ]; then
        printf "OK   %-16s %s%% >= %s%%\n" "${layer_name}:" "$actual" "$threshold"
    elif [ "$status" = "warn" ]; then
        printf "WARN %-16s %s%% (threshold %s%%, warning zone >= %s%%)\n" \
            "${layer_name}:" "$actual" "$threshold" "$warn_threshold"
        WARNED=1
        WARN_ROWS="${WARN_ROWS}| \`${layer_name}\` | ${actual}% | ${threshold}% | ${warn_threshold}% |\n"
    else
        printf "FAIL %-16s %s%% < %s%% (warning zone >= %s%%)\n" \
            "${layer_name}:" "$actual" "$threshold" "$warn_threshold"
        FAILED=1
    fi
}

# ---------------------------------------------------------------------------
# Check each layer against its threshold
# ---------------------------------------------------------------------------
echo "=== Checking layer coverage ==="
check_layer "domain"         "$PREFIX_DOMAIN"         "$COV_UNIT" "$THRESHOLD_DOMAIN"
check_layer "application"    "$PREFIX_APPLICATION"    "$COV_UNIT" "$THRESHOLD_APPLICATION"
check_layer "infrastructure" "$PREFIX_INFRASTRUCTURE" "$COV_INT"  "$THRESHOLD_INFRASTRUCTURE"
check_layer "api"            "$PREFIX_API"            "$COV_API"  "$THRESHOLD_API"
echo ""

# ---------------------------------------------------------------------------
# Post PR comment for warnings (CI only — requires PR_NUMBER and gh CLI)
# ---------------------------------------------------------------------------
if [ "$WARNED" -ne 0 ] && [ -n "${PR_NUMBER:-}" ]; then
    COMMENT_BODY="## Backend Coverage Warning

One or more layers are in the warning zone (within ${WARNING_MARGIN}% of their threshold).
Consider opening a task to improve coverage before it drops below the minimum.

| Layer | Actual | Threshold | Warning zone |
|-------|--------|-----------|--------------|
$(printf '%b' "$WARN_ROWS")
> A layer enters **error** state when it falls below the warning zone."

    gh pr comment "$PR_NUMBER" \
        --repo "${GITHUB_REPOSITORY:-}" \
        --body "$COMMENT_BODY"
    echo "Warning comment posted to PR #${PR_NUMBER}."
fi

# ---------------------------------------------------------------------------
# Final result
# ---------------------------------------------------------------------------
if [ "$FAILED" -ne 0 ]; then
    echo "Coverage check FAILED — one or more layers are below their threshold."
    exit 1
fi

if [ "$WARNED" -ne 0 ]; then
    echo "Coverage check PASSED with warnings — some layers are in the warning zone."
else
    echo "Coverage check PASSED — all layers meet their thresholds."
fi
exit 0
