#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_DIR="${HOME}/.codex/skills/rail"

mkdir -p "${TARGET_DIR}"
ln -sfn "${ROOT_DIR}/skills/rail/SKILL.md" "${TARGET_DIR}/SKILL.md"

echo "Installed rail skill symlink at ${TARGET_DIR}/SKILL.md"
