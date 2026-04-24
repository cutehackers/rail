# Sealed Actor Runtime and Workflow Continuity Design

**Date:** 2026-04-24

**Goal**

Rail harness가 `codex exec`로 actor를 실행하더라도 사용자 개인 Codex 설정에 휘둘리지 않고, 작업 상태를 artifact로 계속 이어가며, 최종 사용자에게 신뢰할 수 있는 답변을 만들게 한다.

## 쉬운 설명

Rail은 직접 코드를 고치는 사람이 아니다. Rail은 감독관이다.

Codex actor가 실제로 파일을 읽고 고치지만, Rail은 다음을 책임져야 한다.

- 어떤 일을 해야 하는지 정한다.
- 어느 범위 안에서 해야 하는지 정한다.
- 지금 어디까지 했는지 기록한다.
- 결과가 목표에 맞는지 검사한다.
- 부족하면 다음 작업을 다시 지시한다.
- 마지막에 사용자에게 검증된 결과만 보고한다.

현재 문제는 Rail actor가 `codex exec`로 실행될 때 일반 사용자 Codex 세션처럼 시작된다는 점이다. 그러면 사용자 전역 skill, rules, plugins, hooks 같은 것들이 actor 실행에 섞일 수 있다. 이번에 관찰된 superpowers skill 발동은 이 구조 문제의 증상이다.

Rail이 원하는 동작은 다르다.

```text
사용자 Codex 세션
  Rail skill을 사용해 자연어 요청을 request로 만든다.
  사용자와 대화한다.

Rail supervisor
  request, workflow, artifact, policy, evaluator를 관리한다.

Rail actor Codex 세션
  Rail이 준 actor brief만 읽는다.
  Rail이 허용한 기능만 쓴다.
  정해진 schema로만 답한다.
```

## Problem Statement

현재 구현은 이미 좋은 방향을 많이 갖고 있다.

- actor graph는 `planner -> context_builder -> critic -> generator -> executor -> evaluator`로 나뉘어 있다.
- actor model과 reasoning은 `.harness/supervisor/actor_profiles.yaml`에서 온다.
- actor backend 설정은 `.harness/supervisor/actor_backend.yaml`로 분리되어 있다.
- actor output은 schema-valid artifact로 저장된다.
- smoke profile은 Codex 호출 없이 deterministic path를 제공한다.
- Codex JSON events를 `runs/` 아래에 보존할 수 있다.

하지만 중요한 경계가 아직 약하다.

- actor용 `codex exec`가 사용자 Codex 설정을 기본적으로 읽을 수 있다.
- actor 실행 환경이 사용자의 shell environment를 거의 그대로 상속한다.
- backend policy가 sandbox와 approval은 다루지만 skills, rules, plugins, hooks, MCP 같은 capability를 명시하지 않는다.
- event log를 저장하더라도 예상 밖 skill 주입이나 plugin 로딩을 policy violation으로 판단하지 않는다.
- actor가 새 세션으로 실행될 때 다음 작업을 이어가기 위한 요약 artifact가 충분히 명확하지 않다.
- 최종 답변이 어떤 evidence를 근거로 해야 하는지 별도 contract가 없다.

이 상태에서는 workflow가 actor 사이를 지나가더라도, 실제 작업 품질은 actor의 순간 판단과 사용자 Codex 환경에 의존할 수 있다.

## Design Principles

- Rail은 governance layer다. Codex agent runtime을 다시 만들지 않는다.
- 사용자-facing Codex 세션과 Rail actor Codex 세션은 다르게 취급한다.
- actor 실행은 기본적으로 sealed mode여야 한다.
- 작업 지속성은 agent memory가 아니라 artifact로 보장한다.
- actor가 새로 시작해도 `next_action`과 evidence를 읽으면 이어서 일할 수 있어야 한다.
- evaluator는 계속 최종 gate다.
- 최종 답변은 느낌이 아니라 evidence에 근거해야 한다.
- 실패도 artifact로 남겨야 한다. 그래야 다음 actor나 사용자가 같은 문제를 다시 추적할 수 있다.

## Target Architecture

목표 구조는 다음과 같다.

```text
User-facing Codex layer
  Rail skill
  natural-language request drafting
  user clarification

Rail governance layer
  request normalization
  workflow state
  actor routing
  artifact schemas
  evaluator decisions
  validation evidence
  final reporting contract

Sealed actor runtime layer
  constrained codex exec
  explicit actor capabilities
  clean execution environment
  output schema enforcement
  event audit

Target repository layer
  source files
  project-local .harness
  run artifacts
  reviewed learning state
```

