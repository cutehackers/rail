# Architecture Rules

- Preserve the current control-plane boundaries across `cmd/`, `internal/`, `.harness/`, `skills/`, and `docs/`.
- Keep request composition, runtime orchestration, routing, reporting, and learning-state operations explicit and reviewable.
- Do not move files across architecture boundaries without explicit permission.
- Keep dependency direction simple: CLI entrypoint -> internal runtime/services -> contracts/assets.
- Prefer deterministic data flow and traceable artifact outputs over hidden automation.
- Reuse adjacent test patterns and keep verification close to the behavior under change.
- Do not introduce generated code edits unless the task explicitly requires regeneration.
- Prefer minimal diffs for low-risk tasks.
