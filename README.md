# rail

`rail` is a skill-first harness control-plane for Codex.

The product contract is simple:

- the user describes the task in natural language through the Rail skill
- the Rail skill writes the structured harness request for them
- Rail runs a bounded, reviewable workflow against a separate target repository

The point of Rail is not to make users hand-author `.harness/requests/request.yaml`. The point is to let the Rail skill turn a messy engineering request into the correct harness shape safely and consistently.

## What Users Actually Do

Install Rail once:

```bash
brew install rail
```

Initialize the target repository once:

```bash
cd /absolute/path/to/target-repo
rail init
```

After that, the normal entrypoint is Codex with the bundled Rail skill.

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

## Advanced Overrides

Most users should stay on the bundled defaults and work through the Rail skill.

Advanced users can override selected product defaults with project-local files:

- `.harness/supervisor/`
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
- real actor invocation defaults to `RAIL_ACTOR_MODEL=gpt-5.4-mini`
- override actor model with `RAIL_ACTOR_MODEL`
- override actor reasoning effort with `RAIL_ACTOR_REASONING_EFFORT`

## Source Repository Role

This repository is for Rail contributors and release engineers.

It owns:

- the Go runtime and CLI
- embedded default harness assets
- the bundled Rail skill source
- release tooling and contributor docs

End users do not need this repository checked out to use Rail as a product.

## Release Checks

For contributors working on Rail itself:

- automated smoke gate: `./tool/v2_release_gate.sh`
- manual real-mode gate: `./tool/real_mode_check.sh`

The smoke gate proves the fast control-plane path.
The real-mode gate proves that the real actor path can execute against a live target repo with actual Codex actor invocation.

## More Detail

- architecture: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Korean architecture guide: [docs/ARCHITECTURE-kr.md](docs/ARCHITECTURE-kr.md)
- v1 release contract: [docs/releases/v1-core-supervisor-gate.md](docs/releases/v1-core-supervisor-gate.md)
- v2 release contract: [docs/releases/v2-integrator-and-learning-gate.md](docs/releases/v2-integrator-and-learning-gate.md)
