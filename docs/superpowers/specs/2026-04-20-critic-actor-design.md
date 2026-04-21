# Critic Actor Design

**Date:** 2026-04-20

**Goal**

Strengthen the `rail` harness as a true multi-actor control plane by adding a mandatory `critic` actor to every task family, moving actor execution policy into checked-in harness configuration, and removing environment-variable defaults and actor-level timeouts from actor command execution.

## Problem Statement

The current harness graph is multi-stage, but its quality behavior is still too uniform:

- all Codex-backed actors are executed through the same runtime defaults
- actor model and reasoning defaults can be influenced by environment variables
- the runtime assumes an actor timeout even when the harness goal is quality-first iteration
- there is no explicit pre-generation critic role that turns likely failure patterns into generator guardrails

This means the harness can show actor traversal, but it does not yet fully exploit role asymmetry as a quality lever.

The project goal is stricter:

- every task family should use a real multi-actor graph, not just a linear sequence of similarly-configured actors
- each actor should carry a repository-owned execution profile
- actor execution policy should be reviewable in the repository, not hidden in operator environment defaults
- the graph should improve output quality by inserting a non-coding critique stage before generation
- final artifacts should make the critic's effect visible and auditable

## Scope

This spec covers:

- adding a mandatory `critic` actor to all task-family graphs
- defining the `critic_report` contract and the generator dependency on it
- adding repository-owned actor execution profiles
- removing environment-variable defaults for actor model and reasoning selection
- removing actor-level timeout configuration from actor command execution
- extending trace and execution reporting so the critic's contribution is visible

This spec does not cover:

- nested per-actor sub-agent orchestration inside a single actor run
- replacing the evaluator as the authoritative supervisor gate
- dynamic runtime mutation of actor profiles
- autonomous supervisor policy rewriting

## Design Principles

- multi-actor quality should come from role asymmetry, not just actor count
- the evaluator remains the only authoritative production gate
- the critic improves generator inputs but does not route supervisor decisions
- actor execution policy must be repository-owned and reviewable
- the harness should not rely on operator environment defaults for core actor quality settings
- actor-level timeout should not cap quality-seeking actor work
- reporting must prove that the critic changed the graph meaningfully

## Core Architecture

The new default actor graph for every task family is:

`planner -> context_builder -> critic -> generator -> executor -> evaluator`

### Planner

The planner continues to refine the user request into a bounded plan, likely file targets, assumptions, risks, and refined acceptance criteria.

### Context Builder

The context builder continues to ground the run in repository-specific context, relevant files, patterns, forbidden changes, and implementation hints.

### Critic

The critic is a non-coding pre-generation quality actor.

Its job is to inspect the current plan and repository context and emit a schema-valid critique that makes likely failure modes explicit before code generation begins. It does not patch files, run supervisor routing, or decide pass versus revise. It exists only to raise the quality floor for the generator.

### Generator

The generator remains the only coding actor in the core graph, but it now consumes the critic output as a required input rather than relying only on plan and context.

### Executor

The executor continues to collect formatting, analysis, test, and failure evidence.

### Evaluator

The evaluator remains the conservative production gate and the only actor allowed to produce supervisor decisions and corrective actions.

## Actor Contract Changes

### New Critic Contract

Inputs:

- `user_request`
- `plan`
- `context_pack`
- `forbidden_changes`
- `rubric`

Output:

- `critic_report`

The `critic_report` must be machine-readable and bounded. It is not a prose review transcript.

Required fields:

- `priority_focus`
- `missing_requirements`
- `risk_hypotheses`
- `validation_expectations`
- `generator_guardrails`
- `blocked_assumptions`

### Generator Contract Extension

The generator input contract must be extended so `critic_report` is required.

This change is deliberate. The critic must be a real graph dependency rather than an optional advisory note. A generator run without `critic_report` should be considered an invalid harness state.

### Evaluator Contract

The evaluator contract does not gain routing authority from the critic. It remains the only actor that emits `decision`, `reason_codes`, `quality_confidence`, and `next_action`.

## Supervisor and Policy Changes

The following repository-owned harness surfaces become authoritative for the critic design:

- `.harness/supervisor/task_router.yaml`
- `.harness/supervisor/registry.yaml`
- `.harness/supervisor/context_contract.yaml`
- `.harness/supervisor/actor_profiles.yaml`
- `.harness/actors/critic.md`

