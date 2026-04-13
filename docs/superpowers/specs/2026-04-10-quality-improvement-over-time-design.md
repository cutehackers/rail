# Quality Improvement Over Time Design

**Date:** 2026-04-10

**Goal**

Define what it means for the `rail` harness to improve quality over time, and specify a production-safe review process for carrying forward useful learning without allowing runtime policy drift.

## Problem Statement

The project goal is stricter than making the supervisor launch-ready. The harness must also show that repeated use leads to better final outputs, not just more visible routing or stricter refusals.

That claim is currently underspecified. Without an explicit definition, "quality improves over time" can collapse into weaker interpretations such as:

- fewer false passes without better outputs
- more retries and more validation activity
- higher internal evaluator confidence with no visible user benefit
- runtime policy changes that make the system appear adaptive while reducing auditability

The harness needs a narrower definition that is measurable, production-safe, aligned with user-visible outcome quality, and fully reviewable from artifacts.

## Definition

For this project, "quality improves over time" means:

- within the same `task family`
- accumulated `context` and `artifact` learning from prior runs
- helps later runs produce final outputs that are more production-ready than earlier runs
- with user outcome quality as the primary judgment and harness evaluation as a supporting signal

This definition explicitly does not mean:

- the runtime automatically changes `pass` criteria
- the runtime automatically changes evaluator routing policy
- retries alone count as improvement
- internal scores alone count as improvement

## Scope

This spec covers:

- the operational meaning of quality improvement over time
- the runtime evidence that should be captured after each run
- the `task family` memory structure used to carry forward useful learning
- the rules for injecting that memory into later runs
- the governance boundary between automatic memory updates and manual policy changes

This spec does not cover:

- automatic supervisor policy mutation
- automatic pass-bar adjustment
- fully autonomous evaluator retraining
- replacing human review of policy changes

## Design Principles

- quality is judged on final outputs, not intervention volume
- user outcome remains the primary quality signal
- learning should be grouped by `task family`, not across unrelated work
- runtime may accumulate review candidates, but may not rewrite policy
- memory should help current work without overriding current requirements
- useful learning must stay reviewable and bounded

## Core Architecture

The design uses three layers.

### 1. Runtime Evidence Layer

Every completed or explicitly terminated run records the quality-relevant evidence needed to understand whether the outcome was strong, weak, or inconclusive.

This layer captures:

- user outcome signals when available
- evaluator confidence and reasons
- which context additions helped or failed to help
- which validation evidence materially supported the final result

### 2. Task Family Review Queue

Evidence is grouped into review candidates by `task family`, such as:

- `bug_fix`
- `feature_addition`
- `safe_refactor`
- `test_repair`

The purpose of this layer is not to preserve full run logs. It exists to preserve compact, reviewable learning candidates about what may have improved or weakened outcomes within a specific kind of work.

`task family` assignment must be deterministic and reviewable. This design assumes an authoritative `task_family` field is carried from the supervisor request or planning contract into runtime artifacts. The runtime should group memory only by that explicit field and must not infer a different family ad hoc from vague heuristics after execution has begun.

### 3. Approved Family Memory Layer

Only human-reviewed candidates may become reusable memory.

This layer exists to hold approved observations for later same-family reuse. Approved memory is a compact quality supplement, not a runtime-generated rule change.

### 4. Governed Review Layer

Patterns that imply a policy problem are not applied automatically at runtime. They are surfaced for a separate hardening or review cycle.

This layer exists to keep the supervisor stable:

- runtime may accumulate candidate artifacts
- human-reviewed promotion may approve reusable family memory
- human-reviewed hardening cycles may change evaluator or supervisor policy

## Quality Signals

Each run should contribute evidence across five signals.

### 1. User Outcome Signal

This is the primary quality signal.

It answers whether the final result more completely satisfied the user's real goal. Representative indicators include:

