#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/tool/release_gate_common.sh"

SMOKE_TASK_ID="$(
  rail_validate_smoke_task_id \
    "${RAIL_RELEASE_SMOKE_TASK_ID:-v1-release-smoke-ci}"
)"
TARGET_ROOT="$REPO_ROOT/examples/smoke-target"

cd "$REPO_ROOT"

go test ./...

mkdir -p build
go build -o build/rail ./cmd/rail

rm -rf ".harness/artifacts/${SMOKE_TASK_ID}"
./build/rail run \
  --request "$TARGET_ROOT/.harness/requests/valid_request.yaml" \
  --project-root "$TARGET_ROOT" \
  --task-id "$SMOKE_TASK_ID"
./build/rail execute --artifact "$TARGET_ROOT/.harness/artifacts/${SMOKE_TASK_ID}"
