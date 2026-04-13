# V1 Core Supervisor Gate Backlog

## Open Release Blockers

- fix schema drift between the live `v1` execution path and checked contracts
- add automated tests for request validation, routing, terminal reporting, and smoke execution
- eliminate analyzer warnings and dead code from the release path
- refactor `bin/rail.dart` into focused modules under `lib/src/`
- refresh stale checked-in smoke and standard evidence
- add CI for the `v1` production release gate
- remove repository hygiene blockers such as `bin/rail.dart.rej`

## Verification Gaps

- no `package:test` infrastructure yet
- no `analysis_options.yaml` yet
- no GitHub Actions release workflow yet
- current smoke execution is not trustworthy until fresh `run -> execute` passes under the `v1` contract

## Done Criteria

This backlog is closed only when:

- `dart analyze` is clean
- `dart test` passes
- smoke and standard verification pass from the current checkout
- `dart compile exe bin/rail.dart -o build/rail` succeeds
- deferred `v2` flows are not required anywhere in the `v1` path
