# V2 Approved-Memory Operations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `v2` approved-memory reuse, queue snapshots, and family evidence state operationally bounded and release-verifiable without making pending review backlog a release blocker.

**Architecture:** Keep all mutation inside the existing review/apply flow in `HarnessRunner`, and expose a separate read-only verification surface for derived learning state. The implementation should preserve one active approved-memory file per family, regenerate queues/evidence from reviewed state, and let the `v2` gate fail only on broken operational state rather than on pending backlog.

**Tech Stack:** Dart CLI, `HarnessRunner`, YAML schemas/templates under `.harness/templates/`, shell gate script, `package:test`

---

### Task 1: Lock approved-memory lifecycle semantics into runtime tests

**Files:**
- Modify: `lib/src/runtime/harness_runner.dart`
- Modify: `test/runtime/review_flow_apply_command_test.dart`
- Create: `test/runtime/approved_memory_consideration_test.dart`

- [ ] **Step 1: Extend the failing apply-flow tests for approved-memory lifecycle**

Add coverage to `test/runtime/review_flow_apply_command_test.dart` for:
- `promote` writing only the canonical approved-memory path for the family
- a malformed `promote` decision that targets a non-canonical approved-memory path and is rejected
- a second same-family `promote` overwriting the same approved file instead of creating another one
- `hold` and `reject` persisting review decisions without deleting an existing approved file

Use the same temp harness-root pattern already present in the file so the tests stay isolated under `.dart_tool/`.

- [ ] **Step 2: Add failing approved-memory consideration tests**

Create `test/runtime/approved_memory_consideration_test.dart` with focused cases that build temporary harness roots and assert:
- a healthy reviewed same-family approved file yields `approved_memory_consideration.disposition == reuse`
- a canonical file whose latest evidence is stale or conflicting yields `quarantine`
- a family/path mismatch yields `drop`

Prefer using the existing `.harness/fixtures/approved-memory/*.yaml` files as source material for mismatch cases instead of inventing new fixture formats.

- [ ] **Step 3: Run the focused runtime tests to verify failure**

Run: `dart test test/runtime/review_flow_apply_command_test.dart test/runtime/approved_memory_consideration_test.dart`

Expected:
- FAIL because lifecycle guarantees are not fully asserted yet, and at least one reuse/drop/quarantine case is either missing or implemented too loosely for the new tests.

- [ ] **Step 4: Implement the minimal lifecycle/runtime changes**

In `lib/src/runtime/harness_runner.dart`:
- keep one canonical approved-memory path per family
- reject `promote` decisions that target any non-canonical approved-memory path
- preserve existing approved files on `hold` and `reject`
- keep reuse bounded by the existing same-family, compatibility, and evidence-conflict checks
- tighten any logic needed so the new tests pass without adding archive/version-file behavior

Do not add time-based expiration, extra approved-memory directories, or any mutation path outside the existing apply commands.

- [ ] **Step 5: Re-run the focused runtime tests**

Run: `dart test test/runtime/review_flow_apply_command_test.dart test/runtime/approved_memory_consideration_test.dart`

Expected:
- PASS

- [ ] **Step 6: Commit the lifecycle semantics**

Run:
```bash
git add lib/src/runtime/harness_runner.dart test/runtime/review_flow_apply_command_test.dart test/runtime/approved_memory_consideration_test.dart
git commit -m "test: lock approved-memory lifecycle semantics"
```

### Task 2: Add a read-only derived-state verification surface

**Files:**
- Modify: `lib/src/runtime/harness_runner.dart`
- Modify: `lib/src/cli/rail_cli.dart`
- Modify: `test/cli/cli_dispatch_test.dart`
- Create: `test/runtime/learning_state_verification_test.dart`

- [ ] **Step 1: Write the failing verification tests first**

Create `test/runtime/learning_state_verification_test.dart` around a temporary harness root that can:
- seed reviewed candidates, review decisions, and approved memory
- call a new read-only verification entrypoint
- assert success for healthy state
- assert failure for:
  - invalid approved-memory schema
  - queue drift
  - family evidence drift
- assert success when pending learning or hardening backlog exists but the snapshots are otherwise coherent

Also extend `test/cli/cli_dispatch_test.dart` so usage output and dispatch include the new verification command.

- [ ] **Step 2: Run the focused verification tests to verify failure**

Run: `dart test test/cli/cli_dispatch_test.dart test/runtime/learning_state_verification_test.dart`

Expected:
- FAIL because there is no public verification command yet and no runtime method that checks derived learning state without mutating it.

- [ ] **Step 3: Implement a read-only verification command**

Add a new CLI/runtime surface with a name that clearly communicates “check, don’t mutate,” for example:
- `verify-learning-state`

Implementation requirements:
- validate every active file under `.harness/learning/approved/` against `approved_family_memory`
- enforce canonical family-path naming for active approved-memory files
- rebuild the expected `review_queue.yaml`, `hardening_queue.yaml`, and `family_evidence_index.yaml` from current candidate/decision/approved state without writing them
- compare regenerated content against the checked-in snapshot files
- return actionable drift reasons on failure
- leave `apply-*` as the only mutating path

