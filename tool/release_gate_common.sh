#!/usr/bin/env bash

rail_validate_smoke_task_id() {
  local task_id="${1:-}"

  if [[ -z "$task_id" ]]; then
    echo "invalid smoke task id: value must not be empty" >&2
    return 1
  fi

  if [[ ! "$task_id" =~ ^[A-Za-z0-9._-]+$ ]]; then
    echo "invalid smoke task id: $task_id" >&2
    return 1
  fi

  if [[ "$task_id" == .* || "$task_id" == *..* ]]; then
    echo "invalid smoke task id: $task_id" >&2
    return 1
  fi

  printf '%s\n' "$task_id"
}
