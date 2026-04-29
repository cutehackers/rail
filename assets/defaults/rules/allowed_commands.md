# Allowed Commands

The executor should follow `.harness/supervisor/execution_policy.yaml` and stay scoped to the smallest credible validation set.

The default Rail profile uses this order for standard validation:

1. `uv run --python 3.12 pytest -q`
2. `uv run --python 3.12 ruff check src tests`
3. `uv run --python 3.12 mypy src/rail`

Disallow by default:
- destructive git commands (`git reset --hard`, `git checkout -- <path>`)
- unrelated dependency updates
- ad-hoc global tool installation during validation
