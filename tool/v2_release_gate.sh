#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/tool/release_gate_common.sh"

SMOKE_TASK_ID="$(
  rail_validate_smoke_task_id \
    "${RAIL_RELEASE_SMOKE_TASK_ID:-v2-integrator-smoke-ci}"
)"
ARTIFACT_PATH=".harness/artifacts/${SMOKE_TASK_ID}"
MISSING_GO_COMMANDS=(
  "integrate"
  "validate-artifact"
  "verify-learning-state"
)

cd "$REPO_ROOT"

go test ./...

mkdir -p build
go build -o build/rail ./cmd/rail

rm -rf "$ARTIFACT_PATH"
./build/rail run \
  --request test/fixtures/valid_request.yaml \
  --project-root "$REPO_ROOT" \
  --task-id "$SMOKE_TASK_ID"
./build/rail execute --artifact "$ARTIFACT_PATH"

printf '%s\n' \
  "Go CLI parity is incomplete for v2 release gate." \
  "Missing commands: ${MISSING_GO_COMMANDS[*]}" \
  "Blocked after Go-first verification and smoke execution." >&2
exit 1