- whether the requested outcome was accepted or reused without material rework
- whether follow-up correction pressure decreased
- whether the result resolved the user's intended problem rather than only satisfying an internal checklist

### 2. Evaluator Confidence Signal

This is a supporting quality signal.

It answers whether the harness had strong internal evidence for its final stance. Relevant indicators include:

- `quality_confidence`
- final `reason_codes`
- validation sufficiency
- whether the run ended in `pass`, explicit refusal, bounded exhaustion, or environment block

Evaluator confidence is useful only as support. It must not substitute for user-visible quality.

### 3. Effective Context Signal

This signal tracks which context additions actually improved output quality.

Examples include:

- a better requirements summary
- a recent failure-pattern reminder
- a focused repository-state recap
- a prior-artifact summary that prevented repeated mistakes

The harness should distinguish context that changed the outcome from context that merely increased prompt size.

### 4. Effective Validation Signal

This signal tracks which validation evidence actually correlated with better outcomes.

Examples include:

- a validation class that repeatedly prevented weak acceptance
- a proof type that increased confidence on a specific family
- a check that reduced user correction churn

The target is not more validation by default. The target is validation that meaningfully supports better final results.

### 5. Guardrail Cost Signal

This signal tracks whether an apparent improvement required disproportionate intervention cost.

Representative indicators include:

- retry count
- context rebuild count
- validation-tightening count
- generator revision count
- whether splitting or environment blocking occurred

Memory should not mark a pattern as broadly helpful if it improved outcomes only through excessive or noisy intervention. Quality learning must remain tied to cost versus value, not value in isolation.

## Reviewable Learning Candidate Model

Each run may emit one or more `quality learning candidates` for the current `task family`. A candidate is a review artifact, not reusable memory.

Each candidate should contain:

- the originating run artifact identity
- the deterministic `task_family` value and its source
- a quality outcome summary combining user outcome status and evaluator support
- the candidate claim: what appears to have improved or weakened quality
- the supporting evidence references for that claim
- the guardrail cost incurred while reaching that outcome
- a runtime recommendation such as `promote`, `hold`, or `reject`

The candidate should not contain:

- direct policy overrides
- implicit pass-bar changes
- unstated heuristics that would silently influence later runs

Candidates exist so a reviewer can decide whether a same-family observation is durable enough to reuse.

## Approved Family Memory Model

Each `task family` may maintain a compact approved memory. This memory is an operational summary of reviewed observations, not a raw log archive and not a runtime-generated rulebook.

The approved summary should contain:

- the deterministic family identifier used for grouping
- the source of that family classification
- the approved observation itself
- applicability conditions for reuse
- the evidence basis for approval
- a guardrail note describing acceptable versus excessive intervention cost
- a freshness marker tied to current policy and repository-state assumptions
- the disposition history that explains whether the observation was approved, quarantined, or later dropped

The approved summary should not contain:

- full actor transcripts
- raw per-run logs duplicated from artifacts
- direct policy overrides
- instructions that conflict with current task requirements

Conceptually, an approved family memory record should answer:

- what usually goes wrong in this family
- what has helped in this family
- what evidence matters most in this family
- what helped at acceptable versus excessive guardrail cost
- when and why the observation was approved
- whether the observation is still reusable under current conditions

Approved family memory must also be invalidatable. If the relevant policy contract changes, if repository assumptions materially change, or if the family classification source changes, stale observations should be dropped or quarantined rather than silently reused.

At minimum, freshness must be checked across these dimensions:

- `task_family` match
- active policy-contract version match
- memory-schema version match
- compatibility with the current repository condition relevant to the task
- compatibility with the latest failure and success evidence for that family

The disposition rule is:

- `reuse` when all freshness dimensions still match current conditions
- `quarantine` when family matches but current repository or evidence conditions partially conflict
- `drop` when the family key, policy contract, or memory schema no longer matches

Every disposition must be artifactized so a reviewer can tell what was considered, what was reused, and why.

