#!/usr/bin/env bash
set -euo pipefail

PACKAGE_NAME="rail-sdk"
OLD_PACKAGE_NAME="rail-harness"
VERSION="0.1.0"
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CODEX_HOME="${CODEX_HOME:-${HOME}/.codex}"
SKILL_DIR="${CODEX_HOME}/skills/rail"

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

python - "${SKILL_DIR}" "${REPO_ROOT}" <<'PY'
from __future__ import annotations

import shutil
import sys
from importlib.resources import files
from pathlib import Path


def copy_tree(source, destination: Path) -> None:
    destination.mkdir(parents=True, exist_ok=True)
    for child in source.iterdir():
        target = destination / child.name
        if child.is_dir():
            copy_tree(child, target)
        else:
            target.write_bytes(child.read_bytes())


skill_dir = Path(sys.argv[1])
repo_root = Path(sys.argv[2])
source = repo_root / "assets" / "skill" / "Rail"

if skill_dir.exists():
    shutil.rmtree(skill_dir)
skill_dir.parent.mkdir(parents=True, exist_ok=True)

if source.is_dir():
    shutil.copytree(source, skill_dir)
else:
    copy_tree(files("rail").joinpath("package_assets", "skill", "Rail"), skill_dir)

print(f"Rail skill installed at {skill_dir}")
PY

python - <<'PY'
from importlib.metadata import version

import rail

print(f"rail-sdk {version('rail-sdk')} installed; import rail OK")
PY

if [[ -z "${OPENAI_API_KEY:-}" ]]; then
  echo "Set OPENAI_API_KEY before running live Rail tasks."
else
  echo "OPENAI_API_KEY detected; live Actor Runtime will enable automatically."
fi

echo "Migration complete. Use the Rail skill with a target repository path and a natural-language task."
