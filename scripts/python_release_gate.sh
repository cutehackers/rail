#!/usr/bin/env bash
set -euo pipefail

cleanup() {
  rm -rf src/rail_sdk.egg-info src/rail_harness.egg-info
}
trap cleanup EXIT

rm -rf dist src/rail_sdk.egg-info src/rail_harness.egg-info
uv run --python 3.12 python scripts/check_package_asset_alignment.py
uv build
uv run --python 3.12 python scripts/check_python_package_assets.py dist
uv run --python 3.12 python scripts/check_installed_wheel.py dist
uv run --python 3.12 pytest -q --ignore=tests/e2e/test_optional_live_sdk_smoke.py
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail

if [[ "${RAIL_ACTOR_RUNTIME_LIVE_SMOKE:-0}" == "1" ]]; then
  if [[ -z "${OPENAI_API_KEY:-}" ]]; then
    echo "RAIL_ACTOR_RUNTIME_LIVE_SMOKE requires OPENAI_API_KEY" >&2
    exit 1
  fi
  export RAIL_ACTOR_RUNTIME_LIVE=1
  uv run --python 3.12 pytest tests/e2e/test_optional_live_sdk_smoke.py -q
else
  echo "Skipping optional live SDK smoke; set RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1 to opt in."
fi
