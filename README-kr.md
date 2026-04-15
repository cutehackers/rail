# rail

`rail`은 Codex를 위한 하네스 control-plane입니다. 자연어로 작성한 엔지니어링 요청을, 별도의 대상 저장소에 대해 실행 가능한 구조화된 워크플로우로 바꾸는 역할을 합니다.

핵심 결과는 단순합니다. 프롬프트 조합, 작업 경계, 리뷰 핸드오프를 사람이 직접 관리하는 대신, Rail skill을 사용해 supervisor가 관리하는 bounded workflow와 명확한 산출물을 얻을 수 있습니다.

## 설치

요구 사항:

- Dart SDK `^3.9.0`
- Codex

설치:

```bash
git clone <repo-url> rail
cd rail
dart pub get
./scripts/install_skill.sh
```

설치 후 Rail을 사용하는 권장 방식은 repo-owned Codex skill을 이용하는 것입니다.

## Rail로 얻을 수 있는 것

Rail은 즉흥적인 프롬프트 체인이 아니라 예측 가능한 하네스를 원하는 운영자를 위한 도구입니다.

Rail을 사용하면 다음을 할 수 있습니다.

- 하네스 YAML을 직접 쓰지 않고 의도에서 시작할 수 있음
- 별도의 프로젝트 저장소를 대상으로 bounded supervisor workflow를 실행할 수 있음
- planning, execution, evaluation, integration 산출물을 한 곳에 추적 가능하게 남길 수 있음
- `v1` release gate와 `v2` learning/review flow를 분리해서 운영할 수 있음
- supervisor의 판단을 명시적으로 남겨 장기적으로 개선할 수 있음

## Codex Skill로 Rail 사용하기

Rail의 가장 큰 장점은 CLI 자체가 아니라, Rail Codex skill을 통해 사용자가 goal, constraint, definition of done 수준에서 작업할 수 있다는 점입니다. 구조화와 라우팅은 하네스가 담당합니다.

대표적인 사용 예시는 다음과 같습니다.

```text
Use the Rail skill from the local Rail checkout.
Target repo: /absolute/path/to/target-repo
Goal: Fix the intermittent profile refresh loading issue.
Constraint: Do not change the API contract.
Definition of done: refresh completes reliably and related tests still pass.
```

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Refactor the settings flow safely without changing user-visible behavior.
Constraint: Keep the existing analytics events unchanged.
Definition of done: behavior is preserved and the change remains easy to review.
```

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Implement the first pass of offline retry handling for failed uploads.
Constraint: Keep the scope small and release-safe.
Definition of done: failed uploads can be retried and the change includes clear follow-up risks.
```

실제 사용에서는 skill이 요청을 구조화하고, repo-owned harness policy를 적용하고, 대상 저장소에 맞는 workflow를 시작합니다.

## V1 Workflow

`v1`은 core supervisor gate입니다.

흐름은 다음과 같습니다.

1. Rail이 사용자 요청을 구조화된 task로 변환합니다.
2. Supervisor가 planning, context building, generation, execution, evaluation 같은 bounded actor 흐름으로 작업을 라우팅합니다.
3. Evaluator가 pass, bounded retry, stop 중 하나를 결정합니다.
4. 결과로 대상 저장소에 대한 reviewable artifact set이 생성됩니다.

`v1`의 초점은 통제 가능한 execution outcome을 만드는 것입니다.

## V2 Workflow

`v2`는 `v1` 위에 integration과 learning review 흐름을 추가합니다.

흐름은 다음과 같습니다.

1. 통과한 `v1` 실행은 integration handoff를 만들 수 있습니다.
2. 운영자는 outcome feedback, learning candidate, hardening candidate를 검토할 수 있습니다.
3. Rail은 숨겨진 mutable state 대신, 리뷰된 파일로부터 queue와 evidence 상태를 다시 생성합니다.
4. 승인된 learning은 같은 family 범위 안에서 통제되게 promote됩니다.

`v2`의 초점은 개선 루프를 명시적이고, 리뷰 가능하고, 안전하게 진화시키는 것입니다.

## Architecture

사용자 안내 문서는 의도적으로 가볍게 유지합니다. supervisor, actor, rubric, artifact, learning state가 어떻게 연결되는지 시스템 관점에서 보려면 [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)를 참고하십시오.

한국어 구조 문서는 [docs/ARCHITECTURE-kr.md](docs/ARCHITECTURE-kr.md)에 있습니다.

## Current Scope

The `v1` runtime supports:

- request composition and validation
- artifact bootstrap
- actor brief generation
- sequential actor execution with `codex exec`
- evaluator-driven `revise` handling back to generator
- validation profiles (`standard`, `smoke`) for executor planning
- deterministic smoke fast-path execution for planner/context_builder/generator/executor/evaluator
- request-level validation roots and targets for narrowing standard-profile executor scope
- supervisor action loops that can route from evaluator back to generator, context_builder, or executor with bounded budgets
- review drafts are operator-authored, while queue, evidence, and approved-memory files are rail-derived snapshots
- approved memory is same-family only, with one active approved file per family
- pending review backlog is allowed, but broken derived state is not

The `v1` scope is intentionally constrained and does not treat these as in-gate:

- parallel actor orchestration
- project-specific adapters beyond the default Flutter + Riverpod profile
- `integrator`
- quality-learning review and apply flows
- hardened end-to-end validation across all task types

The `v2` operator surface now includes explicit file-based `init-*` and `apply-*` learning workflows. Operators edit the draft and decision files; rail regenerates the queue and evidence snapshots, and updates approved memory only on `promote` at `.harness/learning/approved/<task_family>.yaml`.
