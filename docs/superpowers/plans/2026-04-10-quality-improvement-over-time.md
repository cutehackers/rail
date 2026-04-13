# Quality Improvement Over Time Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the full review-only quality-learning model so `rail` can accumulate same-family quality knowledge over time, while keeping runtime policy stable and making every learning effect reviewable from artifacts.

**Architecture:** Use request `task_type` as the authoritative `task_family`, emit one or more per-run `quality_learning_candidate` artifacts plus separate `hardening_candidate` artifacts, require explicit human review decisions before anything becomes `approved_family_memory`, maintain an authoritative family evidence index for freshness checks, and inject only approved memory as a bounded, non-authoritative context supplement with explicit `reuse|quarantine|drop` provenance.

**Tech Stack:** Dart CLI runtime in `bin/rail.dart`, YAML contracts and schemas under `.harness/supervisor` and `.harness/templates`, checked-in evidence under `.harness/artifacts`, learning state under `.harness/learning`

---

### Task 1: Define the Full Learning Contract Before Runtime Depends on It

**Files:**
- Modify: `.harness/supervisor/context_contract.yaml`
- Modify: `.harness/supervisor/policy.yaml`
- Modify: `.harness/templates/context_pack.schema.yaml`
- Modify: `.harness/templates/execution_report.schema.yaml`
- Create: `.harness/templates/quality_learning_candidate.schema.yaml`
- Create: `.harness/templates/hardening_candidate.schema.yaml`
- Create: `.harness/templates/approved_family_memory.schema.yaml`
- Create: `.harness/templates/learning_review_decision.schema.yaml`
- Create: `.harness/templates/hardening_review_decision.schema.yaml`
- Create: `.harness/templates/user_outcome_feedback.schema.yaml`
- Create: `.harness/templates/family_evidence_index.schema.yaml`
- Create: `.harness/templates/quality_improvement_comparison.schema.yaml`
- Create: `.harness/templates/learning_review_queue.schema.yaml`
- Create: `.harness/templates/hardening_review_queue.schema.yaml`
- Create: `.harness/fixtures/invalid-learning/forbidden-policy-candidate.yaml`
- Create: `.harness/fixtures/invalid-learning/forbidden-policy-approved-memory.yaml`

- [ ] **Step 1: Bind `task_type` to `task_family` in the contracts**

Update `.harness/supervisor/context_contract.yaml` so it explicitly says:
- request `task_type` is the authoritative `task_family`
- runtime may emit zero or more `quality_learning_candidate` artifacts per run
- runtime may emit separate `hardening_candidate` artifacts for policy-affecting observations
- only reviewed and approved family memory may be reused
- unreviewed candidates, review queues, and hardening queues must never influence later runs directly
- the contract carries an explicit version field used by freshness checks

- [ ] **Step 2: Make the review-only boundary explicit in policy**

Update `.harness/supervisor/policy.yaml` to state:
- run-end candidate emission is allowed
- candidate promotion requires a human-authored `learning_review_decision`
- hardening candidates are excluded from reusable family memory
- policy changes remain separate hardening work
- approved memory is guidance only and must never override the current request, current repository state, or evaluator policy
- the policy file carries an explicit version field used by freshness checks

- [ ] **Step 3: Fully define `approved_family_memory` as a first-class prerequisite**

Create `.harness/templates/approved_family_memory.schema.yaml` before any runtime promotion or reuse work. The schema must require:
- `task_family`
- `task_family_source`
- approved observation
- applicability conditions
- evidence basis
- guardrail note
- freshness marker tied to policy and repository assumptions
- disposition history
- originating candidate refs

The schema must also be explicitly closed against policy-like payloads. Add restrictions so fields or sections that would imply policy mutation are rejected, for example:
- `next_action`
- routing overrides
- pass-bar overrides
- evaluator-policy overrides

The freshness marker must carry explicit version fields, including:
- contract version
- policy version
- memory schema version

