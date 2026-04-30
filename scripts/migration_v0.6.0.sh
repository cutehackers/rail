#!/usr/bin/env bash
set -euo pipefail

PACKAGE_NAME="rail-sdk"
OLD_PACKAGE_NAME="rail-harness"
VERSION="0.6.0"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CODEX_HOME="${CODEX_HOME:-${HOME}/.codex}"

if ! command -v uv >/dev/null 2>&1; then
  echo "uv is required. Install uv, then rerun this migration." >&2
  exit 1
fi

if uv pip show "${OLD_PACKAGE_NAME}" >/dev/null 2>&1; then
  uv pip uninstall "${OLD_PACKAGE_NAME}"
fi

wheel=""
if compgen -G "${REPO_ROOT}/dist/rail_sdk-${VERSION}-*.whl" >/dev/null; then
  wheel="$(ls "${REPO_ROOT}"/dist/rail_sdk-"${VERSION}"-*.whl | head -n 1)"
fi

if [[ -n "${RAIL_SDK_WHEEL:-}" ]]; then
  uv pip install "${RAIL_SDK_WHEEL}"
elif [[ -n "${wheel}" ]]; then
  uv pip install "${wheel}"
else
  uv pip install "${PACKAGE_NAME}==${VERSION}"
fi

echo "Rail skill destination: ${CODEX_HOME}/skills/rail"
python -m rail.cli.main migrate --codex-home "${CODEX_HOME}"

python - <<'PY'
from importlib.metadata import version

import rail

print(f"rail-sdk {version('rail-sdk')} installed; import rail OK")
PY

echo "Set OPENAI_API_KEY before live Rail tasks; rail doctor checks it."
echo "Migration complete. Use the Rail skill from the target repository."
