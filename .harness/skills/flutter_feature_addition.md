# Skill: Flutter Feature Addition

## When to use
- Feature 추가/확장 within existing feature boundary
- Existing state-management and provider patterns should be preserved

## Workflow
1. Confirm feature boundary and affected layer(s)
2. Reuse existing provider/repository/test patterns
3. Add minimal implementation in existing implementation files
4. Add focused tests for new behavior and edge cases
5. Run analyze and related tests

## Guardrails
- Preserve architecture and wiring
- Avoid new dependencies
- Keep scope within declared files
- No opportunistic redesign

