# Codex Vault Actor Runtime Design

**Date:** 2026-04-30

## Goal

Make Rail usable through Codex login without depending on an operator OpenAI API
quota, while preserving actor isolation and Rail's Python-first governance
contract.

The default local user path should become the `codex_vault` Actor Runtime. The
OpenAI Agents SDK Actor Runtime remains available for operator-controlled API
key, CI, and live smoke environments.

## Decision

Rail remains Python-first. The old Go CLI and Go runtime are not revived.

Rail adopts these official product terms:

- **Actor Runtime:** the component that executes one Rail actor invocation.
- **`rail.specify`:** the public API that turns a skill-produced draft into a
  schema-valid Rail request.
- **`codex_vault`:** the default local-user Actor Runtime provider.
- **`openai_agents_sdk`:** an optional Actor Runtime provider for API-key
  environments.

`rail.specify` is the request-specification API. New docs,
skills, tests, and examples should use `rail.specify`.

## Problem

The current Python Actor Runtime can execute through the OpenAI Agents SDK, but
that path depends on `OPENAI_API_KEY` and the API organization or project having
active quota. For normal Codex users this creates a product mismatch:

1. The user is already authenticated to Codex.
2. The Rail skill is invoked from Codex.
3. Rail then requires a separate API billing path for actor execution.

The earlier raw Codex execution path had the opposite problem. It could use
Codex login, but actor execution inherited parent or user Codex surfaces such as
skills, plugins, MCP configuration, hooks, rules, or general user config. That
caused policy failures such as `illegal_skill_violation`. Those failures were
correct: an actor must not gain capabilities from the parent Codex session.

## Product Boundary

The parent Codex session owns only user interaction and Rail skill invocation.
The Rail skill composes a request draft and calls the Python API.

Rail owns:

- request specification through `rail.specify`
- artifact identity and resume policy
- effective policy loading and narrowing
- supervisor routing
- patch bundle validation and target apply
- validation evidence
- evaluator evidence-chain gates
- terminal summaries and result projection

The Actor Runtime owns:

- executing one actor invocation
- producing schema-valid actor output
- producing runtime evidence
- proposing patch bundles when the actor role allows mutation

The Actor Runtime is not the source of truth for terminal success. Rail accepts
success only through artifact evidence, validation evidence, and evaluator gates.

## Runtime Providers

### `codex_vault`

`codex_vault` is the default local user provider. It uses Codex execution while
preventing actor inheritance from the parent or user's normal Codex environment.

Required properties:

- actor execution uses an artifact-local `CODEX_HOME`
- actor execution does not use the parent Codex `CODEX_HOME`
- actor execution does not read user skills, plugins, MCP configuration, hooks,
  rules, or general user config
- actor execution receives only allowlisted auth material from a Rail-owned
  Codex auth home
- actor execution runs against an external worktree or sandbox
- target mutation still flows only through Rail-validated patch bundles
- runtime events are persisted as Rail evidence
- contamination or unknown materialization fails closed before evaluator routing

### `openai_agents_sdk`

`openai_agents_sdk` remains supported as an optional provider for environments
where an operator intentionally provides API credentials.

Appropriate uses:

- CI live smoke checks
- operator-controlled automation
- environments where API billing and quotas are explicit

It should not be the default local user path.

Target-local policy must not be able to select `openai_agents_sdk` by itself.
That provider is available only when Rail/operator defaults allow it and runtime
readiness confirms an approved SDK credential source. If either condition is
missing, Rail blocks before actor execution.

## Auth Model

Rail owns a user-scoped Codex auth home. It is distinct from:

- the user's normal Codex home
- the target repository
- the artifact directory

The preferred flow:

1. `rail auth login` runs Codex login with the Rail-owned Codex auth home.
2. `rail auth status` checks that same auth home.
3. `rail auth doctor` verifies both auth readiness and `codex_vault` runtime
   readiness.
4. Actor execution creates an artifact-local `CODEX_HOME`.
5. Rail copies only required auth material into the artifact-local
   `CODEX_HOME`.

