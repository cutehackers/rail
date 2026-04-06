# Skill: Flutter Safe Refactor

## When to use
- Improve structure/readability without behavior changes
- Reduce widget build complexity or file coupling

## Workflow
1. Identify exact refactor target
2. Preserve behavior contracts and API boundaries
3. Extract sections into dedicated widget classes where appropriate
4. Keep logic and dependency graph unchanged
5. Validate via analyze + related tests

## Guardrails
- No behavior changes
- No architecture redesign
- No package additions
- No global UI shell changes unless requested

