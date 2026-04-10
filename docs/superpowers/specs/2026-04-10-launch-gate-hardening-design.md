# Launch Gate Hardening Design

**Date:** 2026-04-10

**Goal**

Make the `rail` supervisor-driven harness production-release credible by enforcing a conservative quality gate that prefers blocking weak outcomes over allowing ambiguous passes. The harness must improve result quality over time, refresh context on every improvement cycle, and keep guardrail behavior explicit, reviewable, and operationally useful.

## Problem Statement

The current harness has a documented core launch gate and routing evidence, but the project goal is stricter than simple launch readiness. The harness must:

- prevent low-confidence or weakly-evidenced passes
- make guardrail behavior observable and explainable
- improve output quality over repeated supervisor cycles
- refresh context against the current repository and artifact state before every meaningful retry
- stop only when the resulting quality bar is high enough for production or when the system can explicitly explain why it refused to proceed

This means the supervisor should be designed as a conservative quality gate, not a permissive orchestration layer.

## Scope

This spec covers the first hardening subproject:

- the core launch gate behavior for `planner -> context_builder -> generator -> executor -> evaluator`
- the evaluator decision standard for production-grade quality gating
- the relationship between supervisor actions and quality-improvement loops
- the context refresh discipline required between retry cycles
- the evidence model needed to judge whether guardrails are helping or merely adding cost

This spec does not attempt to fully automate long-term adaptive tuning. Early production hardening should use explicit review, explicit artifacts, and deliberate rule tightening.

## Design Principles

- `pass` is never the default outcome
- ambiguous evidence should route to a corrective supervisor action
- weak validation should be treated as a quality failure, not as success with caveats
- retries must either improve quality or terminate explicitly
- context must be refreshed from current state before another improvement cycle proceeds
- guardrails are only useful if they improve final quality with defensible cost

## Core Architecture

The core production gate remains:

`planner -> context_builder -> generator -> executor -> evaluator`

The critical hardening change is semantic, not structural. `evaluator` becomes the production quality gate for the harness. It should decide whether the current output is trustworthy enough to survive production scrutiny, not merely whether it looks superficially correct.

The runtime should stay simple:

- `planner` defines the intended task shape
- `context_builder` assembles the current repository and task context
- `generator` produces the implementation attempt
- `executor` supplies concrete evidence from execution and validation
- `evaluator` decides whether the outcome is production-credible or which corrective action must happen next

The harness should optimize for avoiding false positives. Lower completion rate is acceptable during hardening if it materially reduces weak passes.

## Quality Gate Model

The evaluator should judge every run across four mandatory dimensions:

### 1. Context Fidelity

Does the work reflect the current request, repository state, known constraints, and latest relevant artifacts?

If not, the system should prefer `rebuild_context`.

### 2. Implementation Adequacy

Does the patch materially satisfy the requested behavior, or does it only appear directionally close?

If not, the system should prefer `revise_generator`.

### 3. Validation Sufficiency

Does the evidence meaningfully cover the actual risk surface, or is the validation narrow, weak, or misaligned?

If not, the system should prefer `tighten_validation`.

### 4. Scope Integrity

Is the task still coherent as a single unit of work, or has it become too broad, mixed, or multi-objective?

If not, the system should prefer `split_task`.

Environment and tooling failures remain a separate class. If execution evidence shows the problem is environmental rather than implementation-related, the system should prefer `block_environment`.

`pass` should be allowed only when all of the following are true:

- the requested behavior and produced changes are clearly connected
- execution evidence and validation evidence do not contradict each other
- no unresolved failure mode remains that would materially weaken trust in the result
- remaining risk is low enough to be acceptable for production use under the current context

## Supervisor Action Semantics

The corrective actions should be interpreted as quality interventions:

- `rebuild_context`
  - chosen when the system is missing or misusing current grounding
- `revise_generator`
  - chosen when the implementation is inadequate even if the task is understood