The `rail auth` commands are setup and diagnostics surfaces only. They must be
Python-owned wrappers around auth readiness and must not become a task execution
CLI contract. Software work still flows through the Rail skill and Python API:
`rail.specify`, `rail.start_task`, `rail.supervise`, `rail.status`, and
`rail.result`.

Rail must not blindly copy the full Rail-owned auth home. The auth material
shape must be allowlisted. If the supported Codex version changes the auth
material shape in a way Rail does not understand, Rail should block with a
secret-safe readiness failure.

Rail must also verify the Codex command identity before actor execution. At
minimum, readiness must prove that the command is resolved from a trusted path,
reports a supported Codex version, and supports the isolation capabilities Rail
requires. If command identity, version, or isolation support is ambiguous, Rail
blocks before actor execution.

## Sealed Actor Environment

For every actor invocation, Rail prepares a sealed actor environment:

```text
Rail-owned Codex auth home
  auth material only

Artifact
  actor runtime directory
    CODEX_HOME
      allowlisted auth material
      Rail-generated minimal config when required
      no user skills
      no user plugins
      no MCP config
      no hooks
      no user rules
    event evidence
    structured actor output
```

The process environment must be scrubbed. `HOME`, `CODEX_HOME`, config paths,
plugin paths, MCP paths, and hook paths must point only at Rail-controlled actor
locations or be absent.

Where supported by Codex, Rail should use execution flags that ignore user
config, ignore user rules, and disable plugin or extension surfaces. These flags
are defense in depth. The primary boundary remains the sealed environment and
allowlisted materialization.

## Capability Rules

The default `codex_vault` Actor Runtime policy is:

- user skills: disabled
- user rules: disabled
- plugins: disabled
- MCP: disabled
- hooks: disabled
- host shell: disabled unless Rail explicitly implements and gates it
- direct target mutation: disabled
- patch bundle proposal: allowed only through actor output schema
- validation: owned by Rail, not actor-invented shell commands

The parent Rail skill is not available inside actor execution. If actor runtime
evidence shows parent or user skill invocation, Rail records a policy violation
and blocks the run.

## Evidence And Audit

Each actor invocation should persist:

- normalized runtime events
- raw Codex event stream when available
- sealed environment summary without secret values
- auth materialization status
- structured actor output
- output schema reference
- policy violation evidence when detected

Audit checks must run before evaluator routing. A schema-valid actor output is
not enough when the sealed runtime contract was violated.

Policy violations include:

- user or parent skill materialization
- user plugin materialization
- MCP config materialization
- hook materialization
- user rule materialization
- unexpected config inheritance
- direct target mutation outside Rail patch apply
- auth material outside the allowlist

Event audit is a secondary defense. Rail must not rely on event audit as the
only isolation boundary.

## API Rename

Add `rail.specify(draft)` as the official public API for request specification.

Rename rules:

1. `rail.specify(draft)` calls the existing request validation behavior.
2. `rail.start_task(draft)` uses `rail.specify`.
3. New skills, docs, and examples use `rail.specify`.
4. Request API tests should validate `rail.specify` end-to-end with fresh and rejected drafts.

The API rename is product-facing and must update:

- `src/rail/api.py`
- `src/rail/__init__.py`
- repo-owned Rail skill
- bundled Rail skill
- package asset copy of the Rail skill
- `docs/SPEC.md`
- `docs/ARCHITECTURE.md`
- request API tests

## Policy And Configuration

The canonical policy file remains `.harness/supervisor/actor_runtime.yaml`.

It should express:

- selected Actor Runtime provider
- model or Codex profile selection where applicable
- timeout limits
- max actor turns
- workspace isolation mode
- network mode
- tool capability policy
- approval policy
- evidence capture requirements

Rail/operator defaults remain the trust root. Target-local policy may narrow but
must not broaden capabilities or select a less isolated runtime posture.

The default provider should become `codex_vault`. `openai_agents_sdk` should be
selectable only when the operator environment explicitly allows it.

