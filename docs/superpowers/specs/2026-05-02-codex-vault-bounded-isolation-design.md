# Codex Vault Bounded Isolation Design

**Date:** 2026-05-02

## Goal

Clarify the `codex_vault` isolation contract so Rail blocks inherited or
user-controlled capabilities without blocking normal Codex CLI bootstrap
behavior inside an actor-local `CODEX_HOME`.

## Decision

Rail keeps Actor Runtime isolation. The product contract is not sterile
filesystem isolation; it is bounded isolation:

- parent or user Codex capabilities must not flow into actor execution
- actor-local Codex CLI bootstrap material may exist when it is owned by the
  invoked Codex process and remains passive
- Rail-approved inspection and patch-bundle proposal remain the only actor
  capabilities
- target mutation remains impossible except through Rail-validated patch bundles
- runtime evidence must explain whether a block came from auth, environment,
  bootstrap provenance, capability use, sandboxing, schema, or validation

This design extends the earlier `codex_vault` Actor Runtime design. It does not
revive the Go CLI, add a task-execution CLI contract, or change the public
request API. Normal work remains Rail skill plus Python API:
`rail.specify`, `rail.start_task`, `rail.supervise`, `rail.status`, and
`rail.result`.

## Problem

The first `codex_vault` isolation model treated actor-local filesystem
materialization as the primary signal for contamination. That was too coarse.

Codex CLI may create the following under the actor-local `CODEX_HOME` during a
normal run:

- `skills/.system`
- `plugins/cache`
- `.tmp`
- `cache`
- `shell_snapshots`
- `config.toml`
- `installation_id`
- `models_cache.json`
- `tmp`
- `memories`

These entries are not automatically policy violations. They can be normal
Codex-owned bootstrap, discovery, cache, or local runtime state. Blocking every
such entry causes false positive policy failures and prevents Rail tasks from
progressing.

The real policy question is narrower:

1. Did a parent, user, target, or unknown capability enter the actor runtime?
2. Did the actor use a capability outside Rail policy?
3. Did any capability alter actor behavior, access external state, or mutate the
   target outside Rail patch validation?

## Product Contract

`codex_vault` must classify runtime material and events by provenance and
effect.

### Materialization

Materialization is the existence of files or directories in actor-local runtime
state.

Allowed materialization:

- allowlisted auth material copied from the Rail-owned Codex auth home
- Codex-owned bootstrap material created inside the actor-local `CODEX_HOME`
- Rail-owned evidence, schema, temp, and sandbox metadata

Blocked materialization:

- parent or user skill materialization
- target-local skill materialization
- custom plugin materialization outside the Codex-owned bootstrap profile
- MCP config materialization
- hook materialization
- user rule materialization
- inherited general user config
- symlink or hardlink escape material
- unknown material outside the actor artifact boundary

Materialization alone is not enough to block when it matches the Codex-owned
bootstrap profile.

### Provenance

Provenance identifies where a capability or material came from.

Allowed provenance:

- Rail-owned auth material
- Rail-owned actor prompts and schemas
- Codex-owned system bootstrap created inside the actor-local `CODEX_HOME`
- Rail-created sandbox and evidence directories

Blocked provenance:

- parent Codex session state
- user Codex home state
- target repository state treated as runtime configuration
- target-local or user-controlled skills, plugins, MCP servers, hooks, rules,
  or config
- unknown provenance that can affect actor behavior

Unknown passive cache material may be reported as diagnostic evidence, but
unknown executable, configurable, or capability-granting material must block.

### Capability Use

Capability use is an event or action that grants the actor behavior beyond its
Rail role.

Allowed capability use:

- schema-constrained actor output
- Rail-approved read-only repository inspection inside the sandbox
- patch bundle proposal through the actor output contract when the actor role
  allows it
- Codex-owned metadata discovery that does not grant tools, read external state,
  invoke skills, or mutate files

Blocked capability use:

- parent or user skill invocation
- target-local or user-controlled skill invocation
- plugin tool execution unless explicitly approved by Rail policy
- MCP server invocation
- hook execution
- user rule application
- inherited config application that changes model, tools, approvals, sandbox,
  or behavior
- direct target mutation
- validation command invention by an actor

Important distinction: passive bootstrap or discovery is not capability use.
Rail blocks only when provenance or effect crosses the Rail policy boundary.

## Audit Model

Rail should split `codex_vault` audit into three layers.

### 1. Bootstrap Profile Audit

This layer recognizes known Codex-owned actor-local material. It answers:

- Is this material inside the artifact boundary?
- Is it a file or directory shape Codex CLI is expected to create?
- Is it passive bootstrap/cache/config state rather than user-controlled
  behavior?