### Task Router

All task families must include `critic` in their actor list. This is not conditional by family or risk tolerance in the initial design.

### Registry

All task families must require `critic_report` as a produced output artifact alongside the existing actor outputs.

### Context Contract

The actor flow schema must define:

- `critic` inputs and outputs
- `generator` input dependency on `critic_report`
- workflow semantics that preserve evaluator authority as the final production gate

## Actor Profiles

Actor model and reasoning selection must move into a checked-in harness policy file:

`.harness/supervisor/actor_profiles.yaml`

Representative structure:

```yaml
version: 1

actors:
  planner:
    model: gpt-5.4
    reasoning: high
  context_builder:
    model: gpt-5.4-mini
    reasoning: high
  critic:
    model: gpt-5.4
    reasoning: high
  generator:
    model: gpt-5.4
    reasoning: high
  evaluator:
    model: gpt-5.4
    reasoning: high
```

Design rules:

- actor model and reasoning must come only from `actor_profiles.yaml`
- environment variables must not provide default actor profiles
- operator overrides for these settings are out of scope for this design
- the harness should fail clearly if a required actor profile is missing or invalid

## Runtime Changes

### Actor Command Execution

`internal/runtime/actor_runtime.go` should stop deriving default actor model and reasoning from environment variables.

Instead, actor command execution should accept an explicit actor profile loaded from repository policy.

### Runner

`internal/runtime/runner.go` should:

- load actor profiles from the checked-in supervisor policy
- resolve the current actor's model and reasoning from that policy
- pass the resolved actor profile into actor command execution
- stop applying actor-level timeouts to Codex-backed actors

### Bootstrap

`internal/runtime/bootstrap.go` should:

- materialize `critic_report.yaml` placeholders where required
- generate actor briefs for `critic`
- ensure workflow and required-output metadata include the critic stage

Smoke-path placeholder generation must also include the critic output so the graph is structurally valid in smoke mode.

## Reporting and Traceability

The harness should not only run the critic. It must also prove that the critic was used.

At minimum, the enriched execution report should include:

- `actor_graph`
- `actor_profiles_used`
- `quality_trajectory`
- `critic_findings_applied`
- `critic_to_evaluator_delta`

### actor_profiles_used

This section should record the checked-in model and reasoning actually used for each actor in the run.

### critic_findings_applied

This section should summarize which critic outputs were carried into generator expectations or later confirmed by execution and evaluation evidence.

### critic_to_evaluator_delta

This section should show the relationship between pre-generation critique and final outcome. The goal is to make critic value reviewable, for example:

- how many critic risks were later confirmed
- which missing requirements were resolved before evaluation
- which guardrails remained violated at terminal state

Terminal summaries and supervisor traces must also include the critic in traversal, visit counts, and quality history.

## Validation Plan

The design is complete only when all of the following are verified:

1. `go test ./...` passes with the critic integrated into runtime, reporting, CLI, and fixtures.
2. `go build -o build/rail ./cmd/rail` succeeds.
3. `./build/rail validate-request --request <fixture>` continues to validate canonical request fixtures.
4. `./build/rail route-evaluation --artifact <fixture-copy>` produces reports that include the critic-aware fields.
5. `./build/rail execute --artifact <smoke-artifact>` shows that every task family graph traverses the critic stage.
6. Generated reports prove that actor model and reasoning were taken from checked-in actor profiles rather than environment defaults.
7. Actor command execution runs without actor-level timeout configuration.

## Success Criteria

This design is successful when:

- every task family includes `critic` as a required actor
- the generator cannot run without a valid `critic_report`
- actor model and reasoning are repository-owned and reviewable
- environment variables are no longer the default source of actor quality policy
- actor command runs no longer use actor-level timeout configuration
- reports make the critic's traversal and effect visible to operators

## Risks

- adding the critic increases graph cost and latency for every task family
- weak critic output could become noisy prompt ballast instead of a quality amplifier
- forcing a critic report into generator input will require fixture and smoke-path updates across runtime tests
- removing actor-level timeouts improves quality latitude but raises the importance of clear process visibility in traces and logs

## Recommendation

This design should be implemented as the default harness path for all task families. The critic should be treated as a required quality actor, not an optional enhancement, because the product goal is to prove that the harness uses multi-actor structure intentionally to improve final output quality.
