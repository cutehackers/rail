# Rail Harness Bundle

이 저장소의 `.harness/` 는 별도 앱 repo에 종속되지 않는 control-plane harness 번들이다.

핵심 역할은 다음과 같다.

- 최소 입력 request를 구조화하고 검증한다.
- task type별 workflow와 actor 계약을 고정한다.
- artifact, actor brief, execution plan을 생성한다.
- actor 실행 시 일관된 rubric/rule/schema를 공급한다.

## 기본 철학

- 사용자 입력은 적게 받는다.
- 작업 범위는 가능한 한 좁힌다.
- 실행과 평가는 분리한다.
- 하네스의 우선순위는 생성 속도보다 일관성과 안전성이다.

## 구조

- `supervisor/`: routing, registry, execution policy, contract
- `actors/`: actor별 지침
- `rules/`: 금지 변경, 아키텍처 규칙, 프로젝트 프로필 규칙
- `rubrics/`: task type별 평가 기준
- `templates/`: request/output schema와 template
- `skills/`: 반복 작업용 보조 skill 문서

## 사용 방식

`rail` 저장소는 harness root이고, 실제 앱 저장소는 target project다.

즉 두 경로가 항상 분리된다.

- harness root: 이 저장소
- target project root: `--project-root` 로 지정하는 외부 repo

## 실행 순서

1. request 생성
   - `dart run bin/rail.dart compose-request ...`
2. request 검증
   - `dart run bin/rail.dart validate-request --request ...`
3. workflow bootstrap
   - `dart run bin/rail.dart run --request ... --project-root /abs/path/to/app-repo`
4. actor 실행
   - `dart run bin/rail.dart execute --artifact .harness/artifacts/<task-id>`

`run` 시점에 target project root가 workflow에 기록되므로, 보통 `execute` 에서는 다시 넘기지 않아도 된다.

## 현재 자동화 수준

현재 runtime은 다음까지 수행한다.

- request compose/validate
- workflow artifact 생성
- actor brief 생성
- `codex exec` 기반 actor 순차 실행
- evaluator의 `revise` 시 generator 재시도

아직 다음은 완성되지 않았다.

- 병렬 actor orchestration
- 다양한 프로젝트 프로필
- 모든 task type에 대한 운영 수준 검증

## source of truth

- `registry.yaml`: task별 actor/output/rubric 정의
- `task_router.yaml`: task_type별 actor route와 risk retry 정책
- `context_contract.yaml`: actor별 입력/출력 계약

runtime은 이 세 파일을 기준으로 부트스트랩과 실행 정합성을 판단한다.
