# Active Release-Ready Tasks

This checklist tracks the remaining work to make the Python-first Rail Harness
Runtime release-ready.

Canonical release-ready product spec:

- `docs/SPEC.md`

Canonical baseline execution plan:

- `docs/superpowers/plans/2026-04-29-python-actor-runtime-release-ready.md`

The dated specs under `docs/superpowers/specs/2026-04-29*.md` are the design
records used to derive `docs/SPEC.md`. The current product boundary is Python
API first, Rail skill first, and Actor Runtime based.

## Current Audit

Status: release-ready criteria are closed for the mandatory local gate. The
distribution package name is `rail-sdk`, with setup entrypoints for
`rail migrate`, `rail doctor`, and `rail-sdk migrate`.

The post-audit blockers have been implemented and verified against
`docs/SPEC.md`: packaged assets survive wheel installation, Actor Runtime
readiness blocks missing, syntactically invalid, or provider-preflight-rejected
credentials before actor work,
blocked result projections distinguish runtime, validation, policy, and
environment categories, evaluator pass is bound to a supervisor-provided
evaluator input digest and validation evidence digest, repo `.harness` defaults
stay aligned with packaged defaults, and the canonical gate is
`scripts/release_gate.sh`. The final critical-review blockers are also
closed: the release gate directly runs the asset alignment checker, fake actor
outputs and inline integration flow helpers live only in tests, patch apply
rejects hardlink targets, policy load failures persist policy-blocked artifacts,
terminal pass rejects validation evidence whose network mode does not match the
effective policy, disabled-network validation ignores target-controlled
`sandbox-exec` shims and fails closed when the trusted OS sandbox is
unavailable, patch apply uses no-follow file descriptors and atomic replacement
to avoid symlink/hardlink escapes, and existing file permissions are preserved
when accepted patch bundles replace files. Artifact handle validation rejects
raw path-traversed `artifact_dir` and `project_root` values before
canonicalizing them for use.

Release publishing is now defined as a tag-driven pipeline:

- `.github/workflows/publish.yml` runs `scripts/release_gate.sh`, validates
  tag/version/changelog alignment, builds artifacts, uploads to PyPI through
  Trusted Publishing, and writes release notes from the same top `CHANGELOG.md`
  entry.

Local verification:

- `scripts/release_gate.sh` passed with optional live SDK smoke skipped
  by default.
- Optional live SDK smoke remains operator-gated by
  `RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1` plus operator SDK credentials.

Completed closure plans:

- `docs/superpowers/plans/2026-04-29-release-ready-audit-closure.md`
- `docs/superpowers/plans/2026-04-29-release-ready-gap-closure.md`

## Must

- [x] Close the Python package distribution gap
  Done when the wheel and sdist include the bundled Rail skill assets and
  Rail-owned default harness assets needed by the installed runtime.

- [x] Move installed runtime resource loading off source-checkout assumptions
  Done when runtime defaults are loaded from package resources or an explicitly
  configured target/project surface, not from a parent directory that only
  exists in the repository checkout.

- [x] Canonicalize packaging checks in the release gate
  Done when `scripts/release_gate.sh` builds the Python distribution and
  fails if required package assets or installed-resource smoke checks are
  missing.

- [x] Run a main-worktree critical review against `docs/SPEC.md`
  Done when an independent review checks that each release-ready criterion is
  enforced by code, tests, release gate, or documented operator-only evidence,
  and any findings are either fixed or recorded here as explicit follow-up.

- [x] Block invalid Actor Runtime credentials before actor work
  Done when missing, syntactically invalid, or provider-preflight-rejected
  operator credentials produce a secret-safe environment-blocked result without
  invoking actor work, and provider preflight uses the configured runtime
  timeout.

- [x] Distinguish blocked result categories in `rail.result(handle)`
  Done when result projection exposes blocked category, reason, and a
  category-specific outcome label from artifacts only.

- [x] Make environment-blocked outcomes reachable
  Done when runtime readiness failures are persisted and projected as
  environment-blocked terminal outcomes with the actual blocking actor.

- [x] Bind evaluator pass to supervisor-provided evaluator input digest
  Done when every evaluator decision requires the evaluator output to echo the
  supervisor-computed evaluator input digest instead of accepting a self-hash of
  evaluator output.

- [x] Bind terminal pass to validation evidence digest
  Done when terminal pass verifies the persisted validation evidence digest in
  addition to request, policy, actor, patch, tree, and evaluator input digests.

- [x] Enforce repo `.harness` defaults and packaged defaults stay aligned
  Done when the release gate fails if `.harness/actors`, `.harness/rules`,
  `.harness/rubrics`, `.harness/supervisor`, or `.harness/templates` drift from
  `assets/defaults`, and package assets still match `assets/defaults`.

- [x] Canonicalize `docs/SPEC.md` release gate text
  Done when `docs/SPEC.md` points to `scripts/release_gate.sh` and lists
  build, package asset, installed-wheel, repo `.harness` alignment, test, lint,
  typing, deterministic smoke, and optional live smoke behavior.

- [x] Add tag-driven publish pipeline
  Done when release publication is bound to a tag push through
  `.github/workflows/publish.yml`, with enforced `pyproject.toml` version,
  top `CHANGELOG.md` version, and gate-first publishing behavior.

## Done

- [x] Complete the Python Actor Runtime redesign baseline
  Done when `docs/superpowers/specs/2026-04-29-python-actor-runtime-rail-redesign.md`
  marks its validation gates complete and the matching implementation plan is
  checked off.

- [x] Remove active dependency on the old runtime path
  Done when active product code and docs no longer depend on the old runtime,
  legacy command subprocess behavior, trusted path handling, or compatibility
  flags.

- [x] Establish release-ready target documentation
  Done when `docs/SPEC.md` and the implementation plan above define the next
  product boundary.

## Completed Must

- [x] Add persisted artifact handle loading
  Done when `handle.yaml` is written, `rail.load_handle(path)` validates it, and
  existing artifact operations can resume without composing a new request.

- [x] Add live Actor Runtime readiness
  Done when the default `codex_vault` Actor Runtime checks Rail-owned Codex auth
  and command readiness before actor work, and readiness reports are secret-safe.

- [x] Replace synthetic validation with Rail-owned validation evidence
  Done when validation evidence is produced by request or policy commands and
  terminal pass cannot rely on synthetic success.

- [x] Add terminal summary projection
  Done when pass, reject, runtime-blocked, validation-blocked, policy-blocked,
  and environment-blocked outcomes are summarized for users without inspecting
  raw artifacts.

- [x] Isolate fake actor runtime to tests
  Done when production runtime modules no longer export fake actor execution and
  public supervise cannot accidentally use it.

## Should

- [x] Align Rail skill copies to the release-ready handle workflow
  Done when repo-owned and bundled skill files describe fresh task, existing
  artifact, readiness failure, supervision, and result reporting through the
  Python API handle flow.

- [x] Add a Python release gate script
  Done when a single local script runs the full Python test suite, lint, typing,
  docs guard, no-legacy guard, and deterministic Actor Runtime smoke.

## Completed Later

- [x] Define optional live SDK smoke criteria
  Done when operators can opt into a real SDK smoke with configured credentials
  and the smoke is skipped by default when credentials are absent.

- [x] Decide external distribution packaging
  Done when the release owner chooses whether this experimental product ships as
  a Python package, API service, app connector, or another wrapper over the same
  Python API.