- [ ] **Step 4: Define the candidate and review schemas**

Create `.harness/templates/quality_learning_candidate.schema.yaml` with required fields for:
- originating run artifact identity
- one candidate identifier
- `task_family`
- `task_family_source`
- quality outcome summary
- structured `user_outcome_signal`
- structured `effective_context_signal`
- structured `effective_validation_signal`
- structured `evaluator_support_signal`
- candidate claim
- supporting evidence refs
- guardrail cost
- runtime recommendation: `promote|hold|reject`

This schema must also be explicitly closed against policy-like payloads. Add restrictions so fields or sections that would imply policy mutation are rejected rather than merely ignored.

`user_outcome_signal` must require:
- status: `confirmed|provisional|unavailable`
- supporting feedback refs when confirmed

`evaluator_support_signal` must require:
- `quality_confidence`
- final `reason_codes`
- validation sufficiency assessment
- terminal outcome class

Create `.harness/templates/hardening_candidate.schema.yaml` with required fields for:
- originating run artifact identity
- candidate identifier
- `task_family`
- policy-affecting observation
- why it must not become reusable family memory
- supporting evidence refs
- hardening recommendation

Create `.harness/templates/learning_review_decision.schema.yaml` with required fields for:
- candidate ref
- reviewer decision: `promote|hold|reject`
- reviewer identity
- decision timestamp
- decision reason
- resulting approved-memory ref when promoted

The decision schema should also support a review predicate outcome for guardrail cost, so a candidate can be held or rejected when an apparent improvement is explained only by excessive intervention cost.

Create `.harness/templates/hardening_review_decision.schema.yaml` with required fields for:
- hardening candidate ref
- reviewer decision such as `accept_for_hardening|hold|reject`
- reviewer identity
- decision timestamp
- decision reason
- optional follow-up hardening note ref

Create `.harness/templates/user_outcome_feedback.schema.yaml` with required fields for:
- originating run artifact identity
- `task_family`
- feedback classification showing whether the user outcome is accepted, corrected, or unresolved
- evidence refs back to the run and any follow-up artifact that confirms the outcome

Create `.harness/templates/quality_improvement_comparison.schema.yaml` with required fields for:
- baseline run ref
- follow-up run ref
- `task_family`
- primary user-outcome comparison
- supporting evaluator delta
- reviewer conclusion on whether the later run is more production-ready

- [ ] **Step 5: Define the authoritative evidence index and queues**

Create `.harness/templates/family_evidence_index.schema.yaml` for the read-only freshness source. It must capture:
- latest approved memory refs by family
- latest confirmed success refs by family
- latest failure refs by family
- latest review decision refs by family
- timestamps or sequence markers needed for freshness evaluation

Create `.harness/templates/learning_review_queue.schema.yaml` and `.harness/templates/hardening_review_queue.schema.yaml`.

`learning_review_queue` must carry:
- pending candidate refs grouped by family
- `task_family_source`
- direct user outcome status
- last disposition state
- latest review decision ref

`hardening_review_queue` must carry:
- pending hardening candidate refs
- why each entry is policy-affecting
- review state

- [ ] **Step 6: Validate schema rejection for policy-like payloads**

Create invalid fixtures for the new closed schemas, then run:

```bash
dart run bin/rail.dart validate-artifact --file .harness/fixtures/invalid-learning/forbidden-policy-candidate.yaml --schema quality_learning_candidate
dart run bin/rail.dart validate-artifact --file .harness/fixtures/invalid-learning/forbidden-policy-approved-memory.yaml --schema approved_family_memory
```

Expected:
- both commands exit non-zero
- policy-like payloads are rejected rather than tolerated

- [ ] **Step 7: Extend context and execution reporting schemas**

Update `.harness/templates/context_pack.schema.yaml` so the runtime has a bounded place for approved-memory guidance, for example:
- `approved_memory_hints`

