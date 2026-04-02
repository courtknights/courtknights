# infra/Makefile

.PHONY: apply-rulesets

GITHUB_REPO ?= courtknights/courtknights

# apply-rulesets — create or update GitHub branch protection rulesets.
# Requires a token with administration: write permission (fine-grained PAT or GitHub App token).
# Override the target repository with: make apply-rulesets GITHUB_REPO=org/repo
apply-rulesets:
	@bash "$(CURDIR)/apply_rulesets.sh" "$(GITHUB_REPO)"
