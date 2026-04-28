# V1 Core Supervisor Gate Backlog

## Open Release Blockers

- none

The `v1` release gate is codified in:

- `./tool/v1_release_gate.sh`
- `.github/workflows/release-gate.yml` for pull requests targeting `main`
- `.github/workflows/go-release-gate.yml`

## Verification Gaps

- none for the `v1` release contract

## Done Criteria

This backlog is closed only when:

- the installed-product command surface for `v1` is stable
- packaged releases include the bundled Rail skill
- embedded defaults resolve correctly when a project has no local override
- project-local `.harness` state stays local and reviewable
- smoke and representative standard-route verification pass through the release gate
- deferred `v2` flows are not required anywhere in the `v1` path