Update `.harness/templates/execution_report.schema.yaml` with compact fields for:
- `quality_learning_candidate_refs`
- `hardening_candidate_refs`
- `approved_memory_usage`
- `approved_memory_disposition`
- `approved_memory_originating_candidate_refs`
- the lookup key and source used for approved-memory reuse

- [ ] **Step 8: Commit the contract/schema slice**

```bash
git add .harness/supervisor/context_contract.yaml .harness/supervisor/policy.yaml .harness/templates/context_pack.schema.yaml .harness/templates/execution_report.schema.yaml .harness/templates/quality_learning_candidate.schema.yaml .harness/templates/hardening_candidate.schema.yaml .harness/templates/approved_family_memory.schema.yaml .harness/templates/learning_review_decision.schema.yaml .harness/templates/hardening_review_decision.schema.yaml .harness/templates/user_outcome_feedback.schema.yaml .harness/templates/family_evidence_index.schema.yaml .harness/templates/quality_improvement_comparison.schema.yaml .harness/templates/learning_review_queue.schema.yaml .harness/templates/hardening_review_queue.schema.yaml .harness/fixtures/invalid-learning/forbidden-policy-candidate.yaml .harness/fixtures/invalid-learning/forbidden-policy-approved-memory.yaml
git commit -m "feat: define full review-only quality learning contracts"
```

### Task 2: Emit Multiple Runtime Candidates and Separate Hardening Candidates

**Files:**
- Modify: `bin/rail.dart`
- Modify: `.harness/templates/quality_learning_candidate.schema.yaml`
- Modify: `.harness/templates/hardening_candidate.schema.yaml`
- Modify: `.harness/templates/learning_review_queue.schema.yaml`
- Modify: `.harness/templates/hardening_review_queue.schema.yaml`
- Modify: `.harness/templates/family_evidence_index.schema.yaml`
- Reference existing: `.harness/requests/rail-standard-beacon-auto.yaml`

- [ ] **Step 1: Register the new artifact schemas**

Update `bin/rail.dart` so `validate-artifact` recognizes:
- `quality_learning_candidate`
- `hardening_candidate`
- `approved_family_memory`
- `learning_review_decision`
- `family_evidence_index`
- `learning_review_queue`
- `hardening_review_queue`

- [ ] **Step 2: Carry the authoritative family key through runtime**

Update `bin/rail.dart` to read request `task_type` and carry it through as runtime `task_family` and `task_family_source`. Do not infer family from changed files, evaluator output, or actor names.

- [ ] **Step 3: Emit one or more quality-learning candidates per run**

Update `bin/rail.dart` so each completed, refused, blocked, split, or exhausted run may write multiple candidate files under the artifact directory, for example:

```text
.harness/artifacts/<task-id>/quality_learning_candidates/<id>.yaml
```

Split candidates by materially distinct claim, such as:
- effective context claim
- effective validation claim
- recurring failure-pattern claim

Keep this bounded:
- emit a single candidate by default
- emit multiple candidates only when there are multiple clearly distinct, well-supported claims
- cap candidate emission per run to a small fixed maximum, for example `3`

- [ ] **Step 4: Emit separate hardening candidates**

Update `bin/rail.dart` so policy-affecting observations are written separately, for example:

```text
.harness/artifacts/<task-id>/hardening_candidates/<id>.yaml
```

These must never be treated as reusable family memory.

- [ ] **Step 5: Prefer confirmed user outcome when building candidate summaries**

Update runtime candidate assembly so the quality summary:
- uses confirmed direct user outcome when available
- falls back to evaluator confidence only when direct user outcome is provisional or unavailable
- keeps that distinction explicit in the candidate payload

Also make runtime populate the structured signal sections explicitly:
- `effective_context_signal` must say which context additions helped, failed, or were neutral
- `effective_validation_signal` must say which validation evidence materially supported, failed to support, or contradicted the result
- `evaluator_support_signal` must always capture `quality_confidence`, final `reason_codes`, validation sufficiency, and terminal outcome class

