#!/usr/bin/env python3
"""check_adr_consistency.py — ADR consistency check via Claude API.

Reads the PR diff and all ADR files from the current branch, sends them to
claude-sonnet-4-6, and acts on the structured JSON response:
  - Applies 'adr-change' label if ADR files were modified.
  - Posts a PR comment and exits 1 on BLOCKING violations.
  - Posts a PR comment and exits 0 on WARNING-only violations.
  - Exits 0 silently when no violations are found.

Required environment variables:
  PR_NUMBER          — pull request number
  GITHUB_TOKEN       — GitHub token (needs pull-requests: write, contents: read)
  ANTHROPIC_API_KEY  — Anthropic API key
  GITHUB_REPOSITORY  — owner/repo  (e.g. courtknights/courtknights)
"""

import json
import os
import re
import subprocess
import sys
from pathlib import Path

import anthropic
from github import Github, GithubException

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

MODEL = "claude-sonnet-4-6"
MAX_TOKENS = 4096

SYSTEM_PROMPT = """\
You are an architecture consistency reviewer for the CourtKnights project.
Your task is to detect whether the provided PR diff contradicts any of the
active Architecture Decision Records (ADRs).

Classify each finding as:
- BLOCKING: a direct, unambiguous violation of a decision stated in an ADR.
- WARNING:  an area of concern that may need human attention but is not a
            clear-cut violation.

Respond ONLY with a JSON object matching this schema:
{
  "violations": [
    {
      "severity": "BLOCKING" | "WARNING",
      "adr": "ADR-NNN",
      "summary": "<one-line description>",
      "diff_excerpt": "<relevant lines from the diff>",
      "suggestion": "<concrete action to resolve the conflict>"
    }
  ],
  "adr_files_modified": ["ADR-NNN", ...]
}

If there are no violations, return { "violations": [], "adr_files_modified": [...] }.
Do not include any text outside the JSON object."""

ADR_CHANGE_LABEL = "adr-change"


# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

def require_env(name: str) -> str:
    value = os.environ.get(name)
    if not value:
        print(f"ERROR: environment variable {name} is required but not set.", file=sys.stderr)
        sys.exit(1)
    return value


def get_repo_root() -> Path:
    result = subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        capture_output=True, text=True, check=True,
    )
    return Path(result.stdout.strip())


def get_pr_diff(pr_number: str) -> str:
    result = subprocess.run(
        ["gh", "pr", "diff", pr_number],
        capture_output=True, text=True, check=True,
    )
    return result.stdout


def read_adr_files(repo_root: Path) -> dict[str, str]:
    """Return {filename: content} for all ADR-*.md files in docs/decisions/."""
    adrs: dict[str, str] = {}
    for path in sorted(repo_root.glob("docs/decisions/ADR-*.md")):
        adrs[path.name] = path.read_text(encoding="utf-8")
    return adrs


def call_claude(adrs: dict[str, str], diff: str) -> dict:
    """Send the ADR corpus and PR diff to Claude; return parsed JSON."""
    client = anthropic.Anthropic()

    adr_section = "\n\n".join(
        f"### {name}\n\n{content}" for name, content in adrs.items()
    )
    user_message = (
        f"## Active ADRs\n\n{adr_section}\n\n"
        f"## PR diff\n\n```diff\n{diff}\n```"
    )

    response = client.messages.create(
        model=MODEL,
        max_tokens=MAX_TOKENS,
        system=SYSTEM_PROMPT,
        messages=[{"role": "user", "content": user_message}],
    )

    raw = response.content[0].text
    # Strip optional markdown code fences (```json ... ``` or ``` ... ```)
    stripped = re.sub(r"^```(?:json)?\s*", "", raw.strip(), flags=re.IGNORECASE)
    stripped = re.sub(r"\s*```$", "", stripped)
    if stripped != raw.strip():
        print(
            "WARNING: Claude wrapped its response in markdown code fences. "
            "This violates the response contract defined in ADR-011 "
            "(Claude must return only a JSON object, no free-form text). "
            "Proceeding with parsing after stripping the fences.",
            file=sys.stderr,
        )
    try:
        return json.loads(stripped)
    except json.JSONDecodeError as exc:
        print(f"ERROR: Claude returned non-JSON output: {exc}", file=sys.stderr)
        print(f"Raw response:\n{raw}", file=sys.stderr)
        sys.exit(1)


