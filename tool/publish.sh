#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
export GH_PROMPT_DISABLED="${GH_PROMPT_DISABLED:-1}"
export GIT_TERMINAL_PROMPT="${GIT_TERMINAL_PROMPT:-0}"
export GIT_PAGER="${GIT_PAGER:-cat}"
export GIT_SSH_COMMAND="${GIT_SSH_COMMAND:-ssh -o BatchMode=yes}"
start_branch=""

log_step() {
  local step="$1"
  local message="$2"
  printf 'publish: step %s - %s\n' "$step" "$message" >&2
}

usage() {
  cat >&2 <<'EOF'
Usage: tool/publish.sh vX.Y.Z [--local-only] [--agent-changelog]

Prepares a release pull request by:
  - creating release/vX.Y.Z from main
  - adding a CHANGELOG.md section for vX.Y.Z
  - updating packaging/homebrew/rail.rb to vX.Y.Z
  - committing with a Conventional Commit message
  - pushing the branch and opening a PR to main

After the PR merges, publish the release with:
  HOMEBREW_TAP_GITHUB_TOKEN=... tool/prepare_release.sh vX.Y.Z --push

Options:
  --local-only       Create the local branch and commit, but do not push or open a PR.
  --agent-changelog  Ask Codex to summarize release changes for CHANGELOG.md.
EOF
}

version="${1:-}"
if [[ -z "$version" ]]; then
  usage
  exit 1
fi
shift

# 01. Validate input and execution mode.
local_only=false
agent_changelog=false
while (($#)); do
  case "$1" in
    --local-only)
      local_only=true
      ;;
    --agent-changelog)
      agent_changelog=true
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

restore_start_branch() {
  if [[ -z "$start_branch" ]]; then
    return
  fi

  if [[ "$(git branch --show-current 2>/dev/null || true)" == "$start_branch" ]]; then
    return
  fi

  if [[ -n "$(git status --porcelain 2>/dev/null || true)" ]]; then
    echo "working tree changed; staying on $(git branch --show-current) for inspection" >&2
    return
  fi

  git switch --quiet "$start_branch" >/dev/null 2>&1 || true
}

assert_main_synchronized() {
  local upstream counts behind ahead
  upstream=$(git rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null || true)
  if [[ -z "$upstream" ]]; then
    echo "main must track origin/main before publishing" >&2
    exit 1
  fi

  git fetch --quiet origin main
  counts=$(git rev-list --left-right --count "${upstream}...HEAD")
  behind="${counts%%[[:space:]]*}"
  ahead="${counts##*[[:space:]]}"
  if [[ "$behind" != "0" || "$ahead" != "0" ]]; then
    echo "main must be synchronized with origin/main before publishing; behind=$behind ahead=$ahead" >&2
    exit 1
  fi
}

assert_pr_create_permission() {
  if $local_only; then
    return
  fi

  local permission
  permission=$(gh repo view --json viewerPermission --jq .viewerPermission 2>/dev/null || true)
  case "$permission" in
    WRITE|MAINTAIN|ADMIN)
      ;;
    "")
      echo "gh cannot determine pull request permission for this repository" >&2
      exit 1
      ;;
    *)
      echo "gh cannot create pull requests for this repository; viewerPermission=$permission" >&2
      exit 1
      ;;
  esac
}