- [ ] **Step 6: Update the review queues and evidence index**

Update `bin/rail.dart` so each run refreshes:
- `.harness/learning/review_queue.yaml`
- `.harness/learning/hardening_queue.yaml`
- `.harness/learning/family_evidence_index.yaml`

The family evidence index should be the authoritative read-only source for latest family success and failure evidence. Queue entries are not sufficient on their own.

- [ ] **Step 7: Validate runtime-emitted candidate artifacts**

Use the existing checked-in request fixture `.harness/requests/rail-standard-beacon-auto.yaml`, then run:

Run:

```bash
dart run bin/rail.dart run --request .harness/requests/rail-standard-beacon-auto.yaml --project-root . --task-id 2026-04-10-quality-learning-candidate --force
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-quality-learning-candidate/quality_learning_candidates/01.yaml --schema quality_learning_candidate
dart run bin/rail.dart validate-artifact --file .harness/learning/review_queue.yaml --schema learning_review_queue
dart run bin/rail.dart validate-artifact --file .harness/learning/hardening_queue.yaml --schema hardening_review_queue
dart run bin/rail.dart validate-artifact --file .harness/learning/family_evidence_index.yaml --schema family_evidence_index
```

Expected:
- the run completes or terminates normally
- one or more `quality_learning_candidate` files exist
- hardening candidates, if any, exist only in the hardening path
- the evidence index updates independently of queue state

- [ ] **Step 8: Commit the runtime candidate slice**

```bash
git add bin/rail.dart .harness/learning/review_queue.yaml .harness/learning/hardening_queue.yaml .harness/learning/family_evidence_index.yaml
git commit -m "feat: emit full quality learning candidate sets"
```

### Task 3: Implement Manual Review Promotion, Hold, Reject, and Audit Trail

**Files:**
- Modify: `bin/rail.dart`
- Modify: `.harness/templates/approved_family_memory.schema.yaml`
- Modify: `.harness/templates/learning_review_decision.schema.yaml`
- Modify: `.harness/templates/user_outcome_feedback.schema.yaml`
- Modify: `.harness/templates/learning_review_queue.schema.yaml`
- Modify: `.harness/templates/hardening_review_decision.schema.yaml`
- Modify: `.harness/templates/family_evidence_index.schema.yaml`
- Create: `.harness/learning/approved/.gitkeep`
- Create: `.harness/learning/reviews/.gitkeep`
- Create: `.harness/learning/feedback/.gitkeep`
- Create: `.harness/learning/hardening-reviews/.gitkeep`
- Create: `.harness/learning/feedback/feature-addition-confirmed.yaml`
- Create: `.harness/learning/reviews/feature-addition-promote.yaml`
- Create: `.harness/learning/reviews/feature-addition-high-cost-hold.yaml`
- Create: `.harness/learning/reviews/feature-addition-hold.yaml`
- Create: `.harness/learning/reviews/feature-addition-reject.yaml`
- Create: `.harness/learning/hardening-reviews/feature-addition-routing-hardening.yaml`

- [ ] **Step 1: Add a user-outcome feedback application command**

Update `bin/rail.dart` to add a manual feedback command, for example:

```bash
dart run bin/rail.dart apply-user-outcome-feedback --feedback <path>
```

The command should:
- validate the feedback file
- append reviewable user-outcome evidence refs to the matching candidate or queue entry
- refresh the family evidence index with confirmed user outcome refs
- preserve a reviewable trail instead of mutating artifacts silently

- [ ] **Step 2: Add a review-application command**

Update `bin/rail.dart` to add a manual review command, for example:

```bash
dart run bin/rail.dart apply-learning-review --decision <path>
```

The command should:
- validate the decision file
- validate the referenced candidate
- consider any appended `user_outcome_feedback` refs before making a final decision
- on `promote`, create or refresh `.harness/learning/approved/<task_family>.yaml`
- on `hold` or `reject`, update queue disposition without creating reusable memory