핵심은 `Sealed actor runtime layer`다. Rail은 Codex를 계속 쓰지만, actor에게는 Rail이 허용한 기능만 준다.

## Sealed Actor Runtime

Rail actor용 `codex exec`는 일반 Codex 실행과 달라야 한다.

기본 actor 실행은 다음 정책을 따라야 한다.

- user config를 읽지 않는다.
- user rules를 읽지 않는다.
- user skills를 자동으로 쓰지 않는다.
- plugins와 MCP는 기본 disabled다.
- hooks는 기본 disabled다.
- sandbox, approval, model, reasoning은 Rail policy에서만 온다.
- actor prompt와 output schema는 Rail이 만든다.
- actor output은 Rail이 검증한 뒤 artifact로 저장한다.

초기 구현에서는 Codex CLI가 이미 제공하는 플래그부터 사용한다.

```text
codex exec
  --ignore-user-config
  --ignore-rules
  --ephemeral
  --output-schema <schema>
  --output-last-message <path>
  --json
```

이 플래그만으로 모든 capability가 완전히 차단된다고 가정하면 안 된다. 그래서 별도 event audit이 필요하다.

## Capability Policy

`.harness/supervisor/actor_backend.yaml`은 실행 인자뿐 아니라 actor capability도 표현해야 한다.

대표 구조는 다음과 같다.

```yaml
version: 1
execution_environment: local
default_backend: codex_cli

backends:
  codex_cli:
    command: codex
    subcommand: exec
    sandbox: workspace-write
    approval_policy: never
    session_mode: per_actor
    ephemeral: true
    capture_json_events: true
    skip_git_repo_check: true
    ignore_user_config: true
    ignore_rules: true
    capabilities:
      user_skills: disabled
      user_rules: disabled
      plugins: disabled
      mcp: disabled
      hooks: disabled
      shell: allowed
      file_editing: allowed

execution_environments:
  local:
    allowed_sandboxes:
      - workspace-write
```

이 정책의 의미는 단순하다.

Rail actor는 사용자의 평소 Codex 도구상자가 아니라 Rail이 허용한 작업 도구만 써야 한다.

## Clean Environment

`exec.CommandContext`는 기본적으로 부모 process 환경변수를 상속한다. actor runtime에서는 이 기본값이 위험하다.

Rail은 actor subprocess 환경을 명시적으로 만들어야 한다.

허용할 수 있는 값:

- `PATH`
- 필요한 인증 관련 환경
- target repository 작업에 필요한 최소 환경
- Rail이 직접 설정하는 runtime metadata

차단하거나 별도 처리해야 하는 값:

- `CODEX_HOME`
- user plugin cache 관련 값
- MCP server 설정 관련 값
- hook 관련 값
- actor model이나 reasoning을 바꿀 수 있는 값
- project root 밖의 home-directory path를 actor prompt에 노출하는 값

장기적으로는 actor 전용 임시 Codex home을 만드는 방식이 더 낫다.

```text
.harness/artifacts/<task-id>/runtime/codex_home/
```

이 디렉터리에는 Rail이 만든 최소 config만 둔다. 사용자 skill directory를 복사하지 않는다.

## Event Audit

Codex JSON event stream은 단순 로그가 아니라 governance evidence다.

Rail은 `runs/*-events.jsonl`을 검사해야 한다.

다음 신호가 나오면 actor run은 실패하거나 최소한 policy violation으로 표시되어야 한다.

- 예상 밖 skill injection
- user home 아래 skill 파일 접근
- disabled plugin 로딩
- disabled MCP 사용
- disabled hook 실행
- actor prompt에 없는 외부 instruction 주입
- output schema를 우회하려는 행동

대표 violation code:

```text
backend_policy_violation
unexpected_skill_injection
unexpected_plugin_load
unexpected_mcp_use
unexpected_hook_execution
dirty_actor_environment
```

이 검사는 evaluator 이전에 일어나야 한다. actor output이 schema-valid여도 backend policy를 어겼다면 좋은 결과로 볼 수 없다.

## Workflow Continuity

Rail actor는 per-actor `codex exec` 세션으로 실행된다. 따라서 actor가 자기 기억으로 다음 단계를 이어갈 수 있다고 가정하면 안 된다.

