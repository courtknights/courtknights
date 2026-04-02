#!/usr/bin/env bash
set -euo pipefail

# apply_rulesets.sh — Idempotent GitHub ruleset applier
#
# Creates or updates each ruleset defined in infra/rulesets/*.json.
# Bypass actors are loaded from infra/rulesets/owners.json and injected
# into every ruleset at apply time — do not hardcode bypass_actors in the
# individual ruleset files.
# Matches by ruleset name: creates if absent, updates if already present.
#
# Usage:
#   bash infra/rulesets/apply_rulesets.sh [GITHUB_REPO]
#
# Arguments:
#   GITHUB_REPO — owner/repo (default: courtknights/courtknights)
#
# Requirements:
#   - gh CLI authenticated with a token that has `administration: write` permission

GITHUB_REPO="${1:-courtknights/courtknights}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RULESET_DIR="${SCRIPT_DIR}"
OWNERS_FILE="${SCRIPT_DIR}/owners.json"

for ruleset_file in "${RULESET_DIR}"/*.json; do
    [[ "$(basename "${ruleset_file}")" == "owners.json" ]] && continue

    name=$(python3 -c "import json; print(json.load(open('${ruleset_file}'))['name'])")

    merged=$(python3 -c "
import json
ruleset = json.load(open('${ruleset_file}'))
owners = json.load(open('${OWNERS_FILE}'))
ruleset['bypass_actors'] = [
    {'actor_id': o['actor_id'], 'actor_type': o['actor_type'], 'bypass_mode': 'pull_request'}
    for o in owners
]
print(json.dumps(ruleset))
")

    existing_id=$(gh api "repos/${GITHUB_REPO}/rulesets" \
        --jq ".[] | select(.name == \"${name}\") | .id" 2>/dev/null || true)

    if [[ -n "${existing_id}" ]] && ! [[ "${existing_id}" =~ ^[0-9]+$ ]]; then
        echo "ERROR: could not retrieve rulesets for '${GITHUB_REPO}'." >&2
        echo "       Ensure the repository is public or the account has GitHub Pro." >&2
        exit 1
    fi

    if [ -n "$existing_id" ]; then
        echo "Updating ruleset '${name}' (id: ${existing_id})..."
        echo "${merged}" | gh api "repos/${GITHUB_REPO}/rulesets/${existing_id}" \
            --method PUT \
            --input -
        echo "OK: ruleset '${name}' updated."
    else
        echo "Creating ruleset '${name}'..."
        echo "${merged}" | gh api "repos/${GITHUB_REPO}/rulesets" \
            --method POST \
            --input -
        echo "OK: ruleset '${name}' created."
    fi
done