- Does it avoid symlink and hardlink escapes?

This profile should live as an explicit runtime contract, not as scattered
inline conditionals in `vault_audit.py`.

### 2. Provenance Audit

This layer answers:

- Is this material Rail-owned, Codex-owned, user-owned, target-owned, or
  unknown?
- Can this material affect actor behavior?
- Does an event refer to a parent/user path, target-local runtime config, or
  unknown external source?

User-owned, target-owned, parent-owned, and behavior-affecting unknown
provenance blocks the actor run.

### 3. Capability Use Audit

This layer answers:

- Did the actor actually invoke or receive a capability?
- Was that capability allowed by Rail policy?
- Did it read external state, change execution behavior, grant tools, or mutate
  files?

Discovery, registry loading, cache sync, and passive metadata events should not
block unless they grant or use a capability outside Rail policy.

## Evidence And Diagnostics

Audit results should become structured evidence rather than only strings.

The desired evidence shape is:

```json
{
  "category": "policy",
  "code": "user_skill_materialized",
  "reason": "user-controlled skill materialized in actor-local CODEX_HOME",
  "audit_layer": "provenance",
  "path_ref": "actor_runtime/actors/planner/.../codex_home/skills/rail",
  "actor": "planner"
}
```

Codes should distinguish at least:

- `unknown_auth_material`
- `unsafe_vault_material`
- `bootstrap_profile_mismatch`
- `user_skill_materialized`
- `user_plugin_materialized`
- `mcp_config_materialized`
- `hook_materialized`
- `user_rule_materialized`
- `inherited_config_applied`
- `plugin_capability_used`
- `mcp_capability_used`
- `skill_capability_used`
- `direct_target_mutation`

The terminal `reason` should be human-readable and secret-safe. It should not
mislabel bootstrap profile mismatches as auth allowlist failures.

## Policy Boundary

This design does not introduce user-facing isolation levels yet.

The default policy remains a single conservative `standard` behavior:

- passive Codex-owned bootstrap is allowed
- user, parent, target-local, or behavior-affecting unknown capability
  provenance is blocked
- Rail policy outside approved actor capabilities is blocked
- target mutation outside patch-bundle apply is blocked

`strict` and `diagnostic` isolation levels may be considered later, but they are
not required for this design. Adding them now would broaden policy surface
before the baseline contract is stable.

## Documentation Contract

`docs/SPEC.md` and `docs/ARCHITECTURE.md` should describe Rail's isolation as
bounded isolation:

- Rail does not promise an empty actor-local Codex home
- Rail does promise no inherited parent/user capability
- Rail allows passive Codex-owned bootstrap material
- Rail blocks behavior-affecting capability use outside policy
- Rail records structured evidence for policy blocks

The Rail skill should remain minimal. It reports blocked artifacts and evidence
refs; it must not patch runtime internals, auth homes, actor prompts, or target
files to continue a blocked run.

## Non-Goals

- Do not remove Actor Runtime isolation.
- Do not allow parent or user Codex skills inside actors.
- Do not allow MCP, hooks, rules, or plugin tools by default.
- Do not let target-local policy broaden capabilities.
- Do not create a task-execution CLI contract.
- Do not revive the Go CLI or Go runtime.
- Do not require users to understand request YAML or runtime flags.

## Acceptance Criteria

- `docs/SPEC.md` and `docs/ARCHITECTURE.md` describe bounded isolation rather
  than sterile filesystem isolation.
- `codex_vault` audit code has explicit bootstrap profile, provenance, and
  capability-use boundaries.
- Passive Codex-owned bootstrap material in actor-local `CODEX_HOME` does not
  block actor execution.
- User, parent, target-local, or behavior-affecting unknown skills, plugins,
  MCP, hooks, rules, or config still block actor execution.
- Discovery or metadata events do not block unless they grant or use a
  capability outside Rail policy.
- Policy block evidence identifies the audit layer and violation code.
- Existing public workflow remains Rail skill plus Python API.

## Risks

- Codex CLI bootstrap behavior may change. The bootstrap profile should be easy
  to update and covered by focused tests.
- Event streams may not always expose enough provenance. In ambiguous
  behavior-affecting cases, Rail should fail closed; in passive cache cases,
  Rail may record diagnostics without blocking.
- Overly broad event keyword matching can block normal tasks. Capability audit
  must avoid treating all mentions of "skill", "plugin", or "config" as use.
- Overly permissive bootstrap matching can hide real contamination. The profile
  must remain actor-local, non-symlinked, and non-capability-granting.

