# Changelog

All notable Rail release changes are summarized here by tag.

## v0.2.2 - 2026-04-21

### Added

- Added the mandatory `critic` stage to the default actor graph so task families now traverse `planner -> context_builder -> critic -> generator -> executor -> evaluator`.
- Added repository-owned actor profile policy through `.harness/supervisor/actor_profiles.yaml`, including checked-in defaults for planner, context builder, critic, generator, evaluator, and integrator.
- Added `ActorWatchdog` progress monitoring for actor command runs. Actor processes are cancelled with `actor_watchdog_expired` when command output stops making observable progress for the quiet window.
- Added runtime evidence for `actor_profiles_used`, critic reporting, and critic-to-evaluator deltas in execution artifacts.

### Changed

- Generalized actor command execution to `runCommand` and made model/reasoning selection profile-only instead of environment-driven.
- Replaced actor-level timeout language with progress-watchdog behavior in active docs and skill material.
- Updated the Rail skill and bundled skill assets to use request-draft terminology and keep user-facing request composition skill-first.
- Replaced the live real-mode helper script with deterministic smoke gates plus runtime coverage for profile-selected actor command wiring.
- Added `CHANGELOG.md` as the release record for tagged changes.

### Fixed

- Made `v1_release_gate.sh` remove the target-repository smoke artifact before rerun, matching the `v2` gate behavior.
- Sanitized lingering user-home path examples in active docs.

### Verification

- `./tool/v1_release_gate.sh`
- `./tool/v2_release_gate.sh`
- `git diff --check`
- Active docs search for user-home paths and deprecated terminology

## v0.2.1 - 2026-04-20

### Changed

- Completed migration cleanup follow-ups after the Go runtime transition.
- Aligned skill-first operator documentation with the installed-product workflow.
- Clarified that normal users work through the Rail skill rather than hand-authoring harness YAML.

### Added

- Added the real actor execution path for actual target-repository work.
- Added Go-first release-gate wiring and package-oriented runtime guidance.

## v0.2.0 - 2026-04-14

### Added

- Added the Go CLI product skeleton with embedded harness defaults and file-level override resolution.
- Added `rail init`, project discovery, request composition, request validation, artifact bootstrap, route evaluation, run, and execute orchestration in the Go runtime.
- Added bundled Rail skill assets for packaged installs.
- Added explicit post-pass integrator support and `integration_result` evidence.
- Added review-driven learning workflows, approved-memory operations, and the `v2` release gate.

### Changed

- Moved the product from the Dart runtime path to the Go runtime path.
- Rewrote active README and architecture documentation around installed-product usage.
- Made the Go runtime the primary release path and retired Dart from the released product flow.

### Fixed

- Hardened asset resolution, init scaffolding, compose-request normalization, route evaluation, terminal artifact recovery, and smoke gate behavior.
- Hardened packaged skill installation layout and trimpath repository-root lookup.

## v0.1.0 - 2026-04-13

### Added

- Released the bounded `v1` core supervisor gate.
- Added smoke and standard validation profiles, narrow validation scopes, supervisor action loops, explicit environment failure routing, supervisor decision traces, and terminal execution artifacts.
- Added conservative evaluator gate behavior, guardrail telemetry, and review-only quality learning foundations.
- Added release-gate automation through local scripts and CI workflows.

### Changed

- Split the supervisor CLI from runtime internals and documented the `v1` release boundary.
- Clarified execution outcomes, routing taxonomy, launch terminal outcomes, and release documentation.