작업 지속성은 artifact로 만들어야 한다.

추가할 핵심 artifact:

```text
.harness/artifacts/<task-id>/work_ledger.md
.harness/artifacts/<task-id>/next_action.yaml
.harness/artifacts/<task-id>/evidence.yaml
.harness/artifacts/<task-id>/final_answer_contract.yaml
```

### `work_ledger.md`

사람이 읽기 쉬운 작업 장부다.

기록할 내용:

- 이번 run의 목표
- actor별 핵심 결정
- 왜 그 결정을 했는지
- 어떤 파일을 봤는지
- 어떤 위험이 남았는지
- 다음 actor가 놓치면 안 되는 점

### `next_action.yaml`

다음 actor가 반드시 따라야 할 machine-readable 지시다.

예시:

```yaml
actor: generator
reason: evaluator_requested_revision
must_do:
  - Keep the patch limited to request normalization.
  - Add focused regression coverage for unknown draft fields.
must_not_do:
  - Do not change the request schema version.
evidence_to_read:
  - plan.yaml
  - context_pack.yaml
  - critic_report.yaml
  - execution_report.yaml
blocking_questions: []
```

### `evidence.yaml`

검증 근거를 한곳에 모은다.

예시:

```yaml
changed_files:
  - internal/request/normalize.go
validation:
  analyze:
    command: go test ./internal/request
    status: pass
  tests:
    command: go test ./...
    status: pass
known_limits:
  - No live Codex actor run was executed in smoke mode.
```

### `final_answer_contract.yaml`

사용자에게 마지막으로 답할 때 빠뜨리면 안 되는 항목을 정의한다.

예시:

```yaml
required_sections:
  - outcome
  - changed_scope
  - validation_evidence
  - residual_risks
  - next_step_if_blocked
forbidden_claims:
  - claim_tests_passed_without_evidence
  - claim_feature_complete_when_only_bootstrapped
  - hide_policy_violation
```

## Actor Prompt Changes

각 actor brief는 기존 contract input 외에 continuity input을 받아야 한다.

예:

```text
## Continuity Inputs
- work_ledger: .harness/artifacts/<task-id>/work_ledger.md
- next_action: .harness/artifacts/<task-id>/next_action.yaml
- evidence: .harness/artifacts/<task-id>/evidence.yaml
```

actor instruction도 더 분명해야 한다.

- 먼저 continuity input을 읽는다.
- 현재 actor가 해야 할 일만 한다.
- 이전 결정을 다시 뒤집으려면 이유를 output에 남긴다.
- schema-valid output 외에 artifact 파일을 직접 쓰지 않는다.
- final answer를 직접 작성하지 않는다. final reporting은 Rail이 한다.

## Supervisor Changes

Supervisor는 actor output만 보고 다음 actor를 정하면 부족하다. 다음도 함께 갱신해야 한다.

- `state.json`
- `work_ledger.md`
- `next_action.yaml`
- `evidence.yaml`
- `supervisor_trace.md`

각 transition에는 이유가 있어야 한다.

예:

```text
generator -> executor
reason: implementation_result schema-valid and changed files stay within context_pack

evaluator -> generator
reason: tests failed and retry budget remains
next_action: revise generator using executor failure details
```

## Final Reporting

최종 답변은 actor의 자유 응답에 맡기면 안 된다.

Rail은 terminal summary와 final answer contract를 바탕으로 사용자 답변에 들어갈 내용을 정해야 한다.

최소 포함 항목:

- 무엇을 했는지
- 어떤 파일 범위가 바뀌었는지
- 어떤 검증을 실행했는지
- 검증 결과가 무엇인지
- 남은 위험이나 제한은 무엇인지
- 실패했다면 정확히 어디서 막혔는지

금지해야 할 답변:

- 검증하지 않았는데 통과했다고 말하기
- bootstrap만 했는데 구현 완료라고 말하기
- policy violation을 숨기기
- actor가 실패했는데 사용자에게 성공처럼 말하기

## Implementation Phases

### Phase 1: Immediate actor isolation

- `ActorBackendConfig`에 `IgnoreUserConfig`와 `IgnoreRules`를 추가한다.
- default backend policy에서 둘 다 true로 둔다.
- `buildCodexCLIArgs`가 `--ignore-user-config`, `--ignore-rules`를 붙인다.
- fake Codex test로 정확한 args를 검증한다.
- superpower skill injection 재현 테스트를 event fixture로 고정한다.