## Approved Memory Reuse Rules

Only approved family memory may be re-injected into future runs, and only as a bounded context supplement.

The supplement must obey these rules:

- only inject approved memory for the same `task family`
- only inject approved memory after a fresh current-state check of repository state, recent artifacts, and active policy assumptions
- preserve artifactized provenance for what was considered and what was actually reused
- reject or quarantine stale observations whose freshness marker no longer matches current conditions
- drop observations that conflict with the current request
- treat approved memory as guidance, not as a forced instruction
- never let approved memory override current repository state or current user intent

The runtime should consume approved memory to improve grounding, not to create hidden defaults.

## Runtime Update Procedure

At the end of each run, the harness should perform a bounded candidate-generation step for the current `task family`.

The update sequence should be:

1. collect the final quality signals for the run
2. classify user outcome evidence as confirmed, provisional, or unavailable
3. identify which context, validation, and intervention-cost elements materially helped or hurt
4. summarize only the candidate observations worth review
5. emit reviewable learning-candidate artifacts for the current family
6. keep policy-changing observations separate as hardening candidates

This update must remain lightweight. The runtime should not attempt broad retrospective analysis every time it finishes a run.

When direct user outcome evidence is unavailable at run end, the runtime may store a provisional learning candidate using explicit fallback signals such as evaluator confidence and final artifact quality. That provisional candidate must stay distinguishable from confirmed learning.

Each provisional candidate must be linked to its originating run artifact identity. Reconciliation may happen only during an explicit review pass. If later user outcome evidence arrives, the candidate may be promoted, corrected, or discarded. If direct user outcome evidence still does not arrive by the next review window, the provisional candidate should expire rather than silently becoming durable memory.

## Review-Only Promotion Procedure

Candidate promotion must be review-driven.

The promotion sequence should be:

1. review the candidate artifact for evidence sufficiency and quality relevance
2. verify that the claimed improvement is not explained only by excessive guardrail cost
3. decide `promote`, `hold`, or `reject`
4. if promoted, emit an approved family memory artifact with provenance back to the originating candidate
5. if held or rejected, record the disposition so future reviewers can see why it was not reused

This review step is the boundary that keeps quality accumulation compatible with launch-hardening safety.

## Policy Change Governance

Some recurring patterns should not be handled as memory. They should be elevated into explicit review candidates.

Examples include:

- evaluator criteria that appear too weak or too strict
- routing rules that repeatedly choose the wrong intervention
- evidence requirements that are consistently insufficient
- family-level instability that persists even after contextual learning

These cases should be surfaced for a manual hardening cycle. They must not be auto-applied during runtime.

This boundary is the core safety property of the design:

- runtime can record what may have helped
- review can approve what is safe to reuse
- runtime cannot silently redefine what counts as quality

## Success Criteria

This design is successful when:

- quality improvement is defined in terms of better final outputs within the same `task family`
- user outcome remains the primary signal and evaluator confidence remains secondary
- repeated runs can reuse only bounded, reviewable, approved family memory
- effective context and validation patterns become easier to reuse
- helpful patterns remain attributable to acceptable guardrail cost rather than opaque intervention churn
- runtime policy remains stable unless explicitly changed through review

## Non-Goals

- claiming quality improvement based only on lower failure rates
- claiming quality improvement based only on internal confidence increases
- allowing runtime to rewrite supervisor contracts
- allowing runtime to inject unreviewed learning into later runs
- making cross-family generalizations without review

## Follow-On Planning Focus

The implementation plan derived from this spec should prioritize:

- defining the artifact format for `quality learning candidate` review records
- defining the artifact format for approved family memory and its provenance
- deciding where and when candidate artifacts enter a review queue
- determining how current runs consume only approved memory as bounded context supplements
- separating runtime candidate generation from review-time promotion and policy changes
- choosing the minimum initial signals needed to prove that later outputs are actually improving
