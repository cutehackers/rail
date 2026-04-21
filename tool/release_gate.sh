#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat >&2 <<'EOF'
Usage: tool/release_gate.sh [all|v1|v2]

Run one or both release gate scripts.
  all (default): run v1 then v2
  v1: run ./tool/v1_release_gate.sh
  v2: run ./tool/v2_release_gate.sh
EOF
}

mode="${1:-all}"
shift || true
if [[ "$#" -gt 0 ]]; then
  echo "unexpected arguments: $*" >&2
  usage
  exit 1
fi

case "$mode" in
  all)
    modes=(v1 v2)
    ;;
  v1|v2)
    modes=("$mode")
    ;;
  -h|--help)
    usage
    exit 0
    ;;
  *)
    usage
    exit 1
    ;;
esac

run_v1() {
  "${REPO_ROOT}/tool/v1_release_gate.sh"
}

run_v2() {
  "${REPO_ROOT}/tool/v2_release_gate.sh"
}

for gate in "${modes[@]}"; do
  case "$gate" in
    v1) run_v1 ;;
    v2) run_v2 ;;
  esac
done

