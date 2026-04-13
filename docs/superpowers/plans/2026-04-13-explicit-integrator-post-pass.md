# Explicit Integrator Post-Pass Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `integrate` as an explicit post-pass handoff command without reopening the `v1` core supervisor gate.

**Architecture:** Keep `integrator` outside the main task registry and outside `execute`. Add a dedicated CLI command that runs only after evaluator has already passed, produces `integration_result.yaml`, and leaves evaluator pass/fail semantics untouched.

**Tech Stack:** Dart CLI, existing `HarnessRunner`, YAML schemas, `package:test`

---

### Task 1: Define the explicit integrator contract

**Files:**
- Modify: `.harness/supervisor/context_contract.yaml`
- Review: `.harness/actors/integrator.md`
- Review: `.harness/templates/integration_result.schema.yaml`

- [ ] Add an `integrator` actor contract that uses already-materialized run artifacts as input.
- [ ] Keep `integrator` listed under `deferred_to_v2` so `v1` scope stays unchanged.
- [ ] Confirm the contract only supports post-pass handoff output and not routing changes.

### Task 2: Add the dedicated CLI command

**Files:**
- Modify: `lib/src/cli/rail_cli.dart`

- [ ] Add `integrate --artifact <path> [--project-root <path>]` to the CLI.
- [ ] Route the command to a new runtime method instead of extending `execute`.
- [ ] Update usage output so the operator surface is explicit.

### Task 3: Implement explicit integrator execution

**Files:**
- Modify: `lib/src/runtime/harness_runner.dart`

- [ ] Add a runtime method that loads an existing artifact directory and requires an evaluator `pass`.
- [ ] Reuse the existing actor-brief and `codex exec` machinery to produce `integration_result.yaml`.
- [ ] Keep state terminal semantics unchanged; the command adds handoff output but does not reopen the supervisor loop.

### Task 4: Cover the new command with focused tests

**Files:**
- Modify: `test/cli/cli_dispatch_test.dart`
- Create: `test/runtime/integrator_command_test.dart`

- [ ] Add a CLI test showing that `integrate` is part of the supported command surface.
- [ ] Add a runtime test that runs `run -> execute -> integrate` and validates `integration_result.yaml`.
- [ ] Assert that integration requires a passed evaluation artifact.

### Task 5: Document the new post-pass entrypoint

**Files:**
- Modify: `README.md`

- [ ] Add a short note that `integrate` is a post-pass handoff command, not part of the `v1` release gate.
- [ ] Keep `## Current scope` accurate: `v1` still excludes `integrator`.
