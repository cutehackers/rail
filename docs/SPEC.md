# Rail Product Spec

## Status

This is the canonical product contract for the Python-first Rail Harness. It
consolidates the current product boundary from:

- `docs/superpowers/specs/2026-04-29-python-actor-runtime-rail-redesign.md`
- `docs/superpowers/specs/2026-04-29-python-actor-runtime-release-ready.md`

Those dated specs remain historical design records. This document is the stable
contract to read before making product, runtime, skill, or release-readiness
changes.

## Goal

Rail is a skill-first harness control plane for bounded agentic software work
against a separate target repository.

Rail is release-ready when a user can describe a software task through the Rail
skill, receive an artifact handle, let Rail supervise SDK-backed actors, inspect
the result, and resume or debug that artifact without understanding request
files, task IDs, runtime flags, or SDK traces.

## Product Contract

Rail must preserve this user contract:

1. The user describes work in natural language through the Rail skill.
2. The Rail skill creates a structured Python request draft.
3. `rail.normalize_request(draft)` produces a schema-valid request.
4. `rail.start_task(draft)` allocates a fresh artifact handle for fresh work.
5. `rail.supervise(handle)` runs a bounded supervisor workflow.
6. `rail.status(handle)` and `rail.result(handle)` project state from artifacts
   only.

The Python API is the governing boundary. Any wrapper or future UI must delegate
to the same API and must not become the product authority.

## Runtime Vocabulary

- **Rail Harness Runtime:** the Python runtime that owns request normalization,
  artifacts, policy, supervisor routing, validation, evaluator gates, and result
  projection.
- **Actor Runtime:** the OpenAI Agents SDK powered component that executes one
  Rail actor invocation and returns schema-valid output plus evidence.
- **Supervisor:** the deterministic Rail graph and routing logic.
- **Policy Gate:** the code path that narrows, blocks, or accepts capabilities,
  patch bundles, validation evidence, and terminal outcomes.
- **Artifact Store:** the durable workflow record under `.harness/artifacts/`.
- **Patch Bundle:** the Actor Runtime's proposed filesystem mutation package.
  Rail validates and applies it.

Avoid using "backend" for the Actor Runtime. The Actor Runtime executes actors;
Rail remains the governance harness.

## Product Boundaries

Rail owns:

- skill-first request draft normalization
- artifact identity and resume policy
- effective policy loading and narrowing
- supervisor graph and deterministic routing
- patch bundle validation and target apply
- Rail-owned validation evidence
- evaluator evidence-chain checks
- terminal summaries, status, and result projection
- reviewed learning state

The Actor Runtime owns:

- OpenAI Agents SDK agent construction
- model invocation
- SDK trace and event normalization
- structured actor output validation
- sandbox-local exploration and patch production

The Actor Runtime is not the source of truth for terminal outcome. SDK traces
are evidence, not decision authority.

## Required User Flows

### Fresh Work

1. User asks the Rail skill to perform work in a target repository.
2. Rail normalizes the draft and allocates a fresh artifact handle.
3. Rail verifies Actor Runtime readiness before actor work starts.
4. Rail supervises planner, context builder, critic, generator, executor, and
   evaluator.
5. Rail applies only validated patch bundles to the target.
6. Rail records validation and evaluator evidence.
7. Rail returns a terminal summary and result projection.

### Existing Artifact

1. User supplies or references an artifact handle.
2. Rail loads and validates the handle.
3. Rail does not compose a new request.
4. Rail does not allocate a new artifact.
5. Rail performs the requested status, result, supervise, retry, or debug
   operation against that artifact.

### Readiness Failure

1. Rail checks credentials and runtime readiness before actor execution.
2. If readiness fails, Rail writes blocked run status with a secret-safe reason.
3. Result projection tells the user how to fix the environment without exposing
   secret values or machine-specific credential paths.

## Task Identity And Resume

Rail does not make users choose task IDs. The durable identity of a run is the
artifact handle returned by `rail.start_task(draft)`.

An artifact handle includes:

- schema version
- opaque artifact ID
- canonical artifact directory
- canonical target project root
- request snapshot digest
- effective policy digest when available
- creation time

Rail must canonicalize every handle before use. It rejects handles when the
artifact directory or project root is symlinked, path-traversed, outside the
expected artifact store, mismatched with the current project, missing its
request snapshot, or inconsistent with recorded digests.

Fresh task flow:

1. Normalize the natural-language request into a request draft.
2. Materialize a request snapshot.
3. Allocate a new artifact atomically.
4. Return the artifact handle.
5. Supervise using that handle.

Existing artifact flow:

1. Resolve the supplied or remembered artifact handle.
2. Do not compose a new request.
3. Do not allocate a new artifact.
4. Run the requested operation against the existing artifact.

Blocked, rejected, or failed artifacts do not automatically capture future work.
If the user gives a new goal, Rail starts a fresh task even when a prior
artifact is blocked. Resume happens only when the user asks to continue prior
work or provides an artifact handle.

Request files under `.harness/requests/` are not durable run identity. The
artifact-local request snapshot and artifact handle are the source of truth.

## Policy

Rail loads Rail/operator defaults first. A target repository policy may only
narrow the effective policy; it cannot enable tools, credentials, providers,
approval behavior, mutation modes, resource ceilings, or network modes that
defaults did not authorize.

Policy composition must:

- reject unknown keys
- fail closed on missing values
- use explicit per-field narrowing
- cap timeouts and resource limits
- prevent direct target mutation
- write the canonical effective policy digest to runtime evidence

Target-local `.harness` policy is never a source of additional trust.

## Workspace And Mutation

Actor work happens in an external sandbox. The target repository is not mutated
until Rail validates and applies a patch bundle.

Required mutation safeguards:

- scrub actor and validation environments
- deny absolute paths that point at the target root
- deny path traversal
- deny symlink and hardlink escapes
- capture target tree digests before actor execution, before patch apply, after
  patch apply, before validation, and after validation
- fail when the target tree changes before Rail applies a patch
- report or block validation-side mutation unless policy allows it
- execute validation commands from request or policy, not actor invention
- apply accepted changes only through Rail-validated patch bundles

Patch bundle authority comes from effective policy, not actor-controlled fields.
Multi-file apply must either complete or leave the target unchanged.

## Evidence And Evaluator Gate

Each actor run writes stable evidence for result projection and human review:

- normalized event log
- SDK trace reference or exported trace
- runtime evidence
- structured actor output
- optional patch bundle
- validation evidence when applicable

Evidence must be hash-linked across:

- request snapshot
- effective policy
- actor invocation
- actor output
- patch bundle
- pre-apply target tree
- post-apply target tree
- validation evidence
- evaluator input

Evaluator `pass` can become terminal success only after the evaluator gate
accepts current validation evidence. A pass is blocked when evidence is missing,
failed, stale, actor-invented, mutation-unsafe, policy-inconsistent, or
digest-inconsistent.

## Auth And Secrets

SDK credentials come from approved operator-controlled sources only.
Target-local credential files and target-local environment requests are
rejected.
When an approved operator `OPENAI_API_KEY` is present, live Actor Runtime
execution is enabled without requiring normal users to set internal runtime
feature flags.

Rail must not persist secret values in:

- artifacts
- SDK event exports
- validation logs
- terminal summaries
- status projection
- result projection

Evidence may record credential source category, but not credential values or
machine-specific secret paths.

## `.harness` Surface

`.harness` remains a first-class product surface:

- `actors/` stores actor prompts
- `templates/` stores output contracts
- `supervisor/` stores policy and routing defaults
- `artifacts/` stores run evidence and status
- `requests/` stores request snapshots or optional inspected drafts
- `rules/` and `rubrics/` store guardrails and evaluation criteria
- `learning/` stores reviewed learning state

Normal users interact through the Rail skill and Python API. They should not
need to hand-write harness files.

The `rail-sdk` package may expose setup-only console entrypoints:

- `rail migrate` installs or refreshes the local Rail Codex skill.
- `rail doctor` checks package, skill, credential, and old Homebrew setup
  readiness.