## Failure Modes

Rail must fail closed with secret-safe reasons for:

- Codex command unavailable or not trusted
- Codex command identity or version unsupported
- required Codex isolation capability unavailable
- Rail-owned Codex auth unavailable
- Rail-owned Codex auth expired or invalid
- unsafe auth home permissions
- unknown auth material shape
- actor-local `CODEX_HOME` materialization failure
- user skill, plugin, MCP, hook, rule, or config contamination
- unsupported Codex isolation flags
- target mutation before Rail patch apply
- missing or stale validation evidence

Blocked artifacts remain evidence. A new user goal should start a fresh artifact
instead of reusing a blocked artifact unless the user explicitly resumes it.

## Testing Strategy

Tests must prove isolation before live execution is considered release-ready.

Required deterministic tests:

- `rail.specify` returns the canonical request contract
- `rail.start_task` uses `rail.specify`
- default Actor Runtime provider loads as `codex_vault`
- `openai_agents_sdk` remains available when explicitly selected
- actor runtime policy rejects unknown provider values
- target-local policy cannot select `openai_agents_sdk` without Rail/operator
  authorization and SDK credential readiness
- Codex command readiness verifies trusted resolution, supported version, and
  required isolation capability support
- sealed environment creation uses artifact-local `CODEX_HOME`
- sealed environment does not include user skills, plugins, MCP config, hooks,
  rules, or general user config
- auth materialization copies only allowlisted paths
- unknown auth material shape blocks execution
- fake Codex parent-skill contamination triggers a policy violation
- fake Codex plugin or MCP materialization triggers a policy violation
- runtime events and audit evidence are persisted
- evaluator routing cannot accept success after runtime policy violation

Live checks should be narrow and opt-in:

- `rail auth login`
- `rail auth doctor`
- a non-mutating `codex_vault` smoke actor in a temporary target

## Documentation Updates

Update canonical docs and skills together:

- `docs/SPEC.md` becomes the source of truth for `rail.specify` and
  `codex_vault`
- `docs/ARCHITECTURE.md` describes Codex login based Actor Runtime execution as
  the default local user path
- `docs/CONVENTIONS.md` keeps Actor Runtime terminology
- Rail skill examples use `rail.specify`
- bundled skill assets stay aligned
- stale historical design docs remain historical records and do not define the
  current product contract

Docs and examples must not contain machine-specific home directory paths.

## Rollout Plan

1. Update canonical docs and public API naming around `rail.specify`.
2. Add typed Actor Runtime provider policy with `codex_vault` and
   `openai_agents_sdk`.
3. Add Rail-owned Codex auth home commands and readiness checks.
4. Add sealed actor environment materialization.
5. Add trusted Codex command/version/capability readiness.
6. Add Codex execution through the sealed environment.
7. Add event capture and contamination audit.
8. Wire supervisor readiness and blocked-result projection.
9. Update Rail skill copies and package assets.
10. Run focused request, policy, Actor Runtime, supervisor, and docs tests.
11. Run full release checks.

## Open Questions

- Which exact Codex auth files are required for the supported Codex CLI version?
- Which Codex isolation flags are stable enough to require in release checks?
- Should `rail auth doctor` include a minimal non-mutating Codex execution smoke
  by default, or only under an explicit live-check flag?


## Acceptance Criteria

- Normal local Codex users can run Rail without an OpenAI API key.
- Actors do not inherit parent or user Codex skills, plugins, MCP config, hooks,
  rules, general user config, or non-allowlisted auth material.
- `illegal_skill_violation` is preserved as a valid block when contamination is
  detected.
- Any contamination or runtime policy violation blocks terminal success before
  evaluator acceptance.
- Codex command identity, version, and required isolation capabilities are
  verified before actor execution.
- Target mutation still happens only through Rail-validated patch bundles.
- OpenAI Agents SDK execution remains available for explicit operator use.
- Canonical docs, skills, package assets, and tests use `rail.specify`.
- Go CLI/runtime surfaces are not revived.
