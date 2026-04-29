# Active Release-Ready Tasks

This checklist tracks the remaining work to make the Python-first Rail Harness
Runtime release-ready.

Canonical release-ready product spec:

- `docs/superpowers/specs/2026-04-29-python-actor-runtime-release-ready.md`

Canonical execution plan:

- `docs/superpowers/plans/2026-04-29-python-actor-runtime-release-ready.md`

The previous release-ready notes for earlier runtime boundaries are historical
context. The current product boundary is Python API first, Rail skill first, and
Actor Runtime based.

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
  Done when the canonical spec and implementation plan above define the next
  product boundary.

## Must

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

## Later

- [ ] Define optional live SDK smoke criteria
  Done when operators can opt into a real SDK smoke with configured credentials
  and the smoke is skipped by default when credentials are absent.

- [ ] Decide external distribution packaging
  Done when the release owner chooses whether this experimental product ships as
  a Python package, API service, app connector, or another wrapper over the same
  Python API.
