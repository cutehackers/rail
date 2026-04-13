#!/usr/bin/env bash

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SMOKE_TASK_ID="${RAIL_RELEASE_SMOKE_TASK_ID:-v1-release-smoke-ci}"

cd "$REPO_ROOT"

dart pub get
dart analyze
dart test

mkdir -p build
dart compile exe bin/rail.dart -o build/rail

rm -rf ".harness/artifacts/${SMOKE_TASK_ID}"
dart run bin/rail.dart run \
  --request test/fixtures/valid_request.yaml \
  --project-root "$REPO_ROOT" \
  --task-id "$SMOKE_TASK_ID"
dart run bin/rail.dart execute --artifact ".harness/artifacts/${SMOKE_TASK_ID}"
