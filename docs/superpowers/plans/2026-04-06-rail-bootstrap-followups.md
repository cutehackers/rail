# Rail Bootstrap Follow-ups

## Implemented

- standalone control-plane repository created at `/Users/junhyounglee/workspace/rail`
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
- global `~/.codex/skills/rail/SKILL.md` now points at the repo-owned skill via symlink

## Next work

1. Let the downstream smoke finish once and confirm schema-valid outputs for `implementation_result`, `execution_report`, and `evaluation_result`.
2. Decide whether executor validation for smoke should keep using full target-repo `melos` commands or switch to a narrower smoke-specific validation plan.
3. Run `dart pub get` and `dart analyze` in the new repo.
4. Add project profiles beyond the default Flutter + Riverpod profile.
