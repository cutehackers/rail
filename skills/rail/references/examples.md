# Rail Examples

These examples show the current user-facing contract: ask Codex to use the Rail
skill, give the target repository path, and describe the work in natural
language. The skill infers the request shape and materializes it with
`rail compose-request --stdin`; users should not hand-write harness YAML. When
execution is requested, the skill starts the fresh task with `rail run` without
asking for a task id, captures the printed artifact path, and uses that path for
`supervise`, `status`, and `result`.

Use `/absolute/path/to/target-repo` as a placeholder for the target application
repository. The target repository is not the Rail source checkout.

## Task Identity Examples

Fresh task prompt:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the checkout total rounding bug.
```

Expected behavior:

- treat this as fresh work
- materialize a request
- run `rail run` without `--task-id`
- use the printed artifact path for execution and reporting

Existing artifact prompt:

```text
Use the Rail skill.
Continue /absolute/path/to/target-repo/.harness/artifacts/request-2 and report the result.
```

Expected behavior:

- treat this as existing artifact work
- do not run `compose-request`
- do not run `rail run`
- run `rail status`, `rail supervise`, `rail result`, or `rail integrate` against the supplied artifact path as requested

## Bug Fix Rubric

Prompt:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the intermittent profile refresh issue where the loading indicator sometimes remains visible after pull-to-refresh completes.
Context: The issue appears on the profile screen after a successful refresh.
Constraint: Do not change the public API contract.
Definition of done: The loading indicator clears after refresh, related regression coverage or equivalent focused validation is added, and analyze/build still passes.
```

Inferred task_type: `bug_fix`

Expected skill behavior:

- infer a low-risk bug-fix request
- keep `validation_profile` omitted so Rail uses the real `standard` path
- materialize the normalized request with `rail compose-request --stdin`
- run `rail run` without `--task-id` when execution is requested
- capture and report the artifact path printed by `rail run`
- use that artifact path for `supervise`, `status`, and `result`

## Feature Addition Rubric

Prompt:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Add a five-minute in-memory cache for profile lookup results.
Context: The cache belongs to the profile feature and should reuse existing service patterns.
Constraint: Do not introduce a new external dependency.
Constraint: Preserve the existing domain interface unless the request becomes unsafe without a small interface change.
Definition of done: Repeated profile lookups reuse fresh cached data, stale entries are refreshed after five minutes, focused tests cover the new behavior, and analyze/build still passes.
```

Inferred task_type: `feature_addition`

Expected skill behavior:

- infer a low-risk feature request
- keep constraints concrete instead of inventing extra policy
- leave file hints empty unless the user supplied reliable paths
- materialize the normalized request with `rail compose-request --stdin`
- run `rail run` without `--task-id` when execution is requested
- capture and report the artifact path printed by `rail run`
- use that artifact path for `supervise`, `status`, and `result`

## Safe Refactor Rubric

Prompt:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Split the club details screen build logic into smaller section-level units.
Context: This is a behavior-preserving cleanup of an existing screen.
Constraint: Preserve user-visible behavior exactly.
Constraint: Do not change state management patterns.
Definition of done: The screen behaves the same, the build logic is easier to scan by section, related tests or golden checks still pass, and analyze/build still passes.
```

Inferred task_type: `safe_refactor`

Expected skill behavior:

- infer a medium-risk safe-refactor request
- keep the definition of done centered on unchanged behavior
- avoid expanding the task into unrelated redesign
- materialize the normalized request with `rail compose-request --stdin`
- run `rail run` without `--task-id` when execution is requested
- capture and report the artifact path printed by `rail run`
- use that artifact path for `supervise`, `status`, and `result`

## Test Repair Rubric

Prompt:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Repair the flaky profile repository test that intermittently fails when cached data expires.
Context: The failure is in the profile test suite and should not require product behavior changes unless the test exposes a real bug.
Constraint: Keep production changes out of scope unless they are necessary to make the test truthful.
Definition of done: The flaky test is deterministic, the assertion remains specific to the profile cache behavior, no unrelated tests are rewritten, and the focused test target passes.
```

Inferred task_type: `test_repair`

Expected skill behavior:

- infer a low-risk test-repair request
- keep the change targeted to the reported test or coverage gap
- preserve product behavior unless the user explicitly accepts a bug fix
- materialize the normalized request with `rail compose-request --stdin`
- run `rail run` without `--task-id` when execution is requested
- capture and report the artifact path printed by `rail run`
- use that artifact path for `supervise`, `status`, and `result`

## Smoke Mode

Smoke mode is an execution profile, not a separate task family. Use it only when
the user asks to verify Rail wiring or the control-plane path itself.

Prompt:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Verify the Rail harness wiring only.
Constraint: Smoke mode.
Constraint: Do not modify application source files.
Definition of done: The smoke actor flow completes, application source files are unchanged, and smoke evidence is left in the artifact directory.
```

Inferred task_type: `test_repair`
Inferred execution mode: `smoke`

Expected skill behavior:

- set `validation_profile` to `smoke`
- keep the request scoped to harness verification
- materialize the normalized request with `rail compose-request --stdin`
- run `rail run` without `--task-id` when execution is requested
- capture and report the artifact path printed by `rail run`
- use that artifact path for `supervise`, `status`, and `result`