validate_changelog_section() {
  local section="$1"
  local first_line="${section%%$'\n'*}"

  if [[ ! "$first_line" =~ ^##[[:space:]]${version}[[:space:]]-[[:space:]][0-9]{4}-[0-9]{2}-[0-9]{2}$ ]]; then
    echo "generated changelog must start with '## ${version} - YYYY-MM-DD'" >&2
    exit 1
  fi

  if [[ "$section" != *"### Verification"* || "$section" != *"tool/prepare_release.sh ${version}"* ]]; then
    echo "generated changelog must include verification for tool/prepare_release.sh ${version}" >&2
    exit 1
  fi

  if [[ "$section" == *'```'* ]]; then
    echo "generated changelog must be plain markdown, not a fenced block" >&2
    exit 1
  fi

  if grep -Eq '(^|[[:space:]])(~|/Users/|/home/)' <<<"$section"; then
    echo "generated changelog must not include home-directory paths" >&2
    exit 1
  fi
}

template_changelog_section() {
  local release_date
  release_date=$(date +%F)
  cat <<EOF
## ${version} - ${release_date}

### Changed

- Prepared ${version} release.

### Verification

- \`tool/prepare_release.sh ${version}\`
EOF
}

agent_changelog_section() {
  if ! command -v codex >/dev/null 2>&1; then
    echo "codex is required when --agent-changelog is used" >&2
    exit 1
  fi

  local previous_tag range commit_log file_stat prompt_file output_file section
  git fetch --tags --quiet origin >/dev/null 2>&1 || true
  previous_tag=$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' --sort=-version:refname | head -n1 || true)
  if [[ -n "$previous_tag" ]]; then
    range="${previous_tag}..HEAD"
    file_stat=$(git diff --stat "$range" || true)
  else
    range="HEAD"
    file_stat=$(git log --first-parent --stat --oneline --max-count=50 HEAD || true)
  fi

  commit_log=$(git log --first-parent --pretty=format:'%h %s' "$range")
  prompt_file=$(mktemp)
  output_file=$(mktemp)

  cat > "$prompt_file" <<EOF
Write the CHANGELOG.md section for Rail release ${version}.

Output only the markdown section. Do not wrap it in a code fence.
The first line must be exactly:
## ${version} - $(date +%F)

Rules:
- Summarize only the concrete changes visible in the commit log and file stat below.
- Group entries under concise headings such as Added, Changed, Fixed, or Verification.
- Include a Verification section with exactly this command as a bullet:
  \`tool/prepare_release.sh ${version}\`
- Do not include home-directory paths such as /Users/<name>/... or ~/...
- Do not mention review_only.

Previous release tag: ${previous_tag:-none}

Commit log:
${commit_log:-No commit log available.}

File stat:
${file_stat:-No file stat available.}
EOF

  if ! codex exec \
    -C "$REPO_ROOT" \
    --sandbox read-only \
    --ephemeral \
    -c 'approval_policy="never"' \
    --color never \
    -o "$output_file" \
    - < "$prompt_file" >/dev/null; then
    echo "codex failed to generate changelog section" >&2
    exit 1
  fi

  section=$(cat "$output_file")
  rm -f "$prompt_file" "$output_file"
  validate_changelog_section "$section"
  printf '%s\n' "$section"
}

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "invalid release version: expected vX.Y.Z, got '$version'" >&2
  exit 1
fi
log_step "01" "validate inputs"

cd "$REPO_ROOT"

# 02. Validate repository state before creating release edits.
log_step "02" "check repository state"
if [[ "$(git rev-parse --is-inside-work-tree)" != "true" ]]; then
  echo "not inside a git work tree" >&2
  exit 1
fi

start_branch=$(git branch --show-current)
if [[ "$start_branch" != "main" ]]; then
  echo "publish must start from main" >&2
  exit 1
fi
trap restore_start_branch EXIT

if [[ -n "$(git status --porcelain)" ]]; then
  echo "working tree is dirty; commit or stash changes before publishing" >&2
  exit 1
fi

assert_main_synchronized

if ! command -v gh >/dev/null 2>&1; then
  echo "gh is required to open the release pull request" >&2
  exit 1
fi
assert_pr_create_permission

# 03. Ensure this version and release branch do not already exist.
log_step "03" "check release uniqueness"
if git rev-parse -q --verify "refs/tags/$version" >/dev/null; then
  echo "release tag already exists locally: $version" >&2
  exit 1
fi

if git ls-remote --exit-code --tags origin "refs/tags/$version" >/dev/null 2>&1; then
  echo "release tag already exists on origin: $version" >&2
  exit 1
fi

# 04. Create an isolated release branch from main.
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

log_step "04" "create release branch"
git checkout -b "$branch"

# 05. Generate and insert the release changelog section.
log_step "05" "generate changelog"
if $agent_changelog; then
  changelog_section=$(agent_changelog_section)
else
  changelog_section=$(template_changelog_section)
fi
validate_changelog_section "$changelog_section"

tmp_changelog=$(mktemp)
export CHANGELOG_SECTION="$changelog_section"
perl -0pe 'BEGIN { $section = $ENV{CHANGELOG_SECTION} . "\n\n" } if (!s/(\n## v[0-9]+\.[0-9]+\.[0-9]+[^\n]*\n)/\n$section$1/s) { $_ .= "\n$section" }' CHANGELOG.md > "$tmp_changelog"
mv "$tmp_changelog" CHANGELOG.md

# 06. Update the source Homebrew formula to the release tag.
log_step "06" "update homebrew formula"
export RELEASE_VERSION="$version"
export RELEASE_NUMBER="${version#v}"
perl -0pi -e 's/tag: "v[0-9]+\.[0-9]+\.[0-9]+"/tag: "$ENV{RELEASE_VERSION}"/; s/version "[0-9]+\.[0-9]+\.[0-9]+"/version "$ENV{RELEASE_NUMBER}"/' packaging/homebrew/rail.rb

# 07. Verify generated edits before committing.
log_step "07" "verify release edits"
GITHUB_REF_NAME="$version" ./tool/verify_release_formula.sh
git diff --check

# 08. Commit the release preparation changes.
log_step "08" "commit release prep"
git add CHANGELOG.md packaging/homebrew/rail.rb
git commit -m "chore: prepare ${version} release"

if $local_only; then
  log_step "09" "skip publish pull request (local only)"
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

# 09. Push the branch and open the release PR.
log_step "09" "publish pull request"
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
