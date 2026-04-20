#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/rail-real-mode-XXXXXX")"
ARTIFACT_PATH="$TARGET_ROOT/.harness/artifacts/real-mode-check"

cleanup() {
  if [[ "${RAIL_KEEP_REAL_CHECK:-0}" != "1" ]]; then
    rm -rf "$TARGET_ROOT"
  fi
}
trap cleanup EXIT

if ! command -v codex >/dev/null 2>&1; then
  printf '%s\n' "real mode check requires \`codex\` on PATH" >&2
  exit 1
fi

cd "$REPO_ROOT"

export RAIL_ACTOR_MODEL="${RAIL_ACTOR_MODEL:-gpt-5.4-mini}"
export RAIL_ACTOR_REASONING_EFFORT="${RAIL_ACTOR_REASONING_EFFORT:-low}"

mkdir -p build
go build -o build/rail ./cmd/rail

git init -q "$TARGET_ROOT"
./build/rail init --project-root "$TARGET_ROOT"
mkdir -p "$TARGET_ROOT/feature"

cat >"$TARGET_ROOT/go.mod" <<'EOF'
module realmodecheck

go 1.25.0
EOF

cat >"$TARGET_ROOT/feature/ready.go" <<'EOF'
package feature

func Ready() bool { return false }
EOF

cat >"$TARGET_ROOT/feature/ready_test.go" <<'EOF'
package feature

import "testing"

func TestReady(t *testing.T) {
	if !Ready() {
		t.Fatal("expected Ready to return true")
	}
}
EOF

cat >"$TARGET_ROOT/.harness/requests/real_mode_check.yaml" <<'EOF'
task_type: bug_fix
goal: restore the Ready helper so the focused test passes
context:
  feature: feature
  suspected_files:
    - feature/ready.go
  validation_roots:
    - feature
  validation_targets:
    - feature/ready_test.go
constraints:
  - keep the change limited to the focused helper and its focused test
  - do not add dependencies
definition_of_done:
  - Ready returns true again
  - the focused test passes
  - analyze remains green
priority: medium
risk_tolerance: low
validation_profile: standard
EOF

./build/rail run \
  --request "$TARGET_ROOT/.harness/requests/real_mode_check.yaml" \
  --project-root "$TARGET_ROOT" \
  --task-id "real-mode-check"

printf '%s\n' "running real actor path check"
printf 'target_root=%s\n' "$TARGET_ROOT"
printf 'artifact_path=%s\n' "$ARTIFACT_PATH"
printf 'actor_model=%s\n' "$RAIL_ACTOR_MODEL"
printf 'reasoning_effort=%s\n' "$RAIL_ACTOR_REASONING_EFFORT"
printf '%s\n' "this can take a few minutes because planner/context/generator/evaluator run through codex"

./build/rail execute --artifact "$ARTIFACT_PATH"
./build/rail validate-artifact \
  --file "$ARTIFACT_PATH/evaluation_result.yaml" \
  --schema evaluation_result

grep -q '"status": "passed"' "$ARTIFACT_PATH/state.json"
grep -q 'func Ready() bool { return true }' "$TARGET_ROOT/feature/ready.go"

printf '%s\n' "real mode check passed"
printf 'target_root=%s\n' "$TARGET_ROOT"
printf 'artifact_path=%s\n' "$ARTIFACT_PATH"
