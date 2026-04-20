# Active Release-Ready Tasks

This checklist tracks the remaining work to make `rail` release-ready at the
`v2 integrator and learning gate` boundary.

Scope of this checklist:

- includes repository-local runtime, CI, docs, packaging, and operator-process work
- includes installer and distribution alignment for the installed Rail product
- treats `v1 core supervisor gate` as already shipped and closed

Current baseline:

- `v1` release blockers: none
- `v1` verification gaps: none
- `v2` runtime surface is implemented locally
- `v2` now has CI coverage and a canonical operating model
- the installed-product direction is now fixed: packaged `rail`, bundled skill, project-local `.harness`
- remaining follow-up is release-surface polish, packaging consistency, and migration cleanup

## Must

- [x] Add a `v2` CI workflow that runs `./tool/v2_release_gate.sh`
  Done when pull requests and pushes to `main` fail on broken `v2` operational state in the same way `v1` already does.

- [x] Write the `Quality Improvement Operating Model`
  Define the retained quality signals, operator review flow, promotion rules, and the line between meaningful improvement and noise.
  Done when the model is documented as the canonical rule set for Workstream 4.

- [x] Turn the `v2` release evidence contract into an operator runbook
  Cover how to review `integration_result.release_readiness`, `blocking_issues`, `follow_up`, and which artifacts must be preserved for a release candidate.
  Done when operators can execute the `v2` gate and make a consistent release decision without relying on tribal knowledge.

- [x] Close the `v2` backlog against its own success criteria
  Update `docs/backlog/v2-integrator-and-learning.md` so completed workstreams are marked explicitly and the remaining open work is only the unfinished release-ready scope.
  Done when the backlog and release docs say the same thing about what is still open.

## Should

- [x] Decide the lifecycle of `--feedback` and `--decision` compatibility aliases
  Chosen direction: remove them before official release and standardize the apply commands on `--file` only.

- [x] Add a repeatable example for `v2` release evidence capture
  Keep one representative `integration_result` and its referenced artifacts as a stable example of a passing `v2` release candidate.
  Done when the evidence shape is easy to inspect and compare during future releases.

- [x] Tighten the `v2` release checklist around operator ownership
  Make the owner of release decisions explicit for `release_readiness`, blocking issues, and follow-up actions so the handoff contract is operational rather than descriptive only.

## Later

- [x] Retire transitional documentation and tooling that still assumes a checkout-era runtime
  Active operator and example documentation now follows the installed-product model: packaged `rail`, bundled skill, and project-local `.harness` state in the target repository.

- [x] Retire stale historical documentation that still references the old Dart runtime
  Remaining Dart references are now explicitly confined to archived evidence and historical planning records. Active product, release, and operator documentation now follows the Go product contract.