def validate_schema(data: object) -> None:
    """Raise ValueError if the response does not match the expected schema."""
    if not isinstance(data, dict):
        raise ValueError("Response is not a JSON object")
    if "violations" not in data:
        raise ValueError("Missing 'violations' key")
    if "adr_files_modified" not in data:
        raise ValueError("Missing 'adr_files_modified' key")
    if not isinstance(data["violations"], list):
        raise ValueError("'violations' must be an array")
    if not isinstance(data["adr_files_modified"], list):
        raise ValueError("'adr_files_modified' must be an array")
    for i, v in enumerate(data["violations"]):
        if not isinstance(v, dict):
            raise ValueError(f"violations[{i}] is not an object")
        for field in ("severity", "adr", "summary", "diff_excerpt", "suggestion"):
            if field not in v:
                raise ValueError(f"violations[{i}] missing field: {field}")
        if v["severity"] not in ("BLOCKING", "WARNING"):
            raise ValueError(f"violations[{i}] invalid severity: {v['severity']!r}")


def format_comment(violations: list[dict]) -> str:
    """Build the GitHub comment body from the list of violations."""
    blocking = [v for v in violations if v["severity"] == "BLOCKING"]
    warnings  = [v for v in violations if v["severity"] == "WARNING"]

    lines: list[str] = ["## ADR Consistency Check\n"]

    if blocking:
        lines.append("### Blocking violations\n")
        for v in blocking:
            excerpt_lines = "\n".join(f"> {line}" for line in v["diff_excerpt"].splitlines())
            lines.append(
                f"**{v['adr']}**\n"
                f"> {v['summary']}\n"
                f">\n"
                f"> **Diff excerpt:**\n"
                f"> ```diff\n"
                f"{excerpt_lines}\n"
                f"> ```\n"
                f"> **Suggestion:** {v['suggestion']}\n"
            )

    if warnings:
        if blocking:
            lines.append("---\n")
        lines.append("### Warnings\n")
        for v in warnings:
            lines.append(
                f"**{v['adr']}**\n"
                f"> {v['summary']}\n"
                f">\n"
                f"> **Suggestion:** {v['suggestion']}\n"
            )

    return "\n".join(lines)


def ensure_label(repo, label_name: str) -> None:
    """Create the label in the repo if it does not already exist."""
    try:
        repo.get_label(label_name)
    except GithubException:
        repo.create_label(label_name, "0075ca")


def apply_label(pr, label_name: str) -> None:
    pr.add_to_labels(label_name)
    print(f"Label '{label_name}' applied to PR.")


def post_comment(pr, body: str) -> None:
    pr.create_issue_comment(body)
    print("Comment posted to PR.")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main() -> None:
    pr_number      = require_env("PR_NUMBER")
    github_token   = require_env("GITHUB_TOKEN")
    repo_name      = require_env("GITHUB_REPOSITORY")
    # ANTHROPIC_API_KEY is consumed automatically by the anthropic client.
    require_env("ANTHROPIC_API_KEY")

    repo_root = get_repo_root()

    # 1. Gather inputs
    print("=== Fetching PR diff ===")
    diff = get_pr_diff(pr_number)
    if not diff.strip():
        print("PR diff is empty — nothing to check.")
        sys.exit(0)

    print("=== Reading ADR files ===")
    adrs = read_adr_files(repo_root)
    if not adrs:
        print("No ADR files found — nothing to check.")
        sys.exit(0)
    print(f"Found {len(adrs)} ADR(s): {', '.join(adrs)}")

    # 2. Call Claude
    print(f"\n=== Calling {MODEL} ===")
    data = call_claude(adrs, diff)

    # 3. Validate schema
    try:
        validate_schema(data)
    except ValueError as exc:
        print(f"ERROR: Claude response failed schema validation: {exc}", file=sys.stderr)
        print(f"Raw data: {json.dumps(data, indent=2)}", file=sys.stderr)
        sys.exit(1)

    violations       = data["violations"]
    adrs_modified    = data["adr_files_modified"]
    blocking         = [v for v in violations if v["severity"] == "BLOCKING"]
    warnings_only    = [v for v in violations if v["severity"] == "WARNING"]

    print(f"Violations: {len(blocking)} BLOCKING, {len(warnings_only)} WARNING")
    print(f"ADR files modified: {adrs_modified or 'none'}")

    # 4. GitHub integration
    gh   = Github(github_token)
    repo = gh.get_repo(repo_name)
    pr   = repo.get_pull(int(pr_number))

    if adrs_modified:
        ensure_label(repo, ADR_CHANGE_LABEL)
        apply_label(pr, ADR_CHANGE_LABEL)

    if violations:
        comment_body = format_comment(violations)
        post_comment(pr, comment_body)

    # 5. Exit code
    if blocking:
        print("\nCoverage check FAILED — BLOCKING ADR violations found.")
        sys.exit(1)

    if warnings_only:
        print("\nADR check passed with warnings — review the PR comment.")
    else:
        print("\nADR check PASSED — no violations found.")

    sys.exit(0)


if __name__ == "__main__":
    main()