The review command must enforce an explicit guardrail-cost predicate:
- do not promote when the apparent improvement is explained only by excessive intervention cost
- emit `hold` or `reject` for high-cost-only wins

- [ ] **Step 3: Add a hardening-review application command**

Update `bin/rail.dart` to add a manual hardening review command, for example:

```bash
dart run bin/rail.dart apply-hardening-review --decision <path>
```

The command should:
- validate the hardening review decision
- update `.harness/learning/hardening_queue.yaml`
- preserve a reviewable disposition for policy-affecting observations
- avoid creating reusable family memory from hardening candidates
- [ ] **Step 4: Preserve provenance from candidate to approved memory**

Update the promotion logic so approved memory records:
- originating candidate refs
- `task_family_source`
- evidence basis for promotion
- guardrail note
- disposition history with reviewer identity, decision timestamp, disposition, and decision ref

- [ ] **Step 5: Implement provisional reconciliation and expiry**

Update the review application flow and queue handling so provisional candidates:
- remain distinguishable from confirmed candidates
- may be promoted only through explicit review
- expire by the next review window if no direct user outcome evidence arrives

Queue state and the family evidence index must both reflect whether a provisional candidate was promoted, was held, was rejected, or expired before becoming reusable memory.

- [ ] **Step 6: Validate user-outcome reconciliation and review decisions**

Create decision fixtures, then run:

```bash
dart run bin/rail.dart validate-artifact --file .harness/learning/feedback/feature-addition-confirmed.yaml --schema user_outcome_feedback
dart run bin/rail.dart apply-user-outcome-feedback --feedback .harness/learning/feedback/feature-addition-confirmed.yaml
dart run bin/rail.dart validate-artifact --file .harness/learning/reviews/feature-addition-promote.yaml --schema learning_review_decision
dart run bin/rail.dart validate-artifact --file .harness/learning/reviews/feature-addition-high-cost-hold.yaml --schema learning_review_decision
dart run bin/rail.dart validate-artifact --file .harness/learning/hardening-reviews/feature-addition-routing-hardening.yaml --schema hardening_review_decision
dart run bin/rail.dart apply-learning-review --decision .harness/learning/reviews/feature-addition-promote.yaml
dart run bin/rail.dart apply-learning-review --decision .harness/learning/reviews/feature-addition-high-cost-hold.yaml
dart run bin/rail.dart apply-learning-review --decision .harness/learning/reviews/feature-addition-hold.yaml
dart run bin/rail.dart apply-learning-review --decision .harness/learning/reviews/feature-addition-reject.yaml
dart run bin/rail.dart apply-hardening-review --decision .harness/learning/hardening-reviews/feature-addition-routing-hardening.yaml
```

Expected:
- feedback application appends reviewable evidence only
- provisional to confirmed happens only inside the explicit review flow when the review command considers appended user outcome evidence
- promotion emits approved family memory with full provenance
- high-cost-only improvement attempts are forced into `hold` or `reject`
- hold and reject persist a non-promotion disposition
- hardening candidates receive a separate reviewed disposition without entering reusable family memory
- provisional expiry is explicit and reviewable instead of silently becoming reusable memory

- [ ] **Step 7: Commit the manual-review slice**

```bash
git add bin/rail.dart .harness/learning/approved/.gitkeep .harness/learning/reviews/.gitkeep .harness/learning/feedback/.gitkeep .harness/learning/hardening-reviews/.gitkeep
git commit -m "feat: add manual quality learning review flow"
```

### Task 4: Inject Approved Memory as a Bounded Context Supplement

