# Allowed Commands

The executor should follow `.harness/supervisor/execution_policy.yaml` and stay scoped to the smallest credible validation set.

The default Rail profile uses this order for standard validation:

1. `gofmt -w <changed files>`
2. `uv run --python 3.12 pytest -q`
3. `uv run --python 3.12 ruff check src tests`

Disallow by default:
- destructive git commands (`git reset --hard`, `git checkout -- <path>`)
- unrelated dependency updates
- ad-hoc global tool installation during validation
