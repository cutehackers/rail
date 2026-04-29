#!/usr/bin/env bash
set -euo pipefail

uv run --python 3.12 pytest -q
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail
