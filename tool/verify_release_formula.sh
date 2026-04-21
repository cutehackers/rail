#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
FORMULA_PATH="$REPO_ROOT/packaging/homebrew/rail.rb"

expected_tag=${GITHUB_REF_NAME:-}
if [[ -z "$expected_tag" ]]; then
  expected_tag=$(git -C "$REPO_ROOT" tag --sort=-v:refname | head -n 1)
fi

if [[ -z "$expected_tag" || "$expected_tag" != v* ]]; then
  echo "formula release mismatch: expected release tag must start with v, got '$expected_tag'" >&2
  exit 1
fi

expected_version=${expected_tag#v}
formula_tag=$(sed -nE 's/.*tag: "(v[^"]+)".*/\1/p' "$FORMULA_PATH" | head -n 1)
formula_version=$(sed -nE 's/.*version "([^"]+)".*/\1/p' "$FORMULA_PATH" | head -n 1)

if [[ "$formula_tag" != "$expected_tag" || "$formula_version" != "$expected_version" ]]; then
  cat >&2 <<EOF
formula release mismatch:
  expected tag:     $expected_tag
  formula tag:      ${formula_tag:-<missing>}
  expected version: $expected_version
  formula version:  ${formula_version:-<missing>}
EOF
  exit 1
fi

echo "formula release version matches $expected_tag"
