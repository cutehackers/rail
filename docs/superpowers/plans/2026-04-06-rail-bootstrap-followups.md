# Rail Bootstrap Follow-ups

## Implemented

- standalone control-plane repository created at `/Users/junhyounglee/workspace/rail`
- `.harness/` bundle migrated into the new repository
- runtime copied and adapted to `bin/rail.dart`
- target repo split introduced through `--project-root`
- repo-local `skills/rail/SKILL.md` added
- skill installation script added
- root README and harness README rewritten for the new model

## Next work

1. Run `dart pub get` and `dart analyze` in the new repo.
2. Smoke-test:
   - `compose-request`
   - `validate-request`
   - `run --project-root /path/to/target-repo`
   - `execute --through planner`
3. Harden `generator`, `executor`, and `evaluator` behavior for non-trivial target repos.
4. Add project profiles beyond the default Flutter + Riverpod profile.
5. Decide whether the global `~/.codex/skills/rail` should become a thin wrapper or be replaced entirely by the repo-owned skill.

