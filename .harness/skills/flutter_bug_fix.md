# Skill: Flutter Bug Fix

## When to use
- Reproducible bug in data/controller/widget flow
- Existing architecture should stay intact
- Minimal diff preferred

## Workflow
1. Confirm bug scope from request (`goal`, `context`, constraints)
2. Identify smallest failing path (controller/repository/page)
3. Collect nearby code patterns and existing tests
4. Apply minimal patch limited to target files
5. Add or adjust focused regression test if available
6. Run analyze + related tests

## Guardrails
- No public API changes unless explicitly requested
- No speculative refactors
- No unrelated files
- Keep behavior change localized

