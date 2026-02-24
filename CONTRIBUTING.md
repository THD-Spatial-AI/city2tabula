# Contributing

Thanks for contributing to **City2TABULA**.

We welcome:

- bug reports
- feature requests
- documentation improvements
- code changes
- reviews and discussion

## Code of Conduct

By participating, you agree to follow the rules in [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).

## Where to post

- **Questions / “how do I…?” / ideas:** use [Discussions](https://github.com/THD-Spatial/city2tabula/discussions)
- **Bugs / tasks / actionable requests:** open an [Issue](https://github.com/THD-Spatial/city2tabula/issues)

If you’re not sure, start a Discussion. We’ll convert it to an Issue if needed.

## Reporting bugs

A good bug report makes the problem reproducible.

Please include:

- what you expected vs what happened
- exact steps to reproduce
- logs / error output (redact secrets)
- your environment: OS, Go version, Postgres/PostGIS versions (if relevant)
- City2TABULA version (`city2tabula --version`)

## Proposing changes

For medium/large changes, open a Discussion or Issue first so we agree on scope before you spend time.

## Development workflow

### 1) Fork and clone (if you don’t have write access)

```bash
git clone https://github.com/THD-Spatial/city2tabula.git
cd city2tabula
```

### 2) Create a branch

Use a short, descriptive name:

```bash
git checkout -b type/short-description
```

Examples:

- `fix/roof-azimuth-validation`
- `feat/version-flag`
- `docs/mkdocs-setup`

### 3) Make changes (keep them focused)

Small PRs are easier to review. If the change is large, split it into logical steps.

### 4) Test (where applicable)

At minimum:

- ensure it builds/runs locally
- run relevant tests if the project provides them
- update docs if behaviour or CLI changes

### 5) Commit and push

```bash
git add .
git commit -m "Concise summary of change"
git push -u origin <your-branch-name>
```

### 6) Open a pull request

In the PR description, include:

- what changed and why
- how you tested it
- links to related Issue(s) (e.g. `Closes #123`)

## Pull request checklist

- [ ] Scope is clear and reasonable
- [ ] I tested the change (or explained why not)
- [ ] Docs were updated (if behaviour changed)
- [ ] No secrets or private data are included
- [ ] Related issues are linked (if applicable)

## Documentation contributions

Docs changes are valuable. Please:

- keep wording practical and to the point
- include short examples where helpful
- check commands and links

## Licensing

By contributing, you confirm you have the right to submit the work and agree that it is licensed under the repository’s licence.

## Need help?

Open a Discussion with:

- your goal
- your dataset (CityGML/CityJSON + LoD)
- what you’ve tried so far
