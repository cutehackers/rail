# V1 Core Supervisor Gate Backlog

## Open Release Blockers

- add CI for the `v1` production release gate
- codify the release gate so analyze, test, compile, and smoke run together

## Verification Gaps

- no GitHub Actions release workflow yet
- no single automated release command or workflow runs compile plus fresh smoke together

## Done Criteria

This backlog is closed only when:

- `dart analyze` is clean
- `dart test` passes
- smoke and standard verification pass from the current checkout
- `dart compile exe bin/rail.dart -o build/rail` succeeds
- deferred `v2` flows are not required anywhere in the `v1` path
