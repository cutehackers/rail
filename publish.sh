#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage:
  ./publish.sh vX.Y.Z

Before running, add the release notes at the top of CHANGELOG.md:
  ## vX.Y.Z - YYYY-MM-DD

The script updates pyproject.toml and uv.lock, runs the release gate, commits
release metadata changes when needed, pushes main, and pushes the release tag.
USAGE
}

if [[ $# -ne 1 ]]; then
  usage
  exit 2
fi

TAG="$1"
if [[ ! "${TAG}" =~ ^v[0-9]+[.][0-9]+[.][0-9]+([.-][0-9A-Za-z][0-9A-Za-z.-]*)?$ ]]; then
  echo "Release tag must look like v0.6.1." >&2
  exit 2
fi
VERSION="${TAG#v}"

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

cleanup() {
  rm -rf dist src/rail_sdk.egg-info src/rail_harness.egg-info
}
trap cleanup EXIT

assert_only_release_files_are_dirty() {
  local status path
  status="$(git status --porcelain)"
  [[ -z "${status}" ]] && return 0

  while IFS= read -r line; do
    [[ -z "${line}" ]] && continue
    if [[ "${line}" == R* || "${line}" == C* ]]; then
      echo "Refusing to publish with renamed or copied file: ${line}" >&2
      echo "Commit, stash, or revert unrelated changes first." >&2
      exit 1
    fi
    path="${line:3}"
    case "${path}" in
      CHANGELOG.md|pyproject.toml|uv.lock) ;;
      *)
        echo "Refusing to publish with unrelated dirty file: ${path}" >&2
        echo "Commit, stash, or revert unrelated changes first." >&2
        exit 1
        ;;
    esac
  done <<< "${status}"
}

assert_changelog_top_entry() {
  python3 - "${TAG}" <<'PY'
from pathlib import Path
import sys

tag = sys.argv[1]
for line in Path("CHANGELOG.md").read_text(encoding="utf-8").splitlines():
    if not line.startswith("## v"):
        continue
    if line.startswith(f"## {tag} - "):
        raise SystemExit(0)
    raise SystemExit(
        f"Top CHANGELOG entry must be {tag}. Found: {line}. "
        "Add the new release notes at the top before publishing."
    )
raise SystemExit("CHANGELOG.md has no release entry.")
PY
}

update_pyproject_version() {
  python3 - "${VERSION}" <<'PY'
from pathlib import Path
import re
import sys

version = sys.argv[1]
path = Path("pyproject.toml")
text = path.read_text(encoding="utf-8")
updated, count = re.subn(
    r'(?m)^version = "[^"]+"$',
    f'version = "{version}"',
    text,
    count=1,
)
if count != 1:
    raise SystemExit("Could not update pyproject.toml project version.")
path.write_text(updated, encoding="utf-8")
PY
}

echo "Preparing ${TAG}."
assert_only_release_files_are_dirty
assert_changelog_top_entry

git fetch origin main --tags
if ! git merge-base --is-ancestor origin/main HEAD; then
  echo "origin/main is not an ancestor of HEAD; refusing non-fast-forward publish." >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null; then
  echo "Local tag already exists: ${TAG}" >&2
  exit 1
fi
if git ls-remote --exit-code --tags origin "refs/tags/${TAG}" >/dev/null 2>&1; then
  echo "Remote tag already exists: ${TAG}" >&2
  exit 1
fi

update_pyproject_version
uv lock
python3 scripts/check_release_metadata.py "${TAG}" --pyproject pyproject.toml --changelog CHANGELOG.md

scripts/release_gate.sh

assert_only_release_files_are_dirty
git add CHANGELOG.md pyproject.toml uv.lock
if ! git diff --cached --quiet; then
  git commit -m "chore: prepare ${TAG} release"
fi

git push origin HEAD:main
git tag "${TAG}"
git push origin "${TAG}"

echo "Published ${TAG}. GitHub Actions publish workflow will upload to PyPI."
