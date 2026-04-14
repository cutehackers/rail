#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source "$REPO_ROOT/tool/release_gate_common.sh"

SMOKE_TASK_ID="$(
  rail_validate_smoke_task_id \
    "${RAIL_RELEASE_SMOKE_TASK_ID:-v2-integrator-smoke-ci}"
)"
ARTIFACT_PATH=".harness/artifacts/${SMOKE_TASK_ID}"

cd "$REPO_ROOT"

dart pub get
dart analyze
dart test

mkdir -p build
dart compile exe bin/rail.dart -o build/rail

rm -rf "$ARTIFACT_PATH"
dart run bin/rail.dart run \
  --request test/fixtures/valid_request.yaml \
  --project-root "$REPO_ROOT" \
  --task-id "$SMOKE_TASK_ID"
dart run bin/rail.dart execute --artifact "$ARTIFACT_PATH"
dart run bin/rail.dart integrate --artifact "$ARTIFACT_PATH"
dart run bin/rail.dart validate-artifact \
  --file "${ARTIFACT_PATH}/integration_result.yaml" \
  --schema integration_result
dart run bin/rail.dart verify-learning-state

if rg -q "^release_readiness:[[:space:]]*['\"]?blocked['\"]?[[:space:]]*$" "${ARTIFACT_PATH}/integration_result.yaml"; then
  echo "V2 gate failed: integration_result reports release_readiness=blocked."
  exit 1
fi

echo "V2 gate completed. Review ${ARTIFACT_PATH}/integration_result.yaml before claiming release."
