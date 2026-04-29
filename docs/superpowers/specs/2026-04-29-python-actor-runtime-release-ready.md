# Python Actor Runtime Release-Ready Product Spec

**Date:** 2026-04-29

## Goal

Make the Python-first Rail Harness Runtime release-ready as an experimental
product: skill-first, SDK-powered, policy-governed, artifact-resumable, and
safe enough to run against real target repositories without relying on the old
runtime.

This spec supersedes older release-ready documents that describe the previous
runtime and packaging direction. Those older documents remain useful as
historical context, but this file is the canonical product target after the
Python Actor Runtime redesign.

## Release-Ready Definition

Rail is release-ready when a user can describe a software task through the Rail
skill, receive an artifact handle, let Rail supervise SDK-backed actors, inspect
the result, and resume or debug the artifact without understanding internal
request files, task IDs, runtime flags, or SDK traces.

The release candidate must satisfy all of these conditions:

- [x] The default Actor Runtime can execute through the OpenAI Agents SDK with a
      real runner when credentials are configured.
- [x] Missing or invalid credentials fail before actor work starts and produce a
      secret-safe readiness report.
- [x] Artifact handles are persisted, reloadable, and bound to artifact id,
      request digest, effective policy digest, and canonical project root.
- [x] Existing artifact operations can resume from a persisted handle without
      composing a new request or allocating a new artifact.
- [x] Validation evidence comes from Rail-owned request or policy commands, not
      synthetic success or actor-invented commands.
- [x] Terminal pass requires matching request, policy, actor invocation, patch,
      target tree, validation, and evaluator digests.
- [x] Result and terminal summaries clearly distinguish pass, reject, blocked
      runtime, blocked validation, blocked policy, and blocked environment
      outcomes.
- [ ] Test-only fake actor paths cannot be used accidentally by the public
      supervise path.
- [ ] The Rail skill and bundled skill document the handle-based API flow and
      do not instruct users to manage request YAML, task IDs, or wrapper flags.
- [x] Active docs and release checklist point to this Python release-ready
      boundary.
- [ ] The release gate runs the full Python test suite, lint, typing, docs
      guard, no-legacy guard, deterministic SDK-adapter smoke, and an optional
      live SDK smoke when credentials are present.

## Product Boundary

Rail owns:

- request draft normalization
- artifact identity and resume policy
- effective policy loading and narrowing
- supervisor graph and routing
- patch bundle validation and apply
- validation evidence
- evaluator gate
- terminal summary and result projection
- skill-first user workflow

The Actor Runtime owns:

- OpenAI Agents SDK agent construction
- model invocation
- SDK trace and event normalization
- structured actor output validation
- sandbox-local exploration and patch production

The Actor Runtime is not the source of truth for terminal outcome. Rail remains
the authority for mutation, validation, evaluator pass, and result projection.

## Required User Flows

### Fresh Work

1. User asks the Rail skill to perform a task in a target repository.
2. Rail normalizes the draft and allocates a fresh artifact handle.
3. Rail verifies Actor Runtime readiness.
4. Rail supervises planner, context builder, critic, generator, executor, and
   evaluator.
5. Rail applies only validated patch bundles to the target.
6. Rail records validation and evaluator evidence.
7. Rail returns a result projection and terminal summary.

### Existing Artifact

1. User supplies or references an artifact handle.
2. Rail loads and validates the handle.
3. Rail does not compose a new request.
4. Rail does not allocate a new artifact.
5. Rail performs the requested status, result, supervise, retry, or debug
   operation against that artifact.

### Readiness Failure

1. Rail checks credentials and runtime readiness before actor execution.
2. If readiness fails, Rail writes a blocked run status with a secret-safe
   reason.
3. Rail result tells the user how to fix the environment without exposing
   secret values or machine-specific credential paths.

## Release Gate

The mandatory release gate is:

```bash
uv run --python 3.12 pytest -q
uv run --python 3.12 ruff check src tests
uv run --python 3.12 mypy src/rail
```

The docs and legacy-surface guards must be part of the test suite:

- `tests/docs/test_no_home_paths.py`
- `tests/docs/test_removed_runtime_surfaces.py`
- `tests/test_no_legacy_runtime_calls.py`

The optional live smoke is gated by operator credentials. It must be skipped
when credentials are absent and must never run in default CI without an explicit
operator signal.

## Non-Goals

- Do not restore the old runtime.
- Do not require users to choose task IDs.
- Do not make a command-line interface the product authority.
- Do not accept target-local credentials as trusted input.
- Do not treat SDK traces as terminal decision authority.
- Do not let actors directly mutate target repositories.
- Do not claim release readiness from deterministic fake actor tests alone.

## Current Status

The Python redesign baseline is complete. Release-readiness hardening remains
open and is tracked in
`docs/superpowers/plans/2026-04-29-python-actor-runtime-release-ready.md`.