### Phase 2: Explicit capability policy

- `ActorBackendConfig`에 capability section을 추가한다.
- unsupported capability 값은 policy load 단계에서 실패시킨다.
- 기본값은 user skills, rules, plugins, MCP, hooks disabled다.
- Architecture docs에서 user-facing Codex와 actor Codex의 차이를 설명한다.

### Phase 3: Clean actor environment

- `runCommand`에서 `cmd.Env`를 명시적으로 구성한다.
- unsafe inherited environment를 제거한다.
- 필요한 auth와 PATH만 허용한다.
- actor 전용 temporary Codex home을 검토한다.

### Phase 4: Event audit

- `runs/*-events.jsonl`을 검사하는 audit module을 추가한다.
- unexpected skill/plugin/MCP/hook 신호를 violation으로 만든다.
- violation은 evaluator pass보다 우선한다.
- terminal summary에 violation reason을 포함한다.

### Phase 5: Continuity artifacts

- `work_ledger.md`, `next_action.yaml`, `evidence.yaml`을 bootstrap에서 생성한다.
- actor transition마다 갱신한다.
- actor brief에 continuity input을 추가한다.
- resume된 run이 같은 artifact를 읽고 이어가도록 테스트한다.

### Phase 6: Final answer contract

- `final_answer_contract.yaml`을 artifact로 만든다.
- terminal summary 생성 시 contract를 반영한다.
- 검증 없는 성공 주장, policy violation 은폐, bootstrap과 implementation 혼동을 막는 테스트를 추가한다.

## Testing Strategy

Unit tests:

- backend args에 `--ignore-user-config`와 `--ignore-rules`가 포함되는지 검증한다.
- capability policy default가 safe한지 검증한다.
- unsafe capability override가 실패하는지 검증한다.
- clean environment builder가 unsafe env를 제거하는지 검증한다.
- event audit이 unexpected skill injection fixture를 잡는지 검증한다.
- `next_action.yaml`이 supervisor transition마다 갱신되는지 검증한다.

Runtime smoke tests:

- smoke profile은 live Codex 없이 계속 deterministic하게 통과해야 한다.
- standard profile은 fake Codex로 sealed args와 event capture를 검증한다.
- policy violation이 있으면 schema-valid actor output이어도 pass하지 않아야 한다.

Documentation checks:

- docs와 examples에 home-directory path를 쓰지 않는다.
- Architecture docs는 user-facing Codex와 sealed actor Codex를 구분한다.
- Rail skill docs는 request composition 책임과 actor execution 책임을 섞지 않는다.

## Acceptance Criteria

- Rail actor 실행에서 사용자 전역 superpowers skill이 자동 발동하지 않는다.
- actor backend policy가 user skills, user rules, plugins, MCP, hooks의 기본 상태를 명시한다.
- actor subprocess 환경은 Rail이 허용한 값만 상속한다.
- event audit이 예상 밖 skill/plugin/MCP/hook 사용을 감지한다.
- workflow가 actor 세션 기억에 의존하지 않고 artifact로 이어진다.
- evaluator revise 후 다음 actor가 `next_action.yaml`만 읽어도 이어서 작업할 수 있다.
- final summary는 validation evidence 없이 성공을 주장하지 않는다.
- 기존 smoke profile은 deterministic path를 유지한다.

## Non-Goals

- Codex agent runtime을 Rail이 다시 구현하지 않는다.
- 모든 Codex plugin 또는 MCP 기능을 영구적으로 금지하지 않는다. 기본값은 disabled이고, 나중에 allowlist로 열 수 있다.
- hosted Rail service를 만들지 않는다.
- request schema 전체를 갈아엎지 않는다.
- downstream application code 변경 정책을 이 spec에서 다루지 않는다.

## Open Questions

- actor 전용 Codex home을 항상 만들 것인가, 아니면 `--ignore-user-config` 기반으로 먼저 갈 것인가?
- auth 관련 환경은 어디까지 허용할 것인가?
- plugin/MCP allowlist는 project-local `.harness`가 아니라 더 신뢰도 높은 policy source가 필요한가?
- final answer contract를 YAML artifact로 둘 것인가, terminal summary schema에 통합할 것인가?
- event audit violation은 즉시 reject로 볼 것인가, `block_environment`로 볼 것인가?
