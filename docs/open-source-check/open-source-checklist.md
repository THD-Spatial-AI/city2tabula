# Open Source Readiness Checklist

Use this checklist before making a repository public under the **THD-Spatial** group.

This checklist is designed to be practical and easy to review. It separates **required**, **conditional**, and **recommended** items so you can focus on what matters first.

---

## Review Summary

| Section | Requirement Level | Status | Notes |
|---|---|---|---|
| [Essential Requirements](#essential-requirements-required) | Required | Pending | Must be completed before public release |
| [Data Management (Git LFS)](#data-management-required-if-applicable) | Required if applicable | Pending | Only if repository contains large files |
| [Optional but Recommended Files](#optional-but-recommended-files-recommended) | Recommended | Pending | Strongly encouraged for public/community repos |
| [Quality Checks](#quality-checks-required) | Required | Pending | Final review before publishing |
| [Final Steps](#final-steps-required) | Required | Pending | Verify repository works from a fresh clone |

---

## Essential Requirements (Required)

Make sure the following essential requirements are completed before making your repository public.

Quick links: [License](#license) · [README](#readme) · [Contributing Guidelines](#contributing-guidelines) · [Code of Conduct](#code-of-conduct)

### License

- [x] `LICENSE` file present in repository root
- [x] Appropriate license chosen (see [Choose a License](https://choosealicense.com/))
- [x] Correct license text added
- [x] License committed to repository

> **Important:** Without a license, you are **not allowed** to make your repository public under **THD-Spatial**.

### README

- [x] `README.md` file present in repository root

#### README content

- [x] Project title and short description included
- [x] Project purpose clearly explained
- [x] Key features listed
- [x] Installation/setup instructions provided (if applicable)
- [x] Usage examples or usage steps included
- [x] Contribution guidance referenced (e.g. `CONTRIBUTING.md`)
- [x] README uses clear Markdown formatting

### Contributing Guidelines

- [x] `CONTRIBUTING.md` file present in repository root
- [x] Issue reporting process documented
- [x] Pull request submission process documented
- [x] Coding standards / best practices outlined (if applicable)
- [x] Commit message guidance included (recommended)
- [x] `CONTRIBUTING.md` linked from `README.md`

### Code of Conduct

- [x] `CODE_OF_CONDUCT.md` file present in repository root
- [x] Code of conduct chosen (e.g. [Contributor Covenant](https://www.contributor-covenant.org/))
- [x] Contact method for reporting issues included
- [x] `CODE_OF_CONDUCT.md` linked from `README.md` or documentation

---

## Data Management (Required if applicable)

### Git LFS (Large File Storage)

Complete this section if your repository contains large files (e.g. datasets, binaries, media, generated assets).

- [x] Git LFS installed locally
- [x] Git LFS initialised in repository (`git lfs install`)
- [x] Large file types tracked (e.g. data files, media, binaries)
- [x] `.gitattributes` committed to repository
- [x] Large files added and committed properly
- [x] Release strategy for LFS files documented (if applicable)

> **Note:** Files tracked by Git LFS are not included in release assets by default.
> If needed, enable **Include Git LFS objects in archives** in repository settings.

---

## Optional but Recommended Files (Recommended)

These files are not always required, but they improve project quality, collaboration, and maintainability.

### CHANGELOG

- [ ] `CHANGELOG.md` file created (if using versioned releases)
- [ ] Version history documented
- [ ] Changes formatted consistently (e.g. [Keep a Changelog](https://keepachangelog.com/))

### Issue and PR Templates

- [x] `.github/ISSUE_TEMPLATE/` directory created (if using issue templates)
- [x] Issue template(s) added
- [ ] `.github/pull_request_template.md` file added
- [ ] PR template includes a contributor checklist (recommended)

### Code Owners

- [ ] `CODEOWNERS` file created (root or `.github/`)
- [ ] Owners assigned for relevant parts of the repository
- [ ] Teams/individuals correctly specified

### Security Policy

- [ ] `SECURITY.md` file created
- [ ] Vulnerability reporting process documented
- [ ] Contact method for security issues provided
- [ ] Supported versions documented (if applicable)

### Support Guidelines

- [ ] `SUPPORT.md` file created
- [ ] Support channels documented (e.g. email, discussions, issue tracker)
- [ ] Response expectations documented (if applicable)

---

## Quality Checks (Required)

### Documentation Quality

- [x] Documentation reviewed for clarity
- [x] Spelling and grammar checked
- [x] Links tested and working
- [x] Code examples/commands tested (if applicable)

### Repository Settings

- [x] Repository description added on GitHub
- [x] Topics/tags added for discoverability
- [x] Repository visibility set to **Public**
- [x] Default branch configured (typically `main`)
- [x] Branch protection rules configured (if needed)

### Code Quality / Safety

- [x] Code reviewed and cleaned up
- [x] Sensitive information removed (API keys, passwords, credentials)
- [x] `.gitignore` configured properly
- [x] Dependencies documented
- [x] Build/test instructions included (if applicable)

---

## Final Steps (Required)

- [x] All essential requirements completed
- [x] Repository tested by cloning a fresh copy
- [x] Links and references verified
- [x] Project builds/runs successfully from scratch (if applicable)
- [x] Team members notified about repository availability (if applicable)

---

## Repository Ready for Open Source

Once all required items are completed, the repository meets the THD-Spatial minimum readiness standard for public release.

- **Date completed:** `DD.MM.YYYY`
- **Reviewed by:** `Name(s)`

---

## Need Help?

If you have questions about any checklist item:

- Review the template `README.md`
- Check the documentation pages in `docs/`
- Ask the THD-Spatial maintainers/reviewers