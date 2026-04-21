#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# Parse the requested release tag and execution mode.
usage() {
  cat >&2 <<'EOF'
Usage: tool/prepare_release.sh vX.Y.Z [--preflight-only] [--allow-existing-tag] [--push]

Checks release readiness for a tag. The release commit must already contain:
  - CHANGELOG.md section: ## vX.Y.Z
  - packaging/homebrew/rail.rb tag/version for vX.Y.Z

Options:
  --preflight-only      Run only fast release-input checks.
  --allow-existing-tag  Allow the tag to already exist, for tag-triggered CI.
  --push                After full verification, push main and create/push tag.
EOF
}

version="${1:-}"
if [[ -z "$version" ]]; then
  usage
  exit 1
fi
shift

preflight_only=false
allow_existing_tag=false
push_release=false

while (($#)); do
  case "$1" in
    --preflight-only)
      preflight_only=true
      ;;
    --allow-existing-tag)
      allow_existing_tag=true
      ;;
    --push)
      push_release=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown option: $1" >&2
      usage
      exit 1
      ;;
  esac
  shift
done

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release version: expected vX.Y.Z, got '$version'" >&2
  exit 1
fi

cd "$REPO_ROOT"

# Validate release environment and repository context before inspecting files.
if [[ -z "${HOMEBREW_TAP_GITHUB_TOKEN:-}" ]]; then
  echo "HOMEBREW_TAP_GITHUB_TOKEN is required for release" >&2
  exit 1
fi

if [[ "$(git rev-parse --is-inside-work-tree)" != "true" ]]; then
  echo "not inside a git work tree" >&2
  exit 1
fi

current_branch=$(git branch --show-current)
if [[ "$current_branch" != "main" && ( "$push_release" == "true" || "$preflight_only" == "false" ) ]]; then
  echo "release publish must run from main" >&2
  exit 1
fi

# Prevent accidental republishing of an existing release tag.
if ! $allow_existing_tag; then
  if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
    echo "release tag already exists locally: $version" >&2
    exit 1
  fi
  if git ls-remote --exit-code --tags origin "refs/tags/$version" >/dev/null 2>&1; then
    echo "release tag already exists on origin: $version" >&2
    exit 1
  fi
fi

# Ensure the release commit already contains the human-facing release record.
if ! grep -Eq "^## ${version}([[:space:]-]|$)" CHANGELOG.md; then
  echo "missing CHANGELOG.md section for $version" >&2
  exit 1
fi

# Ensure the source Homebrew formula points at this exact release.
GITHUB_REF_NAME="$version" ./tool/verify_release_formula.sh

# Ensure release archives will include the bundled Rail skill surface.
for required_asset in \
  "assets/skill/Rail/SKILL.md" \
  "assets/skill/Rail/references/examples.md"
do
  if ! grep -Fq "$required_asset" .goreleaser.yaml; then
    echo "GoReleaser archive is missing required skill asset: $required_asset" >&2
    exit 1
  fi
done

if grep -Fq "assets/skill/Rail/**/*" .goreleaser.yaml; then
  echo "GoReleaser archive uses incomplete recursive skill glob" >&2
  exit 1
fi

# Generated release output must never be committed or pending in git.
if [[ -n "$(git ls-files dist)" || -n "$(git status --porcelain -- dist 2>/dev/null)" ]]; then
  echo "dist/ must not be tracked or pending in git" >&2
  exit 1
fi

# Release preparation must start from a clean commit.
if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree is dirty; commit release changes before preparing a release" >&2
  exit 1
fi

# CI tag jobs use the fast preflight path because GoReleaser does the build.
if $preflight_only; then
  echo "release preflight passed for $version"
  exit 0
fi

# Run local release verification before any tag is created.
go test ./...
go build -o build/rail ./cmd/rail
ruby -c packaging/homebrew/rail.rb
ruby -e 'require "yaml"; YAML.load_file(".goreleaser.yaml"); YAML.load_file(".github/workflows/release.yml")'
go run github.com/goreleaser/goreleaser/v2@latest check

cleanup_dist() {
  rm -rf "$REPO_ROOT/dist"
}
trap cleanup_dist EXIT

# Build a snapshot release locally and inspect the generated artifacts.
go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish

shopt -s nullglob
archives=(dist/*.tar.gz)
if ((${#archives[@]} == 0)); then
  echo "GoReleaser snapshot produced no archives" >&2
  exit 1
fi

for archive in "${archives[@]}"; do
  contents=$(tar -tzf "$archive")
  for required_path in \
    "rail" \
    "README.md" \
    "assets/skill/Rail/SKILL.md" \
    "assets/skill/Rail/references/examples.md"
  do
    if ! grep -Fxq "$required_path" <<<"$contents"; then
      echo "archive $archive is missing $required_path" >&2
      exit 1
    fi
  done
done

if ! grep -Fq 'cp_r (pkgshare/"skill/Rail").children, codex_skill_dir' dist/homebrew/Formula/rail.rb; then
  echo "generated Homebrew formula does not copy Codex skill assets from pkgshare" >&2
  exit 1
fi

cleanup_dist
trap - EXIT

# Publish only after all local checks and artifact checks have passed.
if $push_release; then
  git push origin main
  git tag "$version"
  git push origin "$version"
else
  cat <<EOF
release checks passed for $version

To publish:
  git push origin main
  git tag $version
  git push origin $version
EOF
fi
