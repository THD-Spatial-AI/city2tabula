#!/usr/bin/env python3
"""
Validate commit messages in a PR against the Conventional Commits format.

Expected format:
  <type>(<scope>): <subject>
  <type>: <subject>

Allowed types: feat, fix, docs, style, refactor, perf, test, chore
Rules:
  - Subject must use imperative mood (no "added", "fixed", etc. enforced via type)
  - Subject must not start with a capital letter
  - Subject must not end with a period
  - Full subject line must not exceed 100 characters
  - Merge commits and initial commits are skipped
"""

import re
import sys

TYPES = {"feat", "fix", "docs", "style", "refactor", "perf", "test", "chore"}
PATTERN = re.compile(
    r'^(?P<type>' + '|'.join(TYPES) + r')'
    r'(?:\((?P<scope>[^)]+)\))?'
    r': (?P<subject>.+)$'
)
MAX_LENGTH = 100

SKIP_PATTERNS = [
    re.compile(r'^Merge '),
    re.compile(r'^Initial commit', re.IGNORECASE),
    re.compile(r'^Revert '),
]


def validate(sha, message):
    errors = []

    if any(p.match(message) for p in SKIP_PATTERNS):
        return []

    if len(message) > MAX_LENGTH:
        errors.append(f"exceeds {MAX_LENGTH} characters ({len(message)})")

    match = PATTERN.match(message)
    if not match:
        errors.append(
            f"does not follow conventional commits format\n"
            f"         expected : <type>(<scope>): <subject>\n"
            f"         allowed types : {', '.join(sorted(TYPES))}\n"
            f"         example  : feat(auth): add login endpoint"
        )
        return errors

    subject = match.group("subject")
    if subject[0].isupper():
        errors.append("subject must not start with a capital letter")
    if subject.endswith("."):
        errors.append("subject must not end with a period")

    return errors


def main():
    lines = sys.stdin.read().strip().splitlines()
    if not lines:
        print("No commits to validate.")
        sys.exit(0)

    failed = []
    for line in lines:
        if not line.strip():
            continue
        sha, _, message = line.partition(" ")
        short_sha = sha[:7]
        errors = validate(sha, message)
        if errors:
            failed.append((short_sha, message, errors))

    if not failed:
        print(f"All {len(lines)} commit(s) follow the conventional commits format.")
        sys.exit(0)

    print(f"Found {len(failed)} commit(s) with invalid message(s):\n")
    for short_sha, message, errors in failed:
        print(f"  {short_sha}  \"{message}\"")
        for error in errors:
            print(f"         {error}")
        print()

    print("Fix the commit messages and push again.")
    print("See docs for the full convention: https://THD-Spatial-AI.github.io/GitHub-Template/getting-started/commit-conventions/")
    sys.exit(1)


if __name__ == "__main__":
    main()