- `tighten_validation`
  - chosen when validation cannot justify production confidence
- `split_task`
  - chosen when the unit of work is too broad to maintain quality
- `block_environment`
  - chosen when the environment prevents a trustworthy decision

These actions are not equivalent forms of retry. Each one should have a narrow semantic purpose so reviewers can understand why the system intervened.

## Conservative Pass Policy

The harness should be biased toward refusal over weak approval.

Operationally, that means:

- missing proof should count against passing
- contradictory evidence should count against passing
- sparse executor output should not be treated as implied success
- broad validation should not be accepted as sufficient without risk coverage
- repeated retries without better evidence should terminate explicitly rather than drift

This conservative policy is the intended production posture for the first release. Efficiency can be improved later, but not by lowering the trust bar prematurely.

## Guardrail Effectiveness Model

The harness must evaluate whether its own guardrails are helping. Each completed or terminated run should leave enough evidence to judge three things:

### Guardrail Cost

How much intervention occurred?

- number of retries
- number of context rebuilds
- number of validation tightenings
- number of generator revisions
- whether task splitting or environment blocking occurred

### Guardrail Value

Did the intervention materially improve quality?

- did it prevent an incorrect or weak pass
- did it increase clarity of the final decision
- did it reduce a previously visible risk

### Final Quality Confidence

How trustworthy is the final result after all interventions?

- high confidence pass
- explicit refusal with clear reason
- blocked due to environment
- bounded exhaustion with reviewable evidence

The system should not consider a guardrail successful merely because it intervened. A good guardrail is precise, not simply strict.

## Quality Improvement Loop

The harness should support repeated quality improvement through a closed review loop:

1. observe the reason codes, selected actions, and final outcome
2. determine whether the intervention improved the result or only increased cost
3. identify patterns of over-triggering or missed failures
4. feed those observations into the next hardening cycle

Early production hardening should keep this loop review-driven rather than self-modifying. The immediate objective is to make the system learnable and auditable before attempting automated policy adaptation.

## Context Refresh Discipline

Every meaningful improvement cycle must re-ground itself in current state.

Before another corrective pass proceeds, the harness should re-evaluate:

- current repository condition relevant to the task
- newly produced artifacts
- latest failure evidence
- latest success evidence
- whether the previous `reason_code -> action` link is still valid under the current state

The system should not rely on stale assumptions from prior cycles. A retry that does not refresh context risks repeating a precise but outdated judgment.

This discipline exists to ensure that each improvement step is based on the present state of the world, not on an earlier approximation.

## Operational Outcomes

A production-credible hardening pass should make the following true:

- the harness can clearly explain why it passed, blocked, split, retried, or exhausted
- quality-related retries are bounded and non-silent
- context-sensitive failures route to context refresh rather than blind implementation churn
- weak validation is surfaced as a production risk
- reviewers can tell whether guardrails are improving quality or just adding noise

## Non-Goals

- full automated optimization of supervisor policy
- introducing additional core stages beyond the current launch gate
- broadening the launch gate to include `integrator`
- relaxing the conservative pass standard to improve throughput

## Success Criteria

This hardening effort is successful when:

- the harness behaves as a conservative quality gate in both documentation and runtime behavior
- `pass` requires strong, current, and internally consistent evidence
- every corrective action has a clear quality rationale
- guardrail cost versus value becomes reviewable from artifacts
- every retry cycle explicitly refreshes context against current state
- the system improves quality through repeated review-and-tightening rather than through permissive acceptance

## Follow-On Planning Focus

The implementation plan derived from this spec should prioritize:

- making the conservative pass policy explicit in contracts and evaluator guidance
- capturing guardrail cost and value signals in artifacts
- enforcing current-state context refresh before repeated quality-improvement cycles
- identifying the minimum runtime and documentation changes needed to support these behaviors without making the supervisor harder to audit
