# Review-Driven Learning Flows Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add file-based `init-*` and `apply-*` commands for user outcome feedback, learning review, and hardening review so `v2` learning flows are explicit operator workflows.

**Architecture:** Keep review-driven learning outside the `v1` execution path. Add CLI commands that either generate draft YAML files from an artifact or candidate reference, or validate and apply existing review artifacts through `HarnessRunner`, reusing the existing learning-store refresh logic.

**Tech Stack:** Dart CLI, existing `HarnessRunner`, YAML schemas/templates, `package:test`

---

### Task 1: Expose the v2 review-flow command surface

**Files:**
- Modify: `lib/src/cli/rail_cli.dart`
- Modify: `test/cli/cli_dispatch_test.dart`

- [ ] **Step 1: Write the failing CLI tests**

Add expectations for:
- `init-user-outcome-feedback`
- `init-learning-review`
- `init-hardening-review`
- `apply-user-outcome-feedback`
- `apply-learning-review`
- `apply-hardening-review`

Also add one command-dispatch test proving a new init/apply command routes to the runtime instead of falling through to usage.

- [ ] **Step 2: Run the focused CLI tests to verify failure**

Run: `dart test test/cli/cli_dispatch_test.dart`
Expected: FAIL because usage output and dispatch do not yet include the new commands.

- [ ] **Step 3: Add CLI command parsing**

Implement file-based command parsing:
- `init-user-outcome-feedback --artifact <artifact-dir> [--output <path>]`
- `init-learning-review --candidate <quality-candidate-ref> [--output <path>]`
- `init-hardening-review --candidate <hardening-candidate-ref> [--output <path>]`
- `apply-user-outcome-feedback --file <path>`
- `apply-learning-review --file <path>`
- `apply-hardening-review --file <path>`
- Keep `--file` as the canonical apply flag, but preserve `--feedback` / `--decision` as compatibility aliases while docs migrate.

- [ ] **Step 4: Re-run the focused CLI tests**

Run: `dart test test/cli/cli_dispatch_test.dart`
Expected: PASS

### Task 2: Add draft-generation runtime methods

**Files:**
- Modify: `lib/src/runtime/harness_runner.dart`
- Create: `test/runtime/review_flow_init_test.dart`

- [ ] **Step 1: Write failing runtime tests for draft generation**

Cover:
- generating user outcome feedback from an artifact directory
- generating learning review from a quality candidate ref
- generating hardening review from a hardening candidate ref
- default output paths under:
  - `.harness/learning/feedback/`
  - `.harness/learning/reviews/`
  - `.harness/learning/hardening-reviews/`

- [ ] **Step 2: Run the focused runtime tests to verify failure**

Run: `dart test test/runtime/review_flow_init_test.dart`
Expected: FAIL because the init methods do not exist yet.

- [ ] **Step 3: Implement minimal draft-generation methods**

Add runtime methods that:
- load the source artifact/candidate
- prefill required schema fields from existing evidence
- use safe schema-valid defaults for operator-authored fields
  - `feedback_classification: unresolved`
  - review decisions default to `hold`
  - reviewer/reason fields are emitted as explicit `TODO:` placeholders
- write YAML draft files to the default directory unless `--output` overrides it
- validate each generated artifact against its schema before returning

- [ ] **Step 4: Re-run the focused runtime tests**

Run: `dart test test/runtime/review_flow_init_test.dart`
Expected: PASS

### Task 3: Keep apply flows explicit and file-based

**Files:**
- Modify: `lib/src/runtime/harness_runner.dart`
- Create: `test/runtime/review_flow_apply_command_test.dart`

- [ ] **Step 1: Write failing tests for apply command integration**

Cover:
- applying a generated user outcome feedback file
- applying a generated learning review decision file
- applying a generated hardening review decision file
- verifying the existing refresh/store side effects are still reached through the CLI

- [ ] **Step 2: Run the focused apply tests to verify failure**

Run: `dart test test/runtime/review_flow_apply_command_test.dart`
Expected: FAIL because CLI surface does not yet expose the apply commands end-to-end.

- [ ] **Step 3: Implement only the missing integration glue**

Do not rewrite the existing apply logic. Add only the runtime/CLI glue needed to:
- call the existing `applyUserOutcomeFeedback`
- call the existing `applyLearningReview`
- call the existing `applyHardeningReview`
- emit operator-facing completion messages that point to the updated refs

- [ ] **Step 4: Re-run the focused apply tests**

Run: `dart test test/runtime/review_flow_apply_command_test.dart`
Expected: PASS

### Task 4: Document the v2 operator workflow

**Files:**
- Modify: `README.md`
- Modify: `docs/releases/v2-integrator-and-learning-gate.md`
- Modify: `docs/backlog/v2-integrator-and-learning.md`

- [ ] **Step 1: Add the v2 operator commands to docs**

Document:
- when to use `init-*`
- where generated drafts live by default
- when to use each `apply-*` command
- that `--feedback` / `--decision` remain accepted compatibility aliases while `--file` is the documented canonical flag

- [ ] **Step 2: Keep scope language accurate**

Update release/backlog docs so Workstream 2 is described as explicit file-based operator workflows and not hidden runtime adaptation.

- [ ] **Step 3: Run the relevant focused tests after doc updates**

Run: `dart test test/cli/cli_dispatch_test.dart test/runtime/review_flow_init_test.dart test/runtime/review_flow_apply_command_test.dart`
Expected: PASS
