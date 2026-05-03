# Downstream Actor Canonical Seeds Design

## Goal

Extend `codex_vault` actor live smoke from `planner` and `context_builder` to
all supervisor actors by adding canonical seeded inputs for `critic`,
`generator`, `executor`, and `evaluator`.

The live smoke runner must keep actor failures local to the actor under test.
It must not recreate a full supervisor run, relax policy, mutate target files
directly, or pollute actor output schemas with smoke-only metadata.

## Problem

Downstream actors depend on upstream actor output. Running them without stable
seed input makes failures ambiguous:

- a `generator` failure may come from bad synthetic `context_pack` or
  `critic_report`
- an `executor` failure may come from pretending an unapplied patch exists
- an `evaluator` failure may come from an incorrectly bound
  `evaluator_input_digest`
- reports cannot currently identify which synthetic seed produced the failure

The earlier v1 design deferred these actors until canonical seed contracts were
available. This design defines those contracts.

## Critical Review Adjustments

The design incorporates the review findings from the pre-implementation
subagent check:

- fixture digests must be meaningful even when the smoke report root is under
  `.harness/live-smoke`
- behavior smoke must receive seed and invocation context, not only actor
  output
- reports must record seed schema version, seed digest, and synthetic-seed
  status
- `generator` smoke must validate patch bundles without applying them
- `evaluator` smoke must bind and check `evaluator_input_digest` the same way
  supervisor evaluation does
- `executor` seeds must avoid implying an unapplied patch was already applied
- live smoke must pass the allowed read-only shell executable set to actors and
  forbid tool availability probes; unavailable tooling should be reported as an
  actor output, not discovered by calling forbidden commands

## Seed Contract

Add a typed `LiveSmokeSeed` contract under `rail.live_smoke`.

Each seed records:

- `schema_version`: currently `"1"`
- `actor`: actor under test
- `fixture_digest`: digest of the copied fixture target
- `synthetic`: always `true`
- `upstream_output_digests`: digests of synthetic `prior_outputs`
- `validation_evidence_digest`: optional synthetic validation digest for
  `evaluator`
- `expected_patch_paths`: paths the `generator` smoke expects in a patch bundle
- `seed_digest`: digest of the seed payload excluding `seed_digest`

Synthetic upstream outputs remain in `prior_outputs`. Seed metadata is passed in
actor input as `live_smoke_seed`; it is never added to `PlanOutput`,
`ContextPackOutput`, `CriticReportOutput`, `ImplementationResultOutput`,
`ExecutionReportOutput`, or `EvaluationResultOutput`.

`live_smoke_runtime_contract` is also passed in actor input. It records
sandbox-relative path scope, the read-only shell executable allowlist, the
forbidden executable/probe set, and the rule that actors must report unavailable
tooling without probing commands such as `python -V`, `ruff --version`,
`pytest --version`, or `uv --version`.

## Actor Seed Strategy

`planner` receives no upstream outputs.

`context_builder` receives a schema-valid synthetic `planner` output.

`critic` receives schema-valid synthetic `planner` and `context_builder`
outputs.

`generator` receives schema-valid synthetic `planner`, `context_builder`, and
`critic` outputs. Its request asks for one small fixture change, so behavior
smoke can require a patch bundle and validate that the bundle stays inside the
fixture target.

`executor` receives a read-only synthetic upstream result. It does not receive a
seed that claims a generator patch has already been applied. This avoids
misleading validation failures in a report-only runner.

`evaluator` receives synthetic upstream outputs plus a synthetic validation
evidence digest. The runner computes `evaluator_input_digest` by digesting the
actor input after adding `validation_evidence_digest` and before inserting
`evaluator_input_digest`, matching the supervisor gate binding.

## Behavior Contracts

Behavior smoke remains shallow but actor-specific:

- `planner`: required planning fields exist
- `context_builder`: required context fields exist and core context lists are
  non-empty
- `critic`: critique and generator guardrail fields are present, with non-empty
  `priority_focus`, `validation_expectations`, and `generator_guardrails`
- `generator`: when the seed expects patch paths, output must include an inline
  or artifact-referenced patch bundle; the bundle must validate, use the copied
  fixture digest as `base_tree_digest`, include expected paths, and target must
  remain unchanged
- `executor`: report counts must be internally consistent; logs must be
  present; failing report states must include a machine-readable
  `class=...` failure detail
- `evaluator`: `evaluated_input_digest` must exactly echo the bound
  `evaluator_input_digest`; `revise` decisions require `next_action`; `pass` and
  `reject` omit it

These checks do not grade semantic quality beyond the minimum needed to prove
the actor executed the expected boundary.

## Fixture Digest Binding

`tree_digest()` must ignore `.git` and target-local `.harness` entries relative
to the supplied root, not by matching any absolute path segment. Otherwise a
report root such as `.harness/live-smoke` can make the entire copied fixture
look ignored and produce an empty-tree digest.

## Report Contract

`LiveSmokeReport` records seed provenance for every run that reaches invocation
construction:

- `seed_schema_version`
- `seed_digest`
- `synthetic_seed`

Fixture preparation failures may omit seed metadata because no trustworthy seed
was constructed.

## Interfaces

The shared runner remains the only execution surface. CLI and pytest call the
same `LiveSmokeRunner`.

`rail smoke actors --live` runs every supported actor. `rail smoke actor
<name> --live` accepts all supervisor actors once their seeds are implemented.

## Exclusions

This phase still excludes:

- automatic patch application
- automatic branch creation or commits
- direct target mutation by live smoke
- policy relaxation
- semantic grading beyond actor smoke contracts
- replacing supervisor end-to-end live smoke

## Success Criteria

- all six actors have canonical seeded live smoke input
- `run_all()` returns reports for all six actors
- seed metadata is present in successful setup reports
- generator patch bundles are validated but not applied
- evaluator digest echo uses supervisor-equivalent binding
- fixture digest remains meaningful under `.harness/live-smoke`
- optional live `codex_vault` smoke can run all supported actors without
  `OPENAI_API_KEY`
