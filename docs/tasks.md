# Active Release-Ready Tasks

This checklist tracks the remaining work to make the Python-first Rail Harness
Runtime release-ready.

Canonical release-ready product spec:

- `docs/SPEC.md`

Canonical execution plan:

- `docs/superpowers/plans/2026-04-29-python-actor-runtime-release-ready.md`

The dated specs under `docs/superpowers/specs/2026-04-29*.md` are the design
records used to derive `docs/SPEC.md`. The current product boundary is Python
API first, Rail skill first, and Actor Runtime based.

## Current Audit

Status: release-ready is not yet closed after the `docs/SPEC.md` audit.

The Python runtime criteria are largely implemented and covered by the local
release gate, but the distribution boundary is not yet enforced. The active
architecture says the experimental release artifact is a Python package with
the Rail Python API and bundled Rail skill assets. A built wheel currently does
not prove that the bundled skill assets, default harness assets, and resource
loading behavior survive installation outside the source checkout.

Current execution plan:

- `docs/superpowers/plans/2026-04-29-release-ready-audit-closure.md`

## Must

- [x] Close the Python package distribution gap
  Done when the wheel and sdist include the bundled Rail skill assets and
  Rail-owned default harness assets needed by the installed runtime.

- [x] Move installed runtime resource loading off source-checkout assumptions
  Done when runtime defaults are loaded from package resources or an explicitly
  configured target/project surface, not from a parent directory that only
  exists in the repository checkout.

- [x] Canonicalize packaging checks in the release gate
  Done when `scripts/python_release_gate.sh` builds the Python distribution and
  fails if required package assets or installed-resource smoke checks are
  missing.

- [x] Run a main-worktree critical review against `docs/SPEC.md`
  Done when an independent review checks that each release-ready criterion is
  enforced by code, tests, release gate, or documented operator-only evidence,
  and any findings are either fixed or recorded here as explicit follow-up.

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
  Done when the default Actor Runtime can use a configured OpenAI Agents SDK
  runner, missing credentials block before actor work, and readiness reports are
  secret-safe.

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
  docs guard, no-legacy guard, and deterministic SDK-adapter smoke.

## Completed Later

- [x] Define optional live SDK smoke criteria
  Done when operators can opt into a real SDK smoke with configured credentials
  and the smoke is skipped by default when credentials are absent.

- [x] Decide external distribution packaging
  Done when the release owner chooses whether this experimental product ships as
  a Python package, API service, app connector, or another wrapper over the same
  Python API.