**Files:**
- Create: `.harness/learning/approved/feature_addition.yaml`
- Create: `.harness/fixtures/approved-memory/source-mismatch.yaml`
- Create: `.harness/fixtures/approved-memory/policy-version-mismatch.yaml`
- Create: `.harness/fixtures/approved-memory/schema-version-mismatch.yaml`
- Create: `.harness/fixtures/approved-memory/repository-condition-mismatch.yaml`
- Create: `.harness/fixtures/approved-memory/latest-evidence-conflict.yaml`
- Create: `.harness/requests/quality-learning-feature-family-baseline.yaml`
- Create: `.harness/requests/quality-learning-feature-family-followup.yaml`
- Create: `.harness/requests/quality-learning-feature-family-conflict.yaml`
- Modify: `bin/rail.dart`
- Modify: `.harness/templates/context_pack.schema.yaml`
- Modify: `.harness/templates/execution_report.schema.yaml`
- Modify: `.harness/templates/approved_family_memory.schema.yaml`
- Modify: `.harness/templates/family_evidence_index.schema.yaml`

- [ ] **Step 1: Add approved-memory lookup using the family key**

Update `bin/rail.dart` so the only reusable memory source is:

```text
.harness/learning/approved/<task_family>.yaml
```

Make the same-family precondition explicit: the request `task_type` and the approved memory filename/key must match before reuse is even considered.

Use explicit same-family request fixtures for verification, for example:
- `.harness/requests/quality-learning-feature-family-baseline.yaml`
- `.harness/requests/quality-learning-feature-family-followup.yaml`
- `.harness/requests/quality-learning-feature-family-conflict.yaml`

- [ ] **Step 2: Apply freshness and disposition rules using the evidence index**

Update `bin/rail.dart` so approved memory is considered only after a current-state refresh check and then classified as:
- `reuse`
- `quarantine`
- `drop`

Base freshness on:
- `task_family`
- `task_family_source`
- current request compatibility with the approved observation
- policy-contract version
- memory-schema version
- compatibility with current repository condition
- compatibility with the latest family success and failure evidence from `.harness/learning/family_evidence_index.yaml`

- [ ] **Step 3: Inject approved memory into the context pack as bounded guidance**

Update the runtime context assembly so reused approved memory appears only in a bounded supplement such as `approved_memory_hints` inside `context_pack.yaml`.

The injection must be:
- bounded in item count or size
- descriptive rather than imperative
- unable to override current request requirements or supervisor policy

- [ ] **Step 4: Record approved-memory provenance in `execution_report.yaml`**

Update the runtime so `execution_report.yaml` tells reviewers:
- which approved memory file was considered
- which `task_family_source` keyed that lookup
- whether it was reused, quarantined, or dropped
- why that disposition was chosen
- which originating candidate refs fed that approved memory

- [ ] **Step 5: Validate same-family reuse and bounded injection**

Create a same-family approved-memory fixture and a same-family request, then run:

```bash
dart run bin/rail.dart validate-artifact --file .harness/learning/approved/feature_addition.yaml --schema approved_family_memory
dart run bin/rail.dart run --request .harness/requests/quality-learning-feature-family-followup.yaml --project-root . --task-id 2026-04-10-approved-memory-reuse --force
```

Expected:
- runtime consumes only the approved memory file for the same `task_family`
- `context_pack.yaml` contains bounded guidance, not policy-like instructions
- `execution_report.yaml` shows the same-family lookup key and final disposition

- [ ] **Step 6: Validate stale-memory quarantine and request-conflict drop**

Create deterministic fixtures or setup steps for each freshness dimension before running:
- approved memory with `task_family_source` mismatch
- approved memory with policy-contract version mismatch
- approved memory with memory-schema version mismatch
- approved memory with repository-condition incompatibility
- approved memory with latest-family-evidence conflict
- approved memory with direct request conflict

Run:

```bash
dart run bin/rail.dart run --request .harness/requests/quality-learning-feature-family-followup.yaml --project-root . --task-id 2026-04-10-approved-memory-quarantine --force
dart run bin/rail.dart run --request .harness/requests/quality-learning-feature-family-conflict.yaml --project-root . --task-id 2026-04-10-approved-memory-request-conflict --force
```