- `rail-sdk migrate` is available when an older Homebrew `rail` binary still
  shadows the new console entrypoint.

These setup commands do not replace the Python API or Rail skill as the product
contract for task execution.

## Release-Ready Criteria

Rail is release-ready when all of the following are true:

- The default Actor Runtime can execute through the OpenAI Agents SDK with a
  real runner when credentials are configured.
- Missing or invalid credentials fail before actor work starts and produce a
  secret-safe readiness report.
- Artifact handles are persisted, reloadable, and bound to artifact ID, request
  digest, effective policy digest, and canonical project root.
- Existing artifact operations resume from a persisted handle without composing
  a new request or allocating a new artifact.
- Validation evidence comes from Rail-owned request or policy commands, not
  synthetic success or actor-invented commands.
- Terminal pass requires matching request, policy, actor invocation, patch,
  target tree, validation, and evaluator digests.
- Result and terminal summaries clearly distinguish pass, reject, runtime
  blocked, validation blocked, policy blocked, and environment blocked outcomes.
- Test-only actor runtime fixtures cannot be used accidentally by the public
  supervise path.
- The Rail skill and bundled skill document the handle-based API flow.
- Active docs and release checklist point to this product spec.
- The release gate builds the Python package, verifies packaged Rail assets,
  verifies repo `.harness` defaults stay aligned with packaged defaults, smokes
  the installed wheel, runs tests, lint, typing, docs guards, no-legacy guards,
  deterministic SDK-adapter smoke, and optional live SDK smoke when credentials
  are explicitly enabled.

## Release Gate

The mandatory local release gate is:

```bash
scripts/release_gate.sh
```

The gate removes stale build artifacts, verifies repo `.harness`,
`assets/defaults`, and packaged default/skill assets stay aligned with
`scripts/check_package_asset_alignment.py`, runs `uv build`, verifies required
wheel and sdist assets with `scripts/check_python_package_assets.py`, smokes the
installed wheel with `scripts/check_installed_wheel.py`, runs the Python test
suite excluding the optional live smoke, runs lint and typing, and preserves
docs guards, no-legacy guards, naming guards, repo `.harness` default alignment,
and deterministic SDK-adapter smoke through the test suite.

Optional live SDK smoke is skipped by default. When
`RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1` and operator SDK credentials are configured,
the gate enables live Actor Runtime execution and runs a narrow real-runner
planner smoke to prove SDK adapter readiness. It is not evidence that an
arbitrary downstream target repository task succeeded.

## Release Publishing (operator)

Public release is tag-driven. The operator performs this sequence:

1. Run `./publish.sh v${VERSION}` from the release HEAD intended for `main`.
2. If the top-of-file `CHANGELOG.md` entry named
   `## v${VERSION} - <YYYY-MM-DD>` is missing, the script must create it from
   changes since the previous release tag before changing package metadata.
3. If the changelog entry already exists, the script must preserve it and only
   validate its quality.
4. The script validates changelog quality, updates `pyproject.toml` version to
   match `${VERSION}`, updates `uv.lock`, and runs the local release gate:
   `scripts/release_gate.sh`.
5. The script commits release metadata changes when needed, pushes `main`, tags
   `v${VERSION}`, and pushes the tag.

`.github/workflows/publish.yml` is the canonical publishing pipeline.
It must fail if:

- `pyproject.toml` version and tag version differ.
- the top `CHANGELOG.md` entry version and tag version differ.
- the local release gate fails.
- package build or PyPI publish fails.

Use `CHANGELOG.md` as the only public release note/checkpoint source.
The same top section is used for operator-triggered release notes.

## Non-Goals

- Do not restore the old runtime.
- Do not preserve legacy command compatibility.
- Do not require users to choose task IDs.
- Do not make a wrapper interface the product authority.
- Do not accept target-local credentials as trusted input.
- Do not treat SDK traces as terminal decision authority.
- Do not let actors directly mutate target repositories.
- Do not claim release readiness from deterministic fixture tests alone.