Keep the verification logic in `HarnessRunner`; keep `RailCli` as thin command dispatch.

- [ ] **Step 4: Re-run the focused verification tests**

Run: `dart test test/cli/cli_dispatch_test.dart test/runtime/learning_state_verification_test.dart`

Expected:
- PASS

- [ ] **Step 5: Commit the verification surface**

Run:
```bash
git add lib/src/runtime/harness_runner.dart lib/src/cli/rail_cli.dart test/cli/cli_dispatch_test.dart test/runtime/learning_state_verification_test.dart
git commit -m "feat: verify derived learning state"
```

### Task 3: Wire the v2 release gate to operational-state verification

**Files:**
- Modify: `tool/v2_release_gate.sh`
- Modify: `test/runtime/learning_state_verification_test.dart`

- [ ] **Step 1: Add a failing gate-contract test case**

Extend `test/runtime/learning_state_verification_test.dart` (or add a small companion test in the same file) so it covers the release-gate interpretation:
- healthy state passes verification
- pending backlog does not fail verification
- invalid approved memory or derived-state drift fails verification

This test should exercise the same runtime/CLI surface the shell gate will call, not a private helper that the shell script cannot reach.

- [ ] **Step 2: Run the focused gate-contract tests to verify failure**

Run: `dart test test/runtime/learning_state_verification_test.dart`

Expected:
- FAIL until the command semantics and exit behavior line up with the gate contract.

- [ ] **Step 3: Update the shell gate**

Modify `tool/v2_release_gate.sh` so it:
- keeps the existing `pub get`, `analyze`, `test`, compile, smoke `run`, `execute`, and `integrate` checks
- still validates `integration_result.yaml`
- still fails when `release_readiness: blocked`
- now also runs the new read-only learning-state verification command and fails on drift

Do not add any logic that fails just because there are pending review or hardening candidates.

- [ ] **Step 4: Re-run the focused gate-contract tests**

Run: `dart test test/runtime/learning_state_verification_test.dart`

Expected:
- PASS

- [ ] **Step 5: Commit the gate wiring**

Run:
```bash
git add tool/v2_release_gate.sh test/runtime/learning_state_verification_test.dart
git commit -m "feat: gate v2 on learning-state consistency"
```

### Task 4: Update operator documentation to match the bounded lifecycle

**Files:**
- Modify: `README.md`
- Modify: `docs/releases/v2-integrator-and-learning-gate.md`
- Modify: `docs/backlog/v2-integrator-and-learning.md`

- [ ] **Step 1: Update README scope and operator guidance**

Document clearly that:
- approved memory is same-family only
- there is one active approved file per family
- previous approved content is tracked through git history rather than archive files
- queue and evidence files are rail-derived snapshots, not operator-edited files
- pending review backlog is allowed, but broken derived state is not

- [ ] **Step 2: Update release and backlog docs**

Align `docs/releases/v2-integrator-and-learning-gate.md` and `docs/backlog/v2-integrator-and-learning.md` with the implemented lifecycle:
- operator-authored inputs vs rail-derived state
- canonical approved-memory path rules
- non-blocking backlog policy
- gate responsibility for broken operational state

- [ ] **Step 3: Run the targeted verification set after doc updates**

Run:
```bash
dart test test/runtime/review_flow_apply_command_test.dart test/runtime/approved_memory_consideration_test.dart test/runtime/learning_state_verification_test.dart test/cli/cli_dispatch_test.dart
dart analyze
```

Expected:
- PASS

- [ ] **Step 4: Run the full v2 gate once**

Run:
```bash
./tool/v2_release_gate.sh
```

Expected:
- PASS on a healthy tree
- no failure caused only by pending review backlog

- [ ] **Step 5: Commit the docs and final verification state**

Run:
```bash
git add README.md docs/releases/v2-integrator-and-learning-gate.md docs/backlog/v2-integrator-and-learning.md
git commit -m "docs: describe bounded approved-memory operations"
```

### Task 5: Final integration handoff

**Files:**
- Modify: none required unless verification finds drift

- [ ] **Step 1: Run the full targeted suite one more time**

Run:
```bash
dart test
dart analyze
./tool/v2_release_gate.sh
```

Expected:
- PASS

- [ ] **Step 2: Summarize the resulting v2 boundary**

Capture in the handoff summary:
- what files are operator-authored
- what files are rail-derived
- how same-family reuse is bounded
- what the new gate now enforces

- [ ] **Step 3: Create the final implementation commit**

Run:
```bash
git status --short
git add \
  lib/src/runtime/harness_runner.dart \
  lib/src/cli/rail_cli.dart \
  tool/v2_release_gate.sh \
  test/runtime/review_flow_apply_command_test.dart \
  test/runtime/approved_memory_consideration_test.dart \
  test/runtime/learning_state_verification_test.dart \
  test/cli/cli_dispatch_test.dart \
  README.md \
  docs/releases/v2-integrator-and-learning-gate.md \
  docs/backlog/v2-integrator-and-learning.md
git commit -m "feat: operationalize v2 approved-memory reuse"
```
