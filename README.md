# rail

`rail` is a skill-first harness control-plane for Codex.

The product contract is simple:

- the user describes the task in natural language through the Rail skill
- the Rail skill writes the harness request for them
- Rail runs a bounded, reviewable workflow against a separate target repository

The point of Rail is not to make users hand-author `.harness/requests/request.yaml`. The point is to let the Rail skill turn a messy engineering request into the correct harness shape safely and consistently.

## Codex Runtime Boundary

Rail is the governance control plane. Codex remains the agent runtime.

Rail owns request normalization, bounded workflow state, artifact contracts, evaluation routing, validation evidence, and reviewed learning state. Codex owns repository inspection, file editing, tool execution, sandbox enforcement, rules, skills, hooks, and structured final actor output.

## What Users Actually Do

Install Rail once:

```bash
brew install cutehackers/rail/rail
```

Initialize the target repository once:

```bash
cd /absolute/path/to/target-repo
rail init
```

`rail init` also registers the bundled Rail skill into the active Codex user
skill root as regular files, so Codex can discover it without a source checkout
or symlinked skill directory. If that registration needs repair, run:

```bash
rail doctor
rail install-codex-skill --repair
```

After initialization, the normal entrypoint is Codex with the bundled Rail skill.

Example prompts:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the intermittent profile refresh loading issue.
Constraint: Do not change the API contract.
Definition of done: refresh completes reliably and related tests still pass.
```

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Split the club details screen build logic into smaller sections.
Constraint: Preserve behavior exactly.
Definition of done: behavior is unchanged and focused validation still passes.
```

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Verify the Rail harness wiring only.
Constraint: Smoke mode. Do not modify application source files.
Definition of done: the harness flow completes and leaves smoke evidence.
```

The skill should infer the request fields, ask only when a missing field would make the request unsafe, and materialize the request without making the user write YAML by hand.

## Two Execution Modes

### Real mode

`real` mode is the default product path.

- intended for actual product work
- Rail runs the real actor path
- the generator may edit target-repository files
- executor runs focused validation commands from the execution plan

Internally this is stored as `validation_profile: standard`.

### Smoke mode

`smoke` mode is the fast control-plane path.

- intended for harness verification, release-gate proof, or fast wiring checks
- keeps the workflow deterministic and cheap
- should not be used as the normal product-delivery path
- usually pairs with explicit constraints such as “do not modify application source files”

Internally this is stored as `validation_profile: smoke`.

Rule of thumb:

- choose `real` when the user wants the target repo actually changed and validated
- choose `smoke` when the user wants to prove the harness path itself

## Project-Local `.harness`

Each target repository owns its own local `.harness/`.

Rail keeps project-specific state there:

- project identity
- requests
- artifacts
- reviewed learning state

This state is always local to the target repository. Rail does not expect users to keep a separate shared checkout just to use the product.

## Actor Auth

Standard actor execution needs authentication, but it must not inherit the
user's normal Codex home. For local use, configure Rail actor auth once:

```bash
rail auth login
rail auth doctor
```

`rail auth login` uses Codex's browser login flow in a Rail-owned auth home.
That login persists for the local machine account across terminal sessions and
target repositories. Actor runs still use artifact-local sealed `CODEX_HOME`
directories and receive only the minimum Codex auth material needed to run.

To remove the local Rail actor auth state:

```bash
rail auth logout
```

Rail never prints stored secret values, and actor provenance records non-secret
auth metadata such as source, materialization status, and materialized file
names, not tokens.

## Advanced Overrides

Most users should stay on the bundled defaults and work through the Rail skill.

Advanced users can override selected product defaults with project-local files:

- `.harness/supervisor/`
- `.harness/supervisor/actor_backend.yaml`
- `.harness/actors/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

Override rules are intentionally simple:

1. if a project-local file exists, Rail uses it
2. otherwise Rail uses the embedded default
3. overrides are file-level, not deep merges
4. state directories such as `.harness/artifacts/`, `.harness/requests/`, and `.harness/learning/` always stay local

Advanced users should also know:

- request files use `validation_profile: standard|smoke`
- draft composition also accepts `real` as an alias for the default `standard`
- `smoke` should be treated as an explicit opt-in, not the default path
- every task family now traverses `planner -> context_builder -> critic -> generator -> executor -> evaluator`
- `critic` is a mandatory graph stage, not an optional advisory pass
- actor model and reasoning come from checked-in `.harness/supervisor/actor_profiles.yaml`
- actor Codex runs are isolated from the user's normal Codex skill/rule surface by default
- local users can run `rail auth login` once to configure Rail actor auth
- standard actor Codex runs use artifact-local `CODEX_HOME`, `HOME`, `XDG_*`, and temp directories; they use explicit Rail actor auth rather than the user's normal Codex login state
- Rail treats actor events and artifacts as governance evidence, not as conversational memory
- each run writes `.harness/artifacts/<task-id>/run_status.yaml` so the latest phase, actor, interruption reason, and next step are visible without reading raw logs
- `rail status --artifact /absolute/path/to/target-repo/.harness/artifacts/<task-id>` prints that status for operators and Codex chat sessions
- when `rail execute --artifact ...` is interrupted after an artifact exists, it prints the same status summary before returning the error
- `rail supervise --artifact /absolute/path/to/target-repo/.harness/artifacts/<task-id>` is the preferred operator command for continuing the actor loop to a terminal result because it retries retryable actor/session interruptions before surfacing a blocker
- environment variables are not the default actor-quality contract
- actor command runs do not use actor-level timeouts
- `ActorWatchdog` monitors command progress and reports `actor_watchdog_expired` when an actor stops producing observable progress

## Source Repository Role

This repository is for Rail contributors and release engineers.

It owns:

- the Go runtime and CLI
- embedded default harness assets
- the bundled Rail skill source
- release tooling and contributor docs

End users do not need this repository checked out to use Rail as a product.

## Distribution

The primary release channel is the `cutehackers/rail` Homebrew tap:

```bash
brew install cutehackers/rail/rail
```

Tagged releases publish GitHub Release artifacts from
`https://github.com/cutehackers/rail`, then update the Homebrew tap formula.
The release pipeline is GoReleaser-based and keeps the `rail` binary, bundled
Rail Codex skill, checksums, and provenance attestation in the same release
unit.

The Homebrew package installs canonical skill assets under the product prefix.
`rail init` materializes the active Codex user-facing copy, and `rail doctor`
reports whether that copy is installed, stale, missing, or symlink-based.

`homebrew/core` is a later distribution target, not the initial channel. The
tap remains the authoritative install path while Rail is still establishing its
public release cadence and package-manager notability.

## Release Checks

For contributors working on Rail itself:

- automated smoke gate: `./tool/v2_release_gate.sh`
- release workflow: `.github/workflows/release.yml`

The smoke gate proves the fast control-plane path, including request
materialization, execution, integration, artifact validation, and learning-state
verification. Real actor command wiring is covered by runtime tests that assert
profile-selected model and reasoning arguments without relying on live helper
scripts.

## More Detail

- architecture: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Korean architecture guide: [docs/ARCHITECTURE-kr.md](docs/ARCHITECTURE-kr.md)
- v1 release contract: [docs/releases/v1-core-supervisor-gate.md](docs/releases/v1-core-supervisor-gate.md)
- v2 release contract: [docs/releases/v2-integrator-and-learning-gate.md](docs/releases/v2-integrator-and-learning-gate.md)
