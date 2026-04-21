#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

usage() {
  cat >&2 <<'EOF'
Usage: tool/publish.sh vX.Y.Z [--local-only]

Prepares a release pull request by:
  - creating release/vX.Y.Z from main
  - adding a CHANGELOG.md section for vX.Y.Z
  - updating packaging/homebrew/rail.rb to vX.Y.Z
  - committing with a Conventional Commit message
  - pushing the branch and opening a PR to main

After the PR merges, publish the release with:
  HOMEBREW_TAP_GITHUB_TOKEN=... tool/prepare_release.sh vX.Y.Z --push

Options:
  --local-only  Create the local branch and commit, but do not push or open a PR.
EOF
}

version="${1:-}"
if [[ -z "$version" ]]; then
  usage
  exit 1
fi
shift

local_only=false
while (($#)); do
  case "$1" in
    --local-only)
      local_only=true
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

if [[ "$(git rev-parse --is-inside-work-tree)" != "true" ]]; then
  echo "not inside a git work tree" >&2
  exit 1
fi

if [[ "$(git branch --show-current)" != "main" ]]; then
  echo "publish must start from main" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree is dirty; commit or stash changes before publishing" >&2
  exit 1
fi

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is required to open the release pull request" >&2
  exit 1
fi

if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
  echo "release tag already exists locally: $version" >&2
  exit 1
fi

if git ls-remote --exit-code --tags origin "refs/tags/$version" >/dev/null 2>&1; then
  echo "release tag already exists on origin: $version" >&2
  exit 1
fi

branch="release/${version}"
if git rev-parse -q --verify "refs/heads/$branch" >/dev/null; then
  echo "release branch already exists locally: $branch" >&2
  exit 1
fi

if git ls-remote --exit-code --heads origin "$branch" >/dev/null 2>&1; then
  echo "release branch already exists on origin: $branch" >&2
  exit 1
fi

if grep -Eq "^## ${version}([[:space:]-]|$)" CHANGELOG.md; then
  echo "CHANGELOG.md already contains a section for $version" >&2
  exit 1
fi

git checkout -b "$branch"

release_date=$(date +%F)
changelog_section=$(cat <<EOF
## ${version} - ${release_date}

### Changed

- Prepared ${version} release.

### Verification

- \`tool/prepare_release.sh ${version}\`

EOF
)

tmp_changelog=$(mktemp)
export CHANGELOG_SECTION="$changelog_section"
perl -0pe 'BEGIN { $section = $ENV{CHANGELOG_SECTION} . "\n\n" } if (!s/(\n## v[0-9]+\.[0-9]+\.[0-9]+[^\n]*\n)/\n$section$1/s) { $_ .= "\n$section" }' CHANGELOG.md > "$tmp_changelog"
mv "$tmp_changelog" CHANGELOG.md

export RELEASE_VERSION="$version"
export RELEASE_NUMBER="${version#v}"
perl -0pi -e 's/tag: "v[0-9]+\.[0-9]+\.[0-9]+"/tag: "$ENV{RELEASE_VERSION}"/; s/version "[0-9]+\.[0-9]+\.[0-9]+"/version "$ENV{RELEASE_NUMBER}"/' packaging/homebrew/rail.rb

GITHUB_REF_NAME="$version" ./tool/verify_release_formula.sh
git diff --check

git add CHANGELOG.md packaging/homebrew/rail.rb
git commit -m "chore: prepare ${version} release"

if $local_only; then
  cat <<EOF
release preparation commit created on $branch

To open the release PR:
  git push -u origin $branch
  gh pr create --base main --head $branch --title "chore: prepare ${version} release"

After the PR merges, publish with:
  HOMEBREW_TAP_GITHUB_TOKEN=... tool/prepare_release.sh ${version} --push
EOF
  exit 0
fi

git push -u origin "$branch"

pr_body=$(mktemp)
cat > "$pr_body" <<EOF
Prepares ${version} for release.

After this PR merges, publish with:

\`\`\`bash
HOMEBREW_TAP_GITHUB_TOKEN=... tool/prepare_release.sh ${version} --push
\`\`\`
EOF

gh pr create \
  --base main \
  --head "$branch" \
  --title "chore: prepare ${version} release" \
  --body-file "$pr_body"
