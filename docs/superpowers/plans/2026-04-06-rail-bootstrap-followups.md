# Rail Bootstrap Follow-ups

## Implemented

- standalone control-plane repository created at `/absolute/path/to/rail`
- `.harness/` bundle migrated into the new repository
- runtime copied and adapted to `bin/rail.dart`
- target repo split introduced through `--project-root`
- repo-local `skills/rail/SKILL.md` added
- skill installation script added
- root README and harness README rewritten for the new model
- downstream smoke request fixture added at `.harness/requests/rail-downstream-actors-smoke.yaml`
- actor runtime now selects a rail-repo working directory for non-target actors and for generator runs whose plan stays entirely inside the rail repo
- request-level `validation_profile` now supports `smoke`, allowing executor planning to avoid full target-repo lint/test for harness smoke tasks
- smoke validation now uses a deterministic fast-path for planner/context_builder/generator/executor/evaluator, removing nested actor latency from harness smoke runs
- standard validation can now take request-level `validation_roots` and `validation_targets` so executor planning can stay narrower than workspace-wide fallback
- supervisor orchestration is moving from evaluator-only generator retry toward explicit action-driven loops with bounded budgets
- global `~/.codex/skills/rail/SKILL.md` now points at the repo-owned skill via symlink

## Next work

1. Exercise a real `standard` request through the new supervisor action loop and inspect whether evaluator `next_action` values route to the intended next stage.
2. Decide whether planner/context_builder also need a standard fast-path or cache to reduce nested actor latency.
3. Run `dart pub get` and `dart analyze` in the new repo.
4. Add project profiles beyond the default Flutter + Riverpod profile.
