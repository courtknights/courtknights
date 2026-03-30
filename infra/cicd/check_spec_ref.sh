#!/usr/bin/env bash
set -euo pipefail

# check_spec_ref.sh — Spec reference check
#
# Verifies that a PR body contains a valid spec reference pointing to an approved
# acceptance document in docs/specs/.
#
# Required environment variables:
#   PR_BODY   — full text of the PR description
#   PR_LABELS — comma-separated list of labels on the PR (may be empty)
#
# Exits 0 if the check passes or the PR is exempt via the 'no-spec' label.
# Exits 1 if the check fails.

REPO_ROOT=$(git rev-parse --show-toplevel)

# 1. Check for no-spec exemption
if echo "${PR_LABELS:-}" | grep -qF "no-spec"; then
    echo "Exempt: PR has 'no-spec' label — skipping spec reference check."
    exit 0
fi

# 2. Check for spec reference line.
# printf '%b' expands literal \n sequences that may appear when PR_BODY is passed
# via shell assignment (e.g. PR_BODY="line1\nSpec: docs/specs/...").
SPEC_LINE=$(printf '%b' "${PR_BODY:-}" | grep -F "Spec: docs/specs/" | head -n1 || true)

if [ -z "$SPEC_LINE" ]; then
    echo "ERROR: PR body does not contain a 'Spec: docs/specs/' line."
    echo "Add a line like: Spec: docs/specs/FEATURE_xxx/02_architecture.md"
    exit 1
fi

# 3. Extract the spec path (everything after "Spec: ", whitespace trimmed)
SPEC_PATH=$(echo "$SPEC_LINE" | sed 's/.*Spec: //' | tr -d '[:space:]')

# 4. Validate it starts with docs/specs/ (rejects paths like "some/other/path")
case "$SPEC_PATH" in
    docs/specs/*)
        ;;
    *)
        echo "ERROR: Spec path must start with 'docs/specs/'. Got: $SPEC_PATH"
        exit 1
        ;;
esac

# 5. Derive the spec directory from the path
# Strip trailing slashes first; if the result has no .md extension treat it
# as a directory path directly, otherwise take dirname of the file path.
SPEC_PATH="${SPEC_PATH%/}"
if [[ "$SPEC_PATH" == *.md ]]; then
    SPEC_DIR=$(dirname "$SPEC_PATH")
else
    SPEC_DIR="$SPEC_PATH"
fi

# 6. Check that 05_acceptance.md exists in the spec directory
ACCEPTANCE_FILE="${REPO_ROOT}/${SPEC_DIR}/05_acceptance.md"
if [ ! -f "$ACCEPTANCE_FILE" ]; then
    echo "ERROR: Acceptance document not found: ${SPEC_DIR}/05_acceptance.md"
    echo "The spec directory '${SPEC_DIR}' must contain a '05_acceptance.md' file."
    exit 1
fi

# 7. Check that 05_acceptance.md contains Status: approved
if ! grep -qF "Status: approved" "$ACCEPTANCE_FILE"; then
    echo "ERROR: Acceptance criteria are not approved in: ${SPEC_DIR}/05_acceptance.md"
    echo "The file must contain a line: Status: approved"
    exit 1
fi

echo "OK: Spec reference check passed."
exit 0