Expected:
- stale or incompatible approved memory is quarantined or dropped for the specific forced freshness reason under test
- same-family approved memory is dropped when it conflicts with the live request or user intent
- neither case injects the dropped memory into the context pack

- [ ] **Step 7: Commit the approved-memory injection slice**

```bash
git add bin/rail.dart .harness/templates/context_pack.schema.yaml .harness/templates/execution_report.schema.yaml .harness/templates/approved_family_memory.schema.yaml .harness/templates/family_evidence_index.schema.yaml
git commit -m "feat: inject approved quality memory as bounded guidance"
```

### Task 5: Add Representative Acceptance Evidence for the Full Review-Only Model

**Files:**
- Create: `.harness/artifacts/2026-04-10-quality-learning-candidate/`
- Create: `.harness/artifacts/2026-04-10-hardening-candidate-surfaced/`
- Create: `.harness/artifacts/2026-04-10-quality-improvement-baseline/`
- Create: `.harness/artifacts/2026-04-10-quality-improvement-after-memory/`
- Create: `.harness/artifacts/2026-04-10-quality-improvement-comparison.yaml`
- Create: `.harness/artifacts/2026-04-10-approved-memory-reuse/`
- Create: `.harness/artifacts/2026-04-10-approved-memory-quarantine/`
- Create: `.harness/artifacts/2026-04-10-approved-memory-request-conflict/`
- Modify: `.harness/learning/review_queue.yaml`
- Modify: `.harness/learning/hardening_queue.yaml`
- Modify: `.harness/learning/family_evidence_index.yaml`

- [ ] **Step 1: Capture a representative quality-learning candidate artifact**

Produce or refresh `.harness/artifacts/2026-04-10-quality-learning-candidate/` so it shows:
- candidate emission
- review-queue entry creation
- no approved-memory reuse yet
- a populated `evaluator_support_signal`
- direct user outcome taking priority when confirmed user feedback exists

- [ ] **Step 2: Capture a hardening-candidate artifact**

Produce or refresh `.harness/artifacts/2026-04-10-hardening-candidate-surfaced/` so a policy-affecting observation appears under `hardening_candidates/` and in `.harness/learning/hardening_queue.yaml`, never in reusable family memory.

- [ ] **Step 3: Capture approved-memory reuse, quarantine, and conflict-drop artifacts**

Produce or refresh:
- `.harness/artifacts/2026-04-10-approved-memory-reuse/`
- `.harness/artifacts/2026-04-10-approved-memory-quarantine/`
- `.harness/artifacts/2026-04-10-approved-memory-request-conflict/`

These should show:
- same-family reuse
- one representative freshness-based quarantine path
- request-conflict drop
- bounded guidance injection in `context_pack.yaml`

- [ ] **Step 4: Capture an end-to-end paired-run quality-improvement proof**

Produce or refresh:
- `.harness/artifacts/2026-04-10-quality-improvement-baseline/`
- `.harness/artifacts/2026-04-10-quality-improvement-after-memory/`

The paired-run proof must:
- use the same explicit `task_family`
- run a baseline request without approved memory
- promote approved memory from the reviewed learning candidate
- rerun a same-family follow-up request with approved memory available
- produce a `quality_improvement_comparison` artifact that records the baseline, follow-up, user-outcome comparison, evaluator-support delta, and reviewed conclusion on whether the later run is more production-ready

- [ ] **Step 5: Validate the representative acceptance evidence set**

Run:

```bash
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-quality-learning-candidate/quality_learning_candidates/01.yaml --schema quality_learning_candidate
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-hardening-candidate-surfaced/hardening_candidates/01.yaml --schema hardening_candidate
dart run bin/rail.dart validate-artifact --file .harness/learning/approved/feature_addition.yaml --schema approved_family_memory
dart run bin/rail.dart validate-artifact --file .harness/learning/review_queue.yaml --schema learning_review_queue
dart run bin/rail.dart validate-artifact --file .harness/learning/hardening_queue.yaml --schema hardening_review_queue
dart run bin/rail.dart validate-artifact --file .harness/learning/family_evidence_index.yaml --schema family_evidence_index
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-quality-improvement-comparison.yaml --schema quality_improvement_comparison
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-approved-memory-reuse/execution_report.yaml --schema execution_report
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-approved-memory-quarantine/execution_report.yaml --schema execution_report
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-approved-memory-request-conflict/execution_report.yaml --schema execution_report
```

