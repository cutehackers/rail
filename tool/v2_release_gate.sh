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

FAKE_BIN="$(mktemp -d)"
trap 'rm -rf "$FAKE_BIN"' EXIT
cat > "$FAKE_BIN/codex" <<'PY'
#!/usr/bin/env python3
import json
import os
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

if not output_path:
    raise SystemExit("missing --output-last-message")

os.makedirs(os.path.dirname(output_path), exist_ok=True)
with open(output_path, "w", encoding="utf-8") as handle:
    json.dump({
        "summary": "Deterministic v2 smoke handoff: control-plane run, execute, and integrate paths completed against the checked-in smoke target.",
        "files_changed": [],
        "validation": [
            {
                "check_name": "smoke control-plane orchestration",
                "status": "pass",
                "evidence": "The v2 release gate produced a passing evaluator result before this deterministic integrator handoff.",
                "command": "./build/rail execute --artifact \"$ARTIFACT_PATH\"",
                "details": "Smoke-profile actor outputs are deterministic and schema-valid."
            },
            {
                "check_name": "post-pass integration handoff",
                "status": "pass",
                "evidence": "The integrator command produced schema-valid integration_result output for the smoke artifact.",
                "command": "./build/rail integrate --artifact \"$ARTIFACT_PATH\"",
                "details": "The gate uses this shim only to avoid live-agent dependency in CI."
            }
        ],
        "risks": [
            {
                "description": "Smoke gate proves control-plane orchestration, not broad downstream repair behavior.",
                "severity": "medium",
                "mitigation": "Use representative non-smoke artifacts and operator review before claiming broader release readiness."
            }
        ],
        "follow_up": [
            {
                "action": "Review non-smoke release evidence before claiming broad downstream repair coverage.",
                "owner": "release operator",
                "due": None,
                "notes": "This CI gate intentionally validates the deterministic smoke boundary."
            }
        ],
        "evidence_quality": "adequate",
        "release_readiness": "conditional",
        "blocking_issues": []
    }, handle)
PY
chmod +x "$FAKE_BIN/codex"

rm -rf "$ARTIFACT_PATH"
./build/rail run \
  --request "$TARGET_ROOT/.harness/requests/valid_request.yaml" \
  --project-root "$TARGET_ROOT" \
  --task-id "$SMOKE_TASK_ID"
./build/rail execute --artifact "$ARTIFACT_PATH"
PATH="$FAKE_BIN:$PATH" ./build/rail integrate --artifact "$ARTIFACT_PATH"
./build/rail validate-artifact \
  --file "$ARTIFACT_PATH/integration_result.yaml" \
  --schema integration_result
if grep -q '^release_readiness: blocked$' "$ARTIFACT_PATH/integration_result.yaml"; then
  printf '%s\n' \
    "v2 release gate blocked: integration_result.yaml reported release_readiness=blocked" \
    "Inspect $ARTIFACT_PATH/integration_result.yaml and resolve blocking issues before continuing." >&2
  exit 1
fi
(cd "$TARGET_ROOT" && "$REPO_ROOT/build/rail" verify-learning-state)
