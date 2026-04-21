# Changelog

All notable Rail release changes are summarized here by tag.

## v0.2.6 - 2026-04-21

### Added
- Added Codex actor backend policy as a first-class policy/config surface via `internal/runtime/actor_backend.go`, `.harness/supervisor/actor_backend.yaml`, and `assets/defaults/supervisor/actor_backend.yaml`.
- Added runtime wiring to drive Codex actor command behavior from backend policy in the actor execution path (`internal/runtime/actor_runtime.go`, `internal/runtime/integration.go`, `internal/runtime/runner.go`).
- Added release tooling for publish/release prep with `tool/prepare_release.sh` and `tool/publish.sh`, plus workflow updates in `.github/workflows/release.yml`.
- Added implementation and policy docs for the new Codex boundary and actor backend design in `docs/ARCHITECTURE.md`, `docs/ARCHITECTURE-kr.md`, and new dated Codex boundary docs.
- Added test coverage for new release-gate and actor-backend runtime paths (`internal/releasegate/releasegate_test.go`, `internal/runtime/actor_backend_test.go`, `internal/runtime/actor_runtime_test.go`, `internal/runtime/runner_test.go`).

### Changed
- Changed release flow to be driven from merged `release/v*` branches.
- Changed release preflight and execution routing to support non-main branches while preserving main-branch safety constraints.
- Changed actor execution plumbing by removing the legacy run command bridge and routing actor commands through backend policy.
- Updated docs and runtime boundaries to clarify the Rail Codex runtime/actor contract.

### Fixed
- Fixed full-access policy handling by rejecting full access in all backend environments, aligning the full-access backend contract, and preventing self-authorized full-access backend behavior.
- Fixed actor backend enforcement to reject full-access actor backend policy.
- Fixed publish flow to fail before readonly pull request creation.
- Fixed publish behavior to keep release publishing tied to a synced main branch state.

### Verification
- `tool/prepare_release.sh v0.2.6`


## v0.2.5 - 2026-04-21

### Added
- Added Codex actor backend policy as a first-class routing signal in runtime and supervisor defaults.
  - New policy model in `internal/runtime/actor_backend.go`.
  - New policy files in `.harness/supervisor/actor_backend.yaml` and `assets/defaults/supervisor/actor_backend.yaml`.
- Added release automation support for agent and publish workflows.
  - Added `tool/prepare_release.sh`.
  - Added `tool/publish.sh`.
  - Added `.github/workflows/release.yml`.
- Added release design and architecture documentation for the Codex actor-backend boundary.
  - `docs/ARCHITECTURE.md`
  - `docs/ARCHITECTURE-kr.md`
  - `docs/2026-04-21-codex-boundary-actor-backend.md`
  - `docs/2026-04-21-codex-boundary-actor-backend-design.md`

### Changed
- Routed Codex actor command execution through backend policy (instead of the legacy path), including runtime integration updates in `internal/runtime/actor_runtime.go`, `internal/runtime/integration.go`, and `internal/runtime/runner.go`.
- Aligned runtime and policy contract around full-access backend handling.
- Updated operator-facing guidance to reflect the Rail/Codex runtime boundary.

### Fixed
- Removed legacy run command bridge.
- Prevented insecure full-access backend configurations by rejecting full access where disallowed and rejecting self-authorized full access backends.
- Fixed release publishing flow behavior:
  - allow release preflight on non-main branches
  - keep publish releases on synced main
  - trace publish release steps
  - fail publish before readonly PR creation

### Verification
- `tool/prepare_release.sh v0.2.5`


## v0.2.4 - 2026-04-21

### Fixed

- Fixed Homebrew release archives so packaged builds include both `SKILL.md` and bundled Rail examples.
- Fixed generated Homebrew formula installation so the Codex-facing skill copy is created from packaged `pkgshare` assets.
- Added a release formula version check so tag-triggered releases fail when `packaging/homebrew/rail.rb` points at a different tag or version.

### Verification

- `go test ./...`
- `go build -o build/rail ./cmd/rail`
- `go run github.com/goreleaser/goreleaser/v2@latest release --snapshot --clean --skip=publish`
- `brew install rail`
- `brew test rail`
- `rail init --project-root /absolute/path/to/test-target`

## v0.2.3 - 2026-04-21

### Added

- Added the initial GoReleaser-based GitHub Release workflow for tagged Rail releases.
- Added publishing to the `cutehackers/homebrew-rail` tap with packaged CLI archives, checksums, and provenance attestation.
- Rewrote Rail skill examples around `Use the Rail skill` prompts for `bug_fix`, `feature_addition`, `safe_refactor`, `test_repair`, and smoke-mode harness verification.

### Changed

- Updated active install guidance to use `brew install cutehackers/rail/rail`.
- Updated `scripts/install_skill.sh` to point users to packaged installs instead of checkout-coupled skill installation.

### Note

- This release was superseded by `v0.2.4` because the generated Homebrew formula referenced a moved skill asset path.

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