Expected:
- all representative acceptance artifacts validate
- confirmed-user-outcome evidence is visible as the primary signal when present
- hardening candidates are kept out of reusable family memory
- approved-memory reuse, one representative quarantine path, request-conflict drop, and bounded injection are all reviewable from artifacts alone
- the paired-run baseline versus follow-up comparison is auditable through a dedicated comparison artifact and explicit human-reviewed conclusion

- [ ] **Step 6: Commit the evidence slice**

```bash
git add .harness/artifacts/2026-04-10-quality-learning-candidate .harness/artifacts/2026-04-10-hardening-candidate-surfaced .harness/artifacts/2026-04-10-quality-improvement-baseline .harness/artifacts/2026-04-10-quality-improvement-after-memory .harness/artifacts/2026-04-10-quality-improvement-comparison.yaml .harness/artifacts/2026-04-10-approved-memory-reuse .harness/artifacts/2026-04-10-approved-memory-quarantine .harness/artifacts/2026-04-10-approved-memory-request-conflict .harness/learning/review_queue.yaml .harness/learning/hardening_queue.yaml .harness/learning/family_evidence_index.yaml
git commit -m "test: add representative review-only quality learning evidence"
```

### Task 6: Close the Docs Around the Full Sequential Model

**Files:**
- Modify: `docs/tasks.md`
- Modify: `docs/superpowers/plans/2026-04-07-production-readiness-checklist.md`

- [ ] **Step 1: Update the task list**

Update `docs/tasks.md` so the subproject is described as:
- review-only quality learning
- multi-candidate runtime emission
- separate hardening-candidate surfacing for policy-affecting patterns
- explicit human review decisions before promotion
- same-family approved-memory reuse only
- bounded context injection with provenance

- [ ] **Step 2: Update the readiness checklist**

Add the new evidence paths and record that long-term quality improvement is now defined as:
- reviewable candidate accumulation
- explicit review decisions
- approved-memory reuse with provenance
- separate hardening-candidate escalation for policy changes
- bounded same-family guidance injection instead of hidden adaptation

- [ ] **Step 3: Record implementation drift without rewriting the governing spec**

If implementation reveals a genuine design gap, add a follow-up note to `docs/tasks.md` or the readiness checklist. Do not silently narrow the governing spec from implementation work alone.

- [ ] **Step 4: Final documentation verification**

Run:

```bash
dart run bin/rail.dart validate-artifact --file .harness/artifacts/2026-04-10-quality-learning-candidate/quality_learning_candidates/01.yaml --schema quality_learning_candidate
dart run bin/rail.dart validate-artifact --file .harness/learning/approved/feature_addition.yaml --schema approved_family_memory
dart run bin/rail.dart validate-artifact --file .harness/learning/review_queue.yaml --schema learning_review_queue
dart run bin/rail.dart validate-artifact --file .harness/learning/hardening_queue.yaml --schema hardening_review_queue
dart run bin/rail.dart validate-artifact --file .harness/learning/family_evidence_index.yaml --schema family_evidence_index
```

Expected:
- docs and artifacts describe the same full review-only learning model
- no step relies on unreviewed memory being injected at runtime
- policy-affecting patterns are reviewable without becoming reusable family memory

- [ ] **Step 5: Commit the docs slice**

```bash
git add docs/tasks.md docs/superpowers/plans/2026-04-07-production-readiness-checklist.md
git commit -m "docs: close full review-only quality learning plan"
```
