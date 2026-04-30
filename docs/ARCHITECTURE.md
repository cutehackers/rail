# Rail Architecture

Rail is a Python-first harness control plane. It operates against a separate target repository and keeps workflow decisions explicit, bounded, and reviewable.

## Product Model

The normal user contract is skill-first:

1. the user describes the task in natural language
2. the Rail skill creates a request draft
3. `rail.normalize_request(draft)` produces a schema-valid request object
4. `rail.start_task(draft)` allocates an artifact handle
5. `rail.supervise(handle)` runs the supervisor and Actor Runtime path
6. `rail.result(handle)` projects the terminal outcome from artifacts only

The command surface is optional wrapper UX. The Python API is the governing boundary.

## Core Components

### Request Layer

The request layer accepts skill-produced drafts, rejects unknown fields, applies defaults, and normalizes validation profile aliases. It does not require users to write request YAML.

### Artifact Store

The artifact store allocates opaque artifact ids and writes request snapshots, workflow state, run status, run evidence, and terminal summaries. Artifact handles are canonical and digest-bound.

### Task Identity

Fresh goals allocate fresh artifacts. Existing artifact operations require a known handle. Request files are not run identity.

### Policy V2

Policy is loaded from Rail/operator defaults first. Target policy may only narrow the effective policy. Unknown keys are rejected. Direct target mutation is rejected.

### Actor Runtime

The Actor Runtime is the SDK-powered execution boundary. Each actor receives:

- deterministic prompt source
- Pydantic output schema
- narrowed policy
- secret-safe environment
- evidence writer

Actors return structured output and evidence references. The supervisor owns routing and terminal decisions.

### Supervisor

The deterministic core graph is:

```text
planner -> context_builder -> critic -> generator -> executor -> evaluator
```

Evaluator `pass` can become terminal success only after the evaluator gate accepts current validation evidence.

### Workspace Isolation

Actor edits occur in an external sandbox. Target mutation happens only when Rail applies a validated patch bundle. Tree digests prevent stale or out-of-band target changes.

### Validation And Evaluator Gate

Validation evidence records command source, exit code, logs, credential mode, network mode, sandbox reference, patch digest, tree digest, and mutation status.

Evaluator pass is blocked when evidence is missing, failed, stale, actor-invented, mutation-unsafe, or digest-inconsistent.

### Projection

Status and result projection read artifacts only. They do not call the SDK or infer success from process output.

## Release Gate

`scripts/release_gate.sh` is the local release gate for the Rail control plane. It builds the Python package, inspects required package assets, runs an installed-wheel smoke, and runs the Python test suite, docs guards, removed-surface guards, lint, and typing checks. It does not prove that an arbitrary downstream target repository task succeeded.

When `RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1` and an operator-controlled SDK credential is present, the gate also runs the optional live SDK smoke. That smoke is skipped by default and is not part of normal CI. For normal task execution, an operator-controlled `OPENAI_API_KEY` is enough to enable live Actor Runtime execution; users do not need to set runtime feature flags.

## Release Publishing

Release publication is tag-driven. A `v*` tag push runs `.github/workflows/publish.yml`, which checks:

- tag version matches `pyproject.toml`
- top `CHANGELOG.md` entry version matches the tag
- `scripts/release_gate.sh` passes
- `uv build` succeeds
- PyPI upload succeeds with `PYPI_API_TOKEN`

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
