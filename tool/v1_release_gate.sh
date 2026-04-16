#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/tool/release_gate_common.sh"

SMOKE_TASK_ID="$(
  rail_validate_smoke_task_id \
    "${RAIL_RELEASE_SMOKE_TASK_ID:-v1-release-smoke-ci}"
)"

cd "$REPO_ROOT"

go test ./...

mkdir -p build
go build -o build/rail ./cmd/rail

rm -rf ".harness/artifacts/${SMOKE_TASK_ID}"
./build/rail run \
  --request test/fixtures/valid_request.yaml \
  --project-root "$REPO_ROOT" \
  --task-id "$SMOKE_TASK_ID"
./build/rail execute --artifact ".harness/artifacts/${SMOKE_TASK_ID}"
