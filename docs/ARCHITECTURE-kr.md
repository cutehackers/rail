# Rail 아키텍처

## 개요

Rail은 하네스 control-plane입니다. 실제로 변경되는 다운스트림 제품 코드는 이 저장소 안에 있지 않습니다. 대신 Rail은 다른 저장소를 대상으로 동작하기 위한 runtime, supervisor policy, actor instruction, schema, reviewable artifact flow를 소유합니다.

시스템의 핵심 아이디어는 단순합니다. 사용자가 의도를 말하면, supervisor가 bounded actor workflow를 라우팅하고, Rail은 그 결과를 시간이 지나도 검토하고 발전시킬 수 있는 명시적 artifact로 남깁니다.

## 핵심 개념

### Supervisor

Supervisor는 control layer입니다. 어떤 actor를 다음에 실행할지, 언제 실행을 멈출지, evaluator feedback에 따라 bounded retry를 허용할지를 결정합니다.

Supervisor의 역할은 직접 코드를 생성하는 것이 아니라 workflow를 관리하고, policy를 적용하고, 결정을 명시적으로 남기는 것입니다.

### Actors

Actor는 workflow 안의 bounded specialist입니다. 현재 Rail 모델에서 actor는 대체로 다음 역할을 맡습니다.

- 작업 계획 수립
- 필요한 context 수집
- 후보 변경 생성
- validation 실행
- 결과 평가
- `v2` 흐름에서 integration handoff 생성

각 actor는 좁은 책임을 가지고, 나머지 시스템이 확인할 수 있는 구조화된 출력을 남깁니다.

### Rubric 과 Rule

Rubric과 rule은 Rail이 결과를 어떻게 판단하는지를 정의합니다. 기대치를 숨겨진 결정에 묻지 않고 설정 파일로 표현함으로써 workflow를 reviewable하게 유지합니다.

실제로는 다음과 같은 질문에 답합니다.

- 무엇을 pass로 볼 것인가
- 언제 retry가 허용되는가
- 언제 run을 blocked로 볼 것인가
- learning 및 hardening review를 어떻게 해석할 것인가

### Artifacts

Artifact는 실행의 영속 기록입니다. request, plan, context, implementation result, execution report, evaluation result를 담고, `v2`에서는 integration result와 learning review 파일까지 포함합니다.

이 구조 덕분에 Rail은 사용자 의도부터 supervisor outcome까지 추적 가능한 흐름을 유지합니다.

## Runtime 흐름

주요 흐름은 다음과 같습니다.

1. 사용자 요청이 구조화된 task로 변환됩니다.
2. Rail이 해당 task를 위한 artifact workspace를 만듭니다.
3. Supervisor가 actor를 순차적으로 dispatch합니다.
4. 각 actor는 구조화된 출력을 artifact set에 기록합니다.
5. Evaluator가 pass, policy 안의 retry, stop 중 하나를 결정합니다.
6. 실행이 pass이고 운영자가 확장된 `v2` 흐름을 원하면, Rail은 integration handoff와 learning review 입력을 생성할 수 있습니다.

중요한 점은 모든 전이가 보인다는 것입니다. 이 시스템은 의도적으로 explicit routing과 explicit output을 선호합니다.

## Supervisor, Actors, Rubrics 의 관계

관계는 다음과 같습니다.

- supervisor는 control을 소유함
- actor는 bounded execution을 소유함
- rubric과 rule은 결과 판단 기준을 정의함

Actor는 전체 workflow를 결정하지 않습니다. 자신의 단계를 수행하고 구조화된 출력을 반환합니다. Supervisor는 그 출력을 읽고, rule과 rubric을 적용하고, 다음 action을 결정합니다.

이 분리는 유지보수 측면에서 중요합니다.

- actor prompt는 전체 control flow를 다시 쓰지 않고도 진화할 수 있음
- routing policy는 모든 actor를 수정하지 않고도 바꿀 수 있음
- release 판단은 평가 기준이 명시적이기 때문에 review 가능하게 유지됨

## Artifacts 와 Learning State

Rail은 run artifact와 learning state를 분리합니다.

- run artifact는 특정 task 동안 무슨 일이 일어났는지를 설명합니다
- learning state는 리뷰된 결과 이후 시스템이 무엇을 기억해야 하는지를 설명합니다

`v2`에서는 운영자가 feedback이나 learning decision 같은 review 파일을 편집합니다. 그 다음 Rail이 그 리뷰 파일로부터 queue와 evidence 상태를 다시 생성합니다. 이렇게 하면 derived state가 재현 가능하고 감사하기 쉬워집니다.

Approved memory는 family 단위로 관리됩니다. 어떤 review가 promote되면 숨겨진 mutable state 체인을 만드는 대신, 해당 task family의 활성 approved memory가 갱신됩니다.

## V1 과 V2 의 경계

`v1`은 core supervisor gate입니다. bounded execution과 pass-or-revise control에 집중합니다.

`v2`는 그 위에 두 가지를 더합니다.

- pass 이후의 explicit integration handoff
- 지속적인 품질 개선을 위한 explicit learning/hardening review flow

이 경계는 의도적입니다. release gate는 집중된 상태로 유지하면서도, 장기적인 quality improvement는 별도의 reviewable layer에서 운영할 수 있게 합니다.

## 저장소 구조

상위 수준에서 저장소 구조는 다음과 같습니다.

- [bin/rail.dart](../bin/rail.dart)는 runtime entrypoint를 노출합니다
- `lib/src/runtime/`는 runtime execution과 supervisor logic을 담습니다
- `.harness/`는 actor instruction, rule, rubric, template, learning state를 담습니다
- `skills/Rail/`는 repo-owned Codex skill을 담습니다
- `docs/`는 release contract, architecture note, operator-facing reference를 담습니다

설계 목표는 새로움이 아닙니다. 설계 목표는 충분히 명시적이어서 review, evolution, trust가 가능한 control-plane을 만드는 것입니다.
