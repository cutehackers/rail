#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/tool/release_gate_common.sh"

SMOKE_TASK_ID="$(
  rail_validate_smoke_task_id \
    "${RAIL_RELEASE_SMOKE_TASK_ID:-v2-integrator-smoke-ci}"
)"
TARGET_ROOT="$REPO_ROOT/examples/smoke-target"
ARTIFACT_PATH="$TARGET_ROOT/.harness/artifacts/${SMOKE_TASK_ID}"

cd "$REPO_ROOT"

go test ./...

mkdir -p build
go build -o build/rail ./cmd/rail

rm -rf "$ARTIFACT_PATH"
./build/rail run \
  --request "$TARGET_ROOT/.harness/requests/valid_request.yaml" \
  --project-root "$TARGET_ROOT" \
  --task-id "$SMOKE_TASK_ID"
./build/rail execute --artifact "$ARTIFACT_PATH"
./build/rail integrate --artifact "$ARTIFACT_PATH"
./build/rail validate-artifact \
  --file "$ARTIFACT_PATH/integration_result.yaml" \
  --schema integration_result
if grep -q '^release_readiness: blocked$' "$ARTIFACT_PATH/integration_result.yaml"; then
  printf '%s\n' \
    "v2 release gate blocked: integration_result.yaml reported release_readiness=blocked" \
    "Inspect $ARTIFACT_PATH/integration_result.yaml and resolve blocking issues before continuing." >&2
  exit 1
fi
./build/rail verify-learning-state
