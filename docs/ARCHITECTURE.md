# Rail Architecture

Rail is a Python-first harness control plane. It operates against a separate target repository and keeps workflow decisions explicit, bounded, and reviewable.

## Product Model

The normal user contract is skill-first:

1. the user describes the task in natural language
2. the Rail skill creates a request draft
3. `rail.specify(draft)` produces a schema-valid request object
4. `rail.start_task(draft)` allocates an artifact handle
5. `rail.supervise(handle)` runs the supervisor and Actor Runtime path
6. `rail.result(handle)` projects the terminal outcome from artifacts only

The command surface is optional wrapper UX. The Python API is the governing boundary.

## Core Components

### Request Layer

The request layer accepts skill-produced drafts, rejects unknown fields, applies defaults, and normalizes validation profile aliases. It does not require users to write request YAML.

### Artifact Store

The artifact store allocates opaque artifact ids and writes request snapshots, workflow state, run status, run evidence, and terminal summaries. Runtime evidence is attempt-scoped under `runs/attempt-NNNN/` so retries on the same artifact do not overwrite or project stale actor output. Artifact handles are canonical and digest-bound.

### Task Identity

Fresh goals allocate fresh artifacts. Existing artifact operations require a known handle. Request files are not run identity.

### Policy V2

Policy is loaded from Rail/operator defaults first. Target policy may only narrow the effective policy. Unknown keys are rejected. Direct target mutation is rejected.

### Actor Runtime

The Actor Runtime is the provider-selected execution boundary. The default local
provider is `codex_vault`; `openai_agents_sdk` is optional for
operator/API-key environments. Each actor receives:

- deterministic prompt source
- Codex-compatible strict output schema derived from the Rail actor contract
- narrowed policy
- secret-safe environment
- evidence writer

The `codex_vault` provider audits actor execution in three layers:

- bootstrap profile audit recognizes passive Codex-owned actor-local material
- provenance audit blocks parent/user/target-local or behavior-affecting
  unknown capability sources
- capability-use audit blocks Rail-policy-forbidden tool, skill, plugin, MCP,
  hook, rule, config, target mutation, or validation behavior

Actors return structured output and attempt-scoped evidence references. The supervisor owns routing and terminal decisions.

### Supervisor

The deterministic core graph is:

```text
planner -> context_builder -> critic -> generator -> executor -> evaluator
```

Evaluator `pass` can become terminal success only after the evaluator gate accepts current validation evidence.

### Workspace Isolation

Actor edits occur in an external sandbox. Target mutation happens only when Rail applies a validated patch bundle. Sandbox pre-scan and copy use the same ignored top-level paths so `.git` and `.harness` artifacts are not treated as actor-readable target input. Tree digests prevent stale or out-of-band target changes.

### Validation And Evaluator Gate

Validation evidence records command source, exit code, logs, credential mode, network mode, sandbox reference, patch digest, tree digest, and mutation status.

Evaluator pass is blocked when evidence is missing, failed, stale, actor-invented, mutation-unsafe, or digest-inconsistent.

### Projection

Status and result projection read artifacts only. They do not call the SDK or infer success from process output.

## Release Gate

`scripts/release_gate.sh` is the local release gate for the Rail control plane. It builds the Python package, inspects required package assets, runs an installed-wheel smoke, and runs the Python test suite, docs guards, removed-surface guards, lint, and typing checks. It does not prove that an arbitrary downstream target repository task succeeded.

Optional live smokes are skipped by default and are not part of normal CI.
`RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1` enables the optional
`openai_agents_sdk` smoke when operator SDK credentials are present.
`RAIL_CODEX_VAULT_LIVE_SMOKE=1` enables the optional `codex_vault` smoke when
Rail-owned Codex auth is configured. Normal task execution remains skill-first
and Python API first; setup diagnostics live under `rail auth`.

Live smoke repair is a developer diagnostic, not a release publisher or
downstream task runner. `rail smoke repair ... --live` consumes live smoke
reports, proposes safe Rail-owned repair candidates, and applies them only when
`--apply` is explicit. It does not commit, tag, publish, change release version
metadata, mutate downstream target repositories, or edit Codex auth material.

## Release Publishing

Release publication is tag-driven. A `v*` tag push runs `.github/workflows/publish.yml`, which checks:

- tag version matches `pyproject.toml`
- top `CHANGELOG.md` entry version matches the tag
- `scripts/release_gate.sh` passes
- `uv build` succeeds
- PyPI upload succeeds through Trusted Publishing

The workflow uses the same top `CHANGELOG.md` section as the release note source.

## Distribution Boundary

The release artifact is the `rail-sdk` Python package with the Rail Python API, bundled Rail skill assets, and setup-only console entrypoints. `rail migrate` installs or refreshes the Codex skill, and `rail doctor` checks local setup readiness. There is still no command-line product contract for task execution. Task execution remains governed by the Python API and Rail skill contract; any wrapper must remain a thin caller of the Python API.

## `.harness`

`.harness` remains a first-class product surface:

- `actors/` stores actor prompts
- `templates/` stores output contracts
- `supervisor/` stores policy and routing defaults
- `artifacts/` stores run evidence and status
- `requests/` stores request snapshots or optional inspected drafts

## Security Posture

Rail fails closed:

- no direct target mutation
- no target-local credentials
- no unknown policy keys
- no broadening target overlays
- no artifact identity from request paths
- no unredacted secret canaries in evidence
- no terminal pass without current validation evidence
