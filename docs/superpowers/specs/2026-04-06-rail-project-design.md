# Rail Project Design

## Goal

Create `/absolute/path/to/rail` as the standalone control-plane repository for harness execution.

## Design summary

- `rail` owns the harness runtime and all harness metadata.
- Application repositories are treated as external target repos.
- The runtime stores requests and artifacts inside `rail`.
- The runtime operates against application repositories through `--project-root`.
- The user-facing skill is versioned in this repository instead of being hardcoded in `~/.codex/skills/rail`.

## Main decisions

### 1. Control-plane split

`rail` is not embedded in a product repository.
It is a separate repository that orchestrates work against product repositories.

### 2. Harness root vs target repo root

Two roots are explicit:

- harness root: this repository
- target project root: external application repository

This split is the critical change that makes separation durable.

### 3. First supported profile

The first profile is `Flutter + Riverpod`.
Full framework-agnostic generalization is deferred.

### 4. Migration strategy

Use the implementation in `981park-flutter-app` as the baseline, then generalize:

- remove hardcoded repo paths
- remove project-specific rules
- make actor execution use the target project root
- move the skill source of truth into this repository

## Completion criteria

- standalone runtime exists in `bin/rail.dart`
- `.harness` assets exist in this repository
- repo-local `skills/rail/SKILL.md` exists
- README documents `--project-root`
- 981park-specific hardcoded paths are removed from the new repo

