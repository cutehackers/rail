# rail

`rail`은 Codex를 위한 설치형 control-plane입니다. 자연어로 작성한 엔지니어링 요청을, 별도의 대상 저장소에 대해 실행되는 bounded harness workflow로 바꾸는 역할을 합니다.

Rail은 변경 대상 애플리케이션 자체가 아닙니다. Rail이 소유하는 것은 workflow, policy, artifact, reviewed learning state입니다.

## 제품 모델

Rail의 사용 모델은 단순합니다.

- `rail`을 일반 CLI처럼 한 번 설치합니다.
- Codex에서는 번들된 Rail skill을 사용합니다.
- 프로젝트별 상태는 대상 저장소의 `.harness/` 안에 둡니다.
- 프로젝트가 로컬 override를 두지 않으면 설치된 제품의 embedded defaults를 사용합니다.

이 소스 저장소는 Rail의 개발 및 릴리즈 원본입니다. 최종 사용자가 런타임 루트로 체크아웃해 둘 필요는 없습니다.

## 설치

설치:

```bash
brew install rail
```

패키지 설치에는 다음이 포함됩니다.

- `rail` CLI
- 내장 Rail Codex skill
- embedded harness 기본 자산

즉, 최종 사용자는 별도로 Codex skill을 수동 설치할 필요가 없습니다.

## 빠른 시작

Rail을 적용할 대상 저장소에서 먼저 초기화합니다.

```bash
cd /absolute/path/to/target-repo
rail init
```

`rail init`은 최소한의 project-local `.harness/` 워크스페이스를 생성합니다.

- `.harness/project.yaml`
- `.harness/requests/`
- `.harness/artifacts/`
- `.harness/learning/`

그 다음의 일반적인 진입점은 Codex의 번들 Rail skill입니다.

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the intermittent profile refresh loading issue.
Constraint: Do not change the API contract.
Definition of done: refresh completes reliably and related tests still pass.
```

skill이 요청을 구조화하고, Rail이 이를 실제 request와 artifact 흐름으로 물질화합니다.

## 기본 워크플로우

일반적인 운영 순서는 다음과 같습니다.

1. `rail init`으로 대상 저장소를 초기화합니다.
2. Rail skill, `rail compose-request`, 또는 `rail init-request`로 요청 초안을 만듭니다.
3. `rail validate-request`로 요청을 검증합니다.
4. `rail run`으로 artifact를 bootstrap합니다.
5. `rail execute`로 bounded actor workflow를 실행합니다.
6. 필요하면 `rail route-evaluation`로 evaluator 결과를 다시 적용하거나 refresh합니다.
7. `v2`에서는 integration handoff와 reviewed learning update를 이어서 수행합니다.

직접 CLI만 사용할 때의 예시는 다음과 같습니다.

```bash
rail init --project-root /absolute/path/to/target-repo

cat /absolute/path/to/request-draft.json | rail compose-request --stdin

rail validate-request --request /absolute/path/to/target-repo/.harness/requests/request.yaml

rail run \
  --request /absolute/path/to/target-repo/.harness/requests/request.yaml \
  --project-root /absolute/path/to/target-repo

rail execute \
  --artifact /absolute/path/to/target-repo/.harness/artifacts/request
