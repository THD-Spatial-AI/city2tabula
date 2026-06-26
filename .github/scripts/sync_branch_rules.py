#!/usr/bin/env python3
"""
Sync branch protection rules for 'main' and 'dev' across all org repositories.

Rules applied:
  main — require PR + 1 approval, dismiss stale reviews, require CI to pass,
          require branch up-to-date, admins can bypass
  dev  — require PR + 1 approval, require CI to pass, admins can bypass

Opt-out: add 'branch-rules' or '*' to .github/.templatesync-ignore in a repo.
"""

import os
import sys
import requests

API = "https://api.github.com"
TOKEN = os.environ["SYNC_PAT"]
ORGS = [o.strip() for o in os.environ.get("ORGS", "THD-Spatial-AI,enerplanet").split(",")]
DRY_RUN = os.environ.get("DRY_RUN", "false").lower() == "true"
TEMPLATE_REPO = os.environ.get("TEMPLATE_REPO", "")
# Comma-separated CI check names, e.g. "build,test" — empty means no CI requirement
RAW_CHECKS = os.environ.get("REQUIRED_STATUS_CHECKS", "").strip()
REQUIRED_CHECKS = [c.strip() for c in RAW_CHECKS.split(",") if c.strip()]

IGNORE_FILE = ".github/.templatesync-ignore"
IGNORE_KEY = "branch-rules"

HEADERS = {
    "Authorization": f"Bearer {TOKEN}",
    "Accept": "application/vnd.github+json",
    "X-GitHub-Api-Version": "2022-11-28",
}


# ---------------------------------------------------------------------------
# GitHub API helpers
# ---------------------------------------------------------------------------

def gh_get(path, params=None):
    resp = requests.get(f"{API}{path}", headers=HEADERS, params=params)
    resp.raise_for_status()
    return resp.json()


def gh_put(path, data):
    resp = requests.put(f"{API}{path}", headers=HEADERS, json=data)
    resp.raise_for_status()
    return resp.json()


def gh_delete(path):
    resp = requests.delete(f"{API}{path}", headers=HEADERS)
    resp.raise_for_status()


def get_ignore_list(repo):
    try:
        import base64
        data = gh_get(f"/repos/{repo}/contents/{IGNORE_FILE}")
        content = base64.b64decode(data["content"]).decode("utf-8")
        return {l.strip() for l in content.splitlines() if l.strip() and not l.startswith("#")}
    except requests.HTTPError as e:
        if e.response.status_code in (403, 404):
            return set()
        raise


def branch_exists(repo, branch):
    try:
        gh_get(f"/repos/{repo}/branches/{branch}")
        return True
    except requests.HTTPError as e:
        if e.response.status_code == 404:
            return False
        raise


def list_repos(org):
    repos, page = [], 1
    while True:
        batch = gh_get(f"/orgs/{org}/repos", params={"per_page": 100, "page": page, "type": "all"})
        if not batch:
            break
        repos.extend(batch)
        page += 1
    return repos


# ---------------------------------------------------------------------------
# Protection rule payloads
# ---------------------------------------------------------------------------

def status_checks_payload(strict=True):
    if not REQUIRED_CHECKS:
        return None
    return {"strict": strict, "contexts": REQUIRED_CHECKS}


MAIN_RULES = {
    "required_status_checks": status_checks_payload(strict=True),
    "enforce_admins": False,          # admins can bypass
    "required_pull_request_reviews": {
        "dismiss_stale_reviews": True,
        "require_code_owner_reviews": False,
        "required_approving_review_count": 1,
    },
    "restrictions": None,
    "required_linear_history": False,
    "allow_force_pushes": False,
    "allow_deletions": False,
    "required_conversation_resolution": True,
}

DEV_RULES = {
    "required_status_checks": status_checks_payload(strict=False),
    "enforce_admins": False,          # admins can bypass
    "required_pull_request_reviews": {
        "dismiss_stale_reviews": False,
        "require_code_owner_reviews": False,
        "required_approving_review_count": 1,
    },
    "restrictions": None,
    "required_linear_history": False,
    "allow_force_pushes": False,
    "allow_deletions": False,
    "required_conversation_resolution": False,
}


# ---------------------------------------------------------------------------
# Per-repo processing
# ---------------------------------------------------------------------------

def apply_protection(repo, branch, rules):
    if DRY_RUN:
        print(f"    [dry-run] would protect: {branch}")
        return
    gh_put(f"/repos/{repo}/branches/{branch}/protection", rules)
    print(f"    protected: {branch}")


def process_repo(repo):
    full = repo["full_name"]
    print(f"\n  {full}")

    if repo.get("archived"):
        print("    skip: archived")
        return
    if repo.get("fork"):
        print("    skip: fork")
        return
    if full == TEMPLATE_REPO:
        print("    skip: template repo")
        return

    ignored = get_ignore_list(full)
    if "*" in ignored or IGNORE_KEY in ignored:
        print("    skip: opted out")
        return

    apply_protection(full, "main", MAIN_RULES)

    if branch_exists(full, "dev"):
        apply_protection(full, "dev", DEV_RULES)
    else:
        print("    skip dev: branch does not exist")


def main():
    print(f"Organizations      : {', '.join(ORGS)}")
    print(f"Required CI checks : {REQUIRED_CHECKS or '(none)'}")
    print(f"Dry run            : {DRY_RUN}")

    errors = []
    for org in ORGS:
        print(f"\n=== {org} ===")
        repos = list_repos(org)
        print(f"  {len(repos)} repositories")
        for repo in repos:
            try:
                process_repo(repo)
            except Exception as exc:
                msg = f"  ERROR {repo['full_name']}: {exc}"
                print(msg)
                errors.append(msg)

    if errors:
        print("\nFailed repos:")
        for e in errors:
            print(e)
        sys.exit(1)


if __name__ == "__main__":
    main()
