# rail

`rail` is a harness control-plane for Codex. It helps you turn a natural-language engineering request into a structured, reviewable workflow against a separate target repository.

The main result is simple: instead of manually coordinating prompts, task boundaries, and review handoffs, you can use the Rail skill to run a bounded workflow that produces artifacts, decisions, and a clear supervisor-managed outcome.

## Install

Requirements:

- Homebrew
- Codex

Setup:

```bash
brew install rail
```

For local verification from this source checkout, you can also build from the bundled formula:

```bash
brew install --build-from-source ./packaging/homebrew/rail.rb
```

Packaged installs bundle the Rail Codex skill automatically. This source repository remains the development and release origin, not the required runtime root.

## What You Get From Rail

Rail is designed for operators who want a predictable harness, not an ad hoc prompt chain.

With Rail, you can:

- start from intent instead of hand-writing harness YAML
- run a bounded supervisor workflow against a separate project repository
- keep planning, execution, evaluation, and integration outputs in one traceable place
- separate the `v1` release gate from the `v2` learning and review flow
- make supervisor decisions explicit enough to review and improve over time

## Using Rail Through The Codex Skill

The main merit of Rail is not the CLI by itself. The main merit is that the Rail Codex skill lets the user stay at the level of goal, constraints, and done criteria while the harness handles structure and routing.

Typical skill usage looks like this:

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
Goal: Refactor the settings flow safely without changing user-visible behavior.
Constraint: Keep the existing analytics events unchanged.
Definition of done: behavior is preserved and the change remains easy to review.
```

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Implement the first pass of offline retry handling for failed uploads.
Constraint: Keep the scope small and release-safe.
Definition of done: failed uploads can be retried and the change includes clear follow-up risks.
```

In practice, the skill structures the request, applies the repo-owned harness policy, and starts the correct workflow for the target repository.

## V1 Workflow

`v1` is the core supervisor gate.

At a high level:

1. Rail turns the user request into a structured task.
2. The supervisor routes the task through bounded actors such as planning, context building, generation, execution, and evaluation.
3. The evaluator decides whether the run passes, needs a bounded retry, or should stop.
4. The result is a reviewable artifact set for the target repository.

`v1` is focused on getting to a controlled execution outcome.

## V2 Workflow

`v2` builds on `v1` by adding explicit integration and learning review flows.

At a high level:

1. A passed `v1` run can produce an integration handoff.
2. Operators can review outcome feedback, learning candidates, and hardening candidates.
3. Rail regenerates queue and evidence state from reviewed files instead of relying on hidden mutable state.
4. Approved learning can be promoted in a controlled, same-family way.

`v2` is focused on making improvement loops explicit, reviewable, and safe to evolve.

## Architecture

The user guide stays intentionally lightweight. If you want the system-level explanation of how the supervisor, actors, rubrics, artifacts, and learning state work together, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

For Korean documentation, see [README-kr.md](README-kr.md) and [docs/ARCHITECTURE-kr.md](docs/ARCHITECTURE-kr.md).

## Current Scope

The `v1` runtime supports:

- request composition and validation
- artifact bootstrap
- actor brief generation
- sequential actor execution with `codex exec`
- evaluator-driven `revise` handling back to generator
- validation profiles (`standard`, `smoke`) for executor planning
- deterministic smoke fast-path execution for planner/context_builder/generator/executor/evaluator
- request-level validation roots and targets for narrowing standard-profile executor scope
- supervisor action loops that can route from evaluator back to generator, context_builder, or executor with bounded budgets
- review drafts are operator-authored, while queue, evidence, and approved-memory files are rail-derived snapshots
- approved memory is same-family only, with one active approved file per family
- pending review backlog is allowed, but broken derived state is not

The `v1` scope is intentionally constrained and does not treat these as in-gate:

- parallel actor orchestration
- project-specific adapters beyond the default Flutter + Riverpod profile
- `integrator`
- quality-learning review and apply flows
- hardened end-to-end validation across all task types

The `v2` operator surface now includes explicit file-based `init-*` and `apply-*` learning workflows. Operators edit the draft and decision files; rail regenerates the queue and evidence snapshots, and updates approved memory only on `promote` at `.harness/learning/approved/<task_family>.yaml`.
