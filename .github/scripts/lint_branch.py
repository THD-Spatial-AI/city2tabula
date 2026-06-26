#!/usr/bin/env python3
"""
Validate branch name against the Conventional Branch naming convention.
Extended with additional types to mirror commit convention types.

Valid format: <type>/<description>
  - type     : one of the allowed prefixes
  - description : lowercase letters, digits, hyphens, dots (for versions)
                  no consecutive hyphens/dots, no leading/trailing hyphens/dots

Exempt branches: main, master, dev, develop, staging
"""

import re
import sys

EXEMPT = {"main", "master", "dev", "develop", "staging"}

TYPES = {
    "feat", "feature",   # new features
    "fix", "bugfix",     # bug fixes
    "hotfix",            # urgent production fixes
    "release",           # release preparation
    "chore",             # tooling, dependencies, CI
    "docs",              # documentation
    "refactor",          # code restructuring
    "test",              # tests only
    "style",             # formatting, no logic changes
    "perf",              # performance improvements
}

# description: lowercase, digits, hyphens, dots — no consecutive or edge separators
DESCRIPTION_RE = re.compile(r'^[a-z0-9]([a-z0-9]|(-(?!-))|(\\.(?!\\.)))*[a-z0-9]$|^[a-z0-9]$')

TYPE_LIST = ", ".join(sorted(TYPES))


def validate(branch):
    if branch in EXEMPT:
        return []

    errors = []

    if "/" not in branch:
        errors.append(
            f"missing type prefix\n"
            f"         expected  : <type>/<description>\n"
            f"         allowed types : {TYPE_LIST}\n"
            f"         example   : feat/add-login-page"
        )
        return errors

    prefix, _, description = branch.partition("/")

    if prefix not in TYPES:
        errors.append(
            f"unknown type '{prefix}'\n"
            f"         allowed types : {TYPE_LIST}"
        )

    if not description:
        errors.append("description is empty after the '/'")
    else:
        if description != description.lower():
            errors.append("description must be lowercase")
        if "_" in description:
            errors.append("underscores are not allowed — use hyphens instead")
        if " " in description:
            errors.append("spaces are not allowed — use hyphens instead")
        if "--" in description or ".." in description:
            errors.append("consecutive hyphens or dots are not allowed")
        if description.startswith("-") or description.endswith("-"):
            errors.append("description must not start or end with a hyphen")
        if description.startswith(".") or description.endswith("."):
            errors.append("description must not start or end with a dot")

    return errors


def main():
    if len(sys.argv) < 2:
        print("Usage: lint_branch.py <branch-name>")
        sys.exit(1)

    branch = sys.argv[1].strip()
    errors = validate(branch)

    if not errors:
        print(f"Branch name '{branch}' is valid.")
        sys.exit(0)

    print(f"Invalid branch name: '{branch}'\n")
    for error in errors:
        print(f"  {error}")
    print()
    print("See docs for the full convention: https://THD-Spatial-AI.github.io/GitHub-Template/getting-started/branch-naming/")
    sys.exit(1)


if __name__ == "__main__":
    main()
