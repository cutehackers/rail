# V1 Core Supervisor Gate Backlog

## Open Release Blockers

- none

The `v1` release gate is now codified in:

- `./tool/v1_release_gate.sh`
- `.github/workflows/v1-release-gate.yml`

## Verification Gaps

- none for the `v1` release contract

## Done Criteria

This backlog is closed only when:

- `dart analyze` is clean
- `dart test` passes
- smoke and standard verification pass from the current checkout
- `dart compile exe bin/rail.dart -o build/rail` succeeds
- deferred `v2` flows are not required anywhere in the `v1` path