```

## 현재 CLI 명령

현재 Rail은 다음 operator 명령을 제공합니다.

- `init`
  - project-local `.harness` 워크스페이스 생성
- `init-request`
  - 현재 워크스페이스에 request template 생성
- `compose-request`
  - JSON request draft를 `.harness/requests/request.yaml`로 정규화
- `validate-request`
  - request schema 검증
- `run`
  - request용 artifact 디렉터리 bootstrap
- `execute`
  - artifact에 대해 bounded actor chain 실행
- `route-evaluation`
  - artifact의 evaluator 결과를 다시 적용하거나 persisted output을 refresh
- `validate-artifact`
  - artifact 파일을 schema 이름으로 검증
- `integrate`
  - passing run 이후 `v2` integration handoff 생성
- `init-user-outcome-feedback`
  - artifact로부터 user outcome feedback draft 생성
- `init-learning-review`
  - candidate로부터 learning review draft 생성
- `init-hardening-review`
  - candidate로부터 hardening review draft 생성
- `apply-user-outcome-feedback`
  - reviewed feedback 적용 후 derived state refresh
- `apply-learning-review`
  - reviewed learning decision 적용 후 derived state refresh
- `apply-hardening-review`
  - reviewed hardening decision 적용 후 derived state refresh
- `verify-learning-state`
  - derived learning state의 정합성 검증

## V1 과 V2

Rail은 릴리즈 표면을 명확하게 분리합니다.

### V1

`v1`은 bounded core supervisor gate입니다. 초점은 다음과 같습니다.

- request 정규화
- artifact bootstrap
- deterministic actor execution
- evaluator 기반 bounded retry
- 명시적인 terminal outcome

### V2

`v2`는 `v1` 위에 다음을 추가합니다.

- integration handoff 생성
- explicit user outcome feedback 파일
- explicit learning review 파일
- explicit hardening review 파일
- derived learning-state 검증

중요한 점은 review 파일은 운영자가 작성하고, queue, evidence index, approved family memory는 Rail이 파생 상태로 재생성한다는 것입니다.

## Project-Local `.harness`

각 대상 저장소는 자신의 `.harness/`를 소유합니다. 이것은 글로벌 체크아웃이 아니라 project-local state입니다.

여기에는 다음이 저장됩니다.

- project identity
- requests
- artifacts
- reviewed feedback
- reviewed learning decisions
- approved family memory

이 경로들은 프로젝트별 증거와 상태이므로 로컬에 남아야 합니다.

## Embedded Defaults 와 Override

Rail은 다음과 같은 reusable control-plane 자산에 대해 embedded defaults를 제공합니다.

- supervisor policy
- actor instructions
- rules and rubrics
- request / artifact templates

고급 사용자는 대상 저장소에 전체 파일을 두어 일부 기본값을 override할 수 있습니다.

- `.harness/supervisor/`
- `.harness/actors/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

override precedence는 다음과 같습니다.

1. 프로젝트 로컬 파일이 있으면 그것을 사용합니다.
2. 없으면 설치된 제품의 embedded default를 사용합니다.
3. override는 deep merge가 아니라 file-level override입니다.
4. `.harness/artifacts/`, `.harness/learning/`, `.harness/requests/` 같은 stateful 경로는 항상 로컬 상태입니다.

이 방식은 provenance를 명확히 하고, 업그레이드 시 숨겨진 병합 동작을 피하기 위한 선택입니다.

## Bundled Rail Skill

번들 Rail skill은 Codex에서 사용하는 일반적인 자연어 진입점입니다.

skill의 역할은 다음과 같습니다.

- 사용자 goal, constraint, definition of done을 해석
- 구조화된 request draft 추론
- 안전하지 않은 경우에만 최소한의 clarification 요청
- `rail compose-request`에 넘길 정규화된 draft 준비

일반 사용에서는 사용자가 하네스 YAML을 직접 작성할 필요가 없어야 합니다.

## 소스 저장소의 역할

이 저장소는 Rail 자체를 개발하고 릴리즈하는 사람들을 위한 저장소입니다.

여기에는 다음의 원본이 있습니다.

- Go CLI 구현
- `assets/defaults/` 아래의 embedded default harness 자산
- `assets/skill/` 아래의 번들 Rail skill 소스
- `skills/` 아래의 repo-owned skill 소스
- 릴리즈 도구와 패키징 설정
- architecture / release 문서

최종 사용자는 이 저장소를 체크아웃해 둘 필요가 없습니다. 설치된 `rail` 바이너리와, 대상 저장소의 project-local `.harness`만 있으면 됩니다.

## 추가 문서

- 구조 문서: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- 한국어 구조 문서: [docs/ARCHITECTURE-kr.md](docs/ARCHITECTURE-kr.md)
- 릴리즈 문서: [docs/releases/](docs/releases/)
- 현재 작업 목록: [docs/tasks.md](docs/tasks.md)
