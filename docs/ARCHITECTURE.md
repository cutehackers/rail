# Rail Architecture

## Overview

Rail is a harness control-plane. It does not contain the downstream product being changed. Instead, it owns the runtime, supervisor policy, actor instructions, schemas, and reviewable artifact flow used to operate on another repository.

The system is designed around a simple idea: a user states intent, the supervisor routes bounded actor work, and Rail records the result as explicit artifacts that can be inspected and evolved over time.

## Core Concepts

### Supervisor

The supervisor is the control layer. It decides which actor should run next, when a run should stop, and whether evaluator feedback should trigger a bounded retry.

Its job is not to generate code directly. Its job is to manage the workflow, apply policy, and keep decisions explicit.

### Actors

Actors are the bounded specialists inside the workflow. In the current Rail model, they are responsible for steps such as:

- planning the work
- gathering context
- generating a candidate change
- executing validation
- evaluating the result
- producing an integration handoff in the `v2` flow

Each actor has a narrow responsibility and produces structured output that the rest of the system can inspect.

### Rubrics And Rules

Rubrics and rules define how Rail judges outcomes. They make the workflow reviewable by expressing expectations in configuration instead of burying them in hidden decisions.

In practice, they answer questions such as:

- what counts as a pass
- when a retry is allowed
- when a run should be blocked
- how learning and hardening reviews should be interpreted

### Artifacts

Artifacts are the persistent record of a run. They capture the request, plan, context, implementation result, execution report, evaluation result, and, in `v2`, the integration result and learning review files.

This gives Rail a traceable chain from user intent to supervisor outcome.

## Runtime Flow

The main flow is straightforward.

1. A user request is converted into a structured task.
2. Rail creates an artifact workspace for that task.
3. The supervisor dispatches actors in sequence.
4. Each actor writes structured output back into the artifact set.
5. The evaluator decides whether the run passes, retries within policy, or stops.
6. If the run passes and the operator wants the extended `v2` flow, Rail can create an integration handoff and learning review inputs.

The important property is that every transition is visible. The system is intentionally biased toward explicit routing and explicit outputs.

## How Supervisor, Actors, And Rubrics Work Together

The relationship is:

- the supervisor owns control
- actors own bounded execution
- rubrics and rules define how outcomes are judged

An actor does not decide the whole workflow. It performs its step and returns structured output. The supervisor reads that output, applies rules and rubrics, and decides the next action.

This separation matters because it keeps the system maintainable:

- actor prompts can evolve without rewriting the whole control flow
- routing policy can change without rewriting every actor
- release decisions stay reviewable because the evaluation criteria are explicit

## Artifacts And Learning State

Rail separates run artifacts from learning state.

- run artifacts describe what happened during a specific task
- learning state describes what the system should remember after reviewed outcomes

In `v2`, operators edit review files such as feedback or learning decisions. Rail then regenerates queue and evidence state from those reviewed files. This keeps derived state reproducible and easier to audit.

Approved memory is family-scoped. A promoted review updates the active approved memory for that task family instead of creating a hidden chain of mutable state.

## V1 And V2 Boundary

`v1` is the core supervisor gate. It is concerned with bounded execution and pass-or-revise control.

`v2` adds two things on top:

- an explicit integration handoff after a passing run
- explicit learning and hardening review flows for continuous improvement

This boundary is intentional. It lets Rail keep the release gate focused while still supporting longer-term quality improvement in a separate, reviewable layer.

## Repository Structure

At a high level, the repository is organized like this:

- [bin/rail.dart](bin/rail.dart) exposes the runtime entrypoint
- `lib/src/runtime/` contains runtime execution and supervisor logic
- `.harness/` contains actor instructions, rules, rubrics, templates, and learning state
- `skills/Rail/` contains the repo-owned Codex skill
- `docs/` contains release contracts, architecture notes, and operator-facing references

The design goal is not novelty. The design goal is a control-plane that stays explicit enough to review, evolve, and trust.
