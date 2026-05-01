# rail

`rail` is a skill-first harness control plane for bounded agentic software work.

The canonical product contract is `docs/SPEC.md`.

The product contract is:

- users describe work in natural language through the Rail skill
- the Rail skill creates a structured request draft
- the Python Rail Harness Runtime normalizes that draft, allocates an artifact handle, supervises bounded actors, validates evidence, and projects the result

Users should not hand-write harness YAML or manage task ids.

## Runtime Boundary

Rail owns the governance layer:

- request normalization
- artifact identity and resume policy
- Actor Runtime policy
- supervisor routing
- sandbox and patch bundle mutation boundaries
- validation evidence
- evaluator gate decisions
- result and status projection

The Actor Runtime is the provider-selected execution boundary for bounded actor work. The default local provider is `codex_vault`; `openai_agents_sdk` remains optional for operator/API-key environments. Each actor receives a narrowed policy and schema-bound prompt, then returns structured actor output and evidence references. Target repository mutation happens only through Rail-validated patch bundles.

## Python API

The primary product boundary is the Python API:

```python
import rail

draft = {
    "project_root": "/absolute/path/to/target-repo",
    "task_type": "bug_fix",
    "goal": "Fix the intermittent profile refresh loading issue.",
    "constraints": ["Do not change the API contract."],
    "definition_of_done": ["Refresh completes reliably.", "Focused validation passes."],
}

request = rail.specify(draft)
handle = rail.start_task(draft)
rail.supervise(handle)
result = rail.result(handle)
```

The optional command surface, when present, is only a thin wrapper over this API.

## Task Identity And Resume

`start_task(draft)` always allocates a fresh artifact for a fresh goal. This remains true even when an older artifact is blocked, rejected, or failed.

Existing artifact operations use the artifact handle returned by `start_task`. A request file path is not run identity, and users do not choose task ids.

Resume-like intents such as continue, retry, status, result, debug, or integrate require a known artifact handle. Without one, Rail asks for clarification instead of guessing.

An artifact handle includes:

- schema version
- opaque artifact id
- canonical artifact directory
- canonical project root
- request snapshot digest
- effective policy digest when available
- creation time

Rail rejects forged or unsafe handles, including symlinked artifact directories, traversal, mismatched project roots, missing request snapshots, and digest mismatches.

## Actor Runtime Policy

Rail loads operator defaults first. A target repository policy may only narrow the effective policy; it cannot enable tools, credentials, providers, approval behavior, mutation modes, resource ceilings, or network modes that defaults did not authorize.

The default policy disables host shell, filesystem, network, and MCP tools for actors. Direct target mutation is not allowed.

## Sandbox And Patch Boundary

Actor work happens in an external sandbox. The target repository is not mutated until Rail validates and applies a patch bundle.

Patch bundles reject:

- absolute paths
- path traversal
- writes into `.harness/artifacts` unless explicitly allowed for evidence
- symlink and hardlink escapes
- binary writes unless policy allows them
- executable bit changes unless policy allows them
- stale target tree digests

Validation evidence is tied to the applied patch and target tree digest. Evaluator pass is blocked when required validation is missing, failed, stale, actor-invented, or mutation-unsafe.

## Credentials

The default `codex_vault` path uses Rail-owned Codex auth configured through `rail auth login` and checked with `rail auth doctor`. `openai_agents_sdk` credentials are optional operator-controlled inputs for explicit API-key environments. Target-local credential files and target-local environment requests are rejected.

Actor environments receive the minimum necessary variables. Secret canaries and common API key shapes are redacted from traces, normalized events, validation logs, runtime evidence, terminal summaries, and result projection.

## Project-Local `.harness`

Each target repository owns its project-local `.harness/` state:

- requests and request snapshots
- artifacts
- actor prompts
- templates
- policy overlays
- reviewed learning state

The layout remains explicit and reviewable, but normal users interact through the Rail skill and Python API rather than hand-written files.

## Contributor Notes

This repository owns the Python Rail Harness Runtime, bundled Rail skill, default `.harness` assets, tests, examples, and design documents.

Use the local release gate before treating this control-plane repository as release-ready:

```bash
scripts/release_gate.sh
```

The gate proves the Rail runtime, package build, package asset inspection, installed-wheel smoke, docs guards, removed-surface guards, lint, and typing checks for this repository. It is not a downstream target application success proof.

Optional live SDK smoke is operator-gated. Set `RAIL_ACTOR_RUNTIME_LIVE_SMOKE=1` with an operator-controlled `OPENAI_API_KEY`; the release gate then enables `RAIL_ACTOR_RUNTIME_LIVE=1` for the live smoke only.

## Installation

The Python package distribution is `rail-sdk`. It exposes the Rail Python API,
bundled Rail skill assets, and setup helpers. There is no command surface as
the product contract; wrapper UX may exist later only as a thin layer over the
same API.

Install the package:

```bash
uv tool install rail-sdk
```

Install or refresh the local Rail skill, prepare Rail-owned Codex auth, then
check readiness:

```bash
rail migrate
rail auth login
rail auth doctor
```

`openai_agents_sdk` and `OPENAI_API_KEY` are optional operator paths, such as
the opt-in live SDK smoke described above. They are not required for the default
local `codex_vault` path.

If an older Homebrew `rail` binary is still first on `PATH`, use the package
name entrypoint first:

```bash
rail-sdk migrate
rail-sdk doctor
```

Then remove the old formula with the command reported by `rail doctor`. The
usual cleanup is:

```bash
brew uninstall rail
brew cleanup rail
```

After setup, open the target repository and use the Rail skill with a
natural-language task:

```text
Use the Rail skill.
Fix the profile loading bug.
```

Normal users do not need to set runtime feature flags or repeat the target path
when they are already working inside the target repository.

## Release Publishing (operator)

For public release publication, use one changelog source only:

- [CHANGELOG.md](/absolute/path/to/rail/CHANGELOG.md)

Release is now triggered by a version tag, not by local upload commands:

Homebrew is used only for cleanup of old installs; it does not drive release.

1) Run the publish script.

```bash
./publish.sh v0.6.1
```

If the top `CHANGELOG.md` entry for that version is missing, the script creates
one from changes since the previous release tag, then validates changelog
quality. If the entry already exists, the script keeps it and validates it.
After that it updates `pyproject.toml` and `uv.lock`, runs
`scripts/release_gate.sh`, commits release metadata changes when needed, pushes
`main`, and pushes the release tag.

A GitHub tag push (`v*`) triggers `.github/workflows/publish.yml`.
The workflow validates:

- `pyproject.toml` version equals tag version
- the top `CHANGELOG.md` entry matches the tag version
- local release gate
- package build
- PyPI publish through PyPI Trusted Publishing
- GitHub release notes generated from the same `CHANGELOG.md` section

Use `CHANGELOG.md` as the only user-facing release note/changelog source.
After publish, users install the version they need:

```bash
uv tool install rail-sdk==${VERSION}
```
