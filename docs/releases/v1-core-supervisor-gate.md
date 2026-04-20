# V1 Core Supervisor Gate

## Release Contract

`rail v1` is the production release for the bounded core supervisor gate.

Supported in `v1`:

- `rail init`
- `rail compose-request`
- `rail validate-request`
- `rail run --request ... --project-root ...`
- `rail execute --artifact ...`
- `rail route-evaluation`
- sequential execution of:
  - `planner`
  - `context_builder`
  - `generator`
  - `executor`
  - `evaluator`
- evaluator-driven bounded corrective loops
- deterministic supervisor actions
- reviewable terminal artifacts
- packaged distribution that includes the bundled Rail skill and embedded defaults

Deferred to `v2`:

- `integrator`
- `apply-user-outcome-feedback`
- `apply-learning-review`
- `apply-hardening-review`
- approved-memory, review-queue, and hardening-queue operations
- quality-improvement-over-time workflows

## Core Supervisor Gate

The supported `v1` runtime behavior is:

1. initialize a project-local `.harness/` workspace when needed
2. compose or validate a request
3. execute `planner -> context_builder -> generator -> executor -> evaluator`
4. let `evaluator` choose a deterministic next action
5. continue only within explicit retry budgets
6. stop in a visible terminal state

The bounded corrective loop is part of `v1`.

The self-learning loop is not part of `v1`.

## Operator Commands

For the installed product, the canonical operator commands are:

```bash
rail init
rail validate-request --request <request-file>
rail run --request <request-file> --project-root <target-repo>
rail execute --artifact <artifact-dir>
rail route-evaluation --artifact <artifact-or-evaluation-result>
```

The normal end-user entrypoint is still the Rail skill, not manual CLI request authoring.

Those commands operate on the target repository and its project-local `.harness/` workspace. They do not require the Rail source repository as the runtime root.

## Release Gate

Production release requires all of the following:

- the packaged `rail` binary exposes the `v1` command surface above
- the bundled Rail skill is included with the shipped product
- embedded defaults resolve correctly when a project does not override files locally
- project-local `.harness` state remains local and reviewable
- fresh smoke verification succeeds with `run -> execute`
- representative standard-route verification succeeds against the current request and artifact contracts
- terminal artifacts remain readable without raw actor log inspection
- no deferred `v2` field or flow is required for the `v1` path

Repository verification remains explicit and automated through:

- `./tool/v1_release_gate.sh`
- `.github/workflows/v1-release-gate.yml`
- `.github/workflows/go-release-gate.yml`

Additional release-ready confirmation should include:

- automated smoke verification for the fast control-plane path
- manual `./tool/real_mode_check.sh` verification for the real actor path

Those repository checks are release-engineering evidence for the product. They are not the end-user operating model.

## Operational Notes

- `v1` is intentionally conservative. Weak evidence should refuse or route to a corrective action rather than pass.
- `v1` only claims the core supervisor gate. It does not claim post-pass integration or review-driven learning behavior.
- Project customization in `v1` is file-level override only. If a project-local harness file exists, it replaces the embedded default for that file.
