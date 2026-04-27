# Rail 아키텍처

## 개요

Rail은 설치형 하네스 control-plane 제품입니다. 실제로 변경되는 다운스트림 애플리케이션 코드는 이 저장소에 있지 않습니다. 대신 Rail은 패키지된 CLI, 번들 Rail skill, embedded default control-plane asset, 그리고 다른 저장소를 대상으로 실행되는 runtime을 제공합니다.

핵심 모델은 다음과 같습니다.

- 설치된 제품이 재사용 가능한 기본값과 orchestration logic을 소유함
- 대상 저장소가 `.harness/` 아래의 project-local state를 소유함
- supervisor가 workflow 결정을 명시적이고 reviewable하게 유지함

## 제품 Runtime 모델

Rail은 일반적인 개발 도구처럼 설치해서 사용하도록 설계되었습니다.

```bash
brew install cutehackers/rail/rail
cd /absolute/path/to/target-repo
rail init
```

설치된 제품에는 다음이 포함됩니다.

- 네이티브 `rail` CLI
- 번들 Rail Codex skill
- supervisor policy, actor, rule, rubric, template를 위한 embedded defaults

소스 저장소는 이 자산들의 개발 및 기여 출처입니다. 최종 사용자가 반드시 checkout 해야 하는 runtime root는 아닙니다.

Rail은 먼저 `cutehackers/rail` Homebrew tap으로 배포됩니다. 배포
artifact는 `https://github.com/cutehackers/rail`의 GitHub Releases에
게시되고, GoReleaser가 태그된 CLI artifact, checksum, provenance, 그리고
바이너리와 제품 prefix 아래의 canonical 번들 Codex skill asset을 설치하는
tap formula를 생성합니다. 이후 `rail init`이 현재 Codex home에 user-facing
Codex skill copy를 regular file로 등록합니다. 이렇게 package-manager state와
per-user Codex discovery state를 분리하면서도 일반 설치 workflow에서 skill을
바로 사용할 수 있게 합니다.

Rail은 actor backend policy를 통해 Codex 실행 방식을 설정합니다. 첫 번째
backend는 Codex CLI이며, 로컬 기본 실행에는 `workspace-write` sandbox를
사용합니다.

Rail은 사용자가 대화하는 Codex 세션과 actor 실행용 sealed Codex 세션을
분리합니다. 사용자-facing 세션은 Rail skill을 사용하고, 필요한 질문을 하며,
운영자와 상호작용할 수 있습니다. 반면 actor 세션은 Rail backend policy로
실행되며 기본적으로 사용자의 skill, rule, plugin, hook, MCP surface를
상속하지 않습니다.

Sealed actor 세션은 운영자의 `CODEX_HOME`, `HOME`, `XDG_*`, temp directory,
shell identity, SSH agent, tool-specific config environment를 상속하지
않습니다. Rail은 run artifact 아래 actor-local runtime directory를 만들고,
비밀값을 제외한 provenance를 `runtime/<actor-run-id>/actor_environment.yaml`에
기록합니다. 사용자 Codex login state는 의도적으로 읽지 않기 때문에 standard
actor 실행은 명시적인 Rail actor auth를 사용합니다. 로컬 사용자는
`rail auth login`을 한 번 실행해 설정할 수 있고, CI는 `OPENAI_API_KEY`를
직접 사용할 수 있습니다.

## 핵심 Runtime 구성 요소

### Rail CLI

CLI는 실행 엔진입니다. 다음 책임을 가집니다.

- request 구성 및 정규화
- schema validation
- project 초기화
- artifact bootstrap
- actor orchestration
- evaluation routing
- terminal reporting

또한 actor backend policy를 로드하여 Codex 실행을 결정적으로 선택하고
형상화합니다.

### 번들 Rail Skill

Rail skill은 자연어 진입점입니다. 사용자의 goal, constraint, definition of done을 해석한 뒤 구조화된 draft를 CLI에 전달합니다. 이 skill은 `rail`이 `PATH`에 있다고 가정하지만, 로컬 소스 checkout은 가정하지 않습니다.

CLI는 user-level skill registration을 소유합니다. `rail init`은 번들 skill을
현재 Codex user skill root에 materialize하고, `rail doctor`는 missing, stale,
symlink 기반 registration 상태를 보고합니다. 문제가 있으면
`rail install-codex-skill --repair`로 복구합니다.

### Actor Backend Policy

Actor backend policy는 Rail이 각 bounded actor에 대해 Codex를 어떻게
호출할지 정의합니다. 체크인된 policy와 embedded default 모두 첫 backend로
Codex CLI를 사용합니다. backend 실행 evidence는 artifact의 `runs/`
디렉터리에 보관되어 전체 run state와 함께 검토 가능하게 유지됩니다.

기본적으로 actor Codex run은 isolation flag를 사용하고, 정리된 환경에서
실행되며, 캡처된 event에서 예상하지 못한 user-surface injection을 audit합니다.
Rail은 actor event와 artifact를 governance evidence로 취급합니다. 대화
transcript 자체를 workflow memory로 취급하지 않습니다.

### Embedded Defaults

Rail은 재사용 가능한 하네스 자산을 embedded defaults 형태로 배포합니다. 패키지 기본값에는 다음이 포함됩니다.

- supervisor policy
- actor instruction
- rule과 rubric
- request 및 artifact template

Embedded defaults는 제품 계약의 일부입니다. 프로젝트가 자체 하네스 파일을 아직 커스터마이즈하지 않았더라도, 설치된 제품만으로 알려진 baseline을 제공합니다.

### Project-Local `.harness`

각 대상 저장소는 project-local `.harness/` workspace를 소유합니다. 이것이 프로젝트별 control-plane 표면입니다.

반드시 로컬에 있어야 하는 상태:

- `.harness/project.yaml`
- `.harness/requests/`
- `.harness/artifacts/`
- `.harness/learning/`

이 경로들은 프로젝트 식별자, 실행 이력, evidence, reviewed memory를 담기 때문에 항상 로컬로 유지됩니다. 전역 fallback으로 대체되지 않습니다.

## Runtime 흐름

Runtime 흐름은 명시적으로 유지됩니다.

1. 사용자가 번들 Rail skill 또는 CLI를 직접 호출합니다.
2. Rail이 요청을 구조화된 task로 변환합니다.
3. Rail이 대상 저장소 안의 artifact state를 생성하거나 갱신합니다.
4. Supervisor가 bounded actor를 순차적으로 dispatch합니다.
5. 각 actor는 구조화된 출력을 artifact set에 기록합니다.
6. Evaluator가 pass, budget 안의 retry, stop 중 하나를 결정합니다.
7. 선택적인 `v2` integration 및 learning flow는 core run이 통과한 뒤 결과를 확장합니다.

이 설계는 숨겨진 자동화보다 review 가능한 전이를 우선합니다.

Rail은 현재 두 가지 실행 프로필을 유지합니다.

- 실제 대상 저장소 작업을 위한 기본 actor 경로로서의 `real` 모드
- control-plane 검증을 위한 빠르고 결정적인 경로로서의 `smoke` 모드

## Supervisor, Actors, Rubrics

제어 관계는 다음과 같습니다.

- supervisor는 workflow control을 소유함
- actor는 bounded execution을 소유함
- rubric과 rule은 결과 판단 기준을 정의함

Actor는 전체 workflow를 소유하지 않습니다. 좁은 범위의 단계를 수행하고 구조화된 출력을 반환합니다. Supervisor는 그 출력을 읽고 policy를 적용해 다음 동작을 결정합니다.

이 분리는 유지보수에 중요합니다.

- actor 동작은 전체 control loop를 다시 쓰지 않고도 진화할 수 있음
- routing policy는 모든 actor를 수정하지 않고도 변경할 수 있음
- release 판단은 평가 기준이 명시적이어서 계속 검토 가능함

## Advanced Overrides

Rail은 project-local `.harness` 파일을 통한 고급 커스터마이징을 지원합니다. 이것이 의도된 advanced override surface입니다.

대표적인 override 위치:

- `.harness/supervisor/`
- `.harness/actors/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

해결 순서는 단순합니다.

1. project-local 파일이 있으면 그것을 사용합니다.
2. 없으면 설치된 제품의 embedded defaults를 사용합니다.
3. 결과는 deep merge가 아니라 file-level override로 취급합니다.
4. stateful directory는 기본값과 무관하게 항상 project-local로 유지합니다.

### File-Level Override를 사용하는 이유

file-level override 규칙은 의도적입니다.

- override 출처를 명확하게 유지할 수 있음
- 디버깅이 단순해짐
- 제품 업그레이드 과정에서 숨겨진 merge semantics를 피할 수 있음
- 고급 커스터마이징을 명시적이고 reviewable하게 유지할 수 있음

제품은 policy나 template 파일의 부분 병합을 시도하지 않습니다. 프로젝트가 어떤 파일을 바꾸고 싶다면 그 파일 전체를 소유합니다.

## Artifacts 와 Learning State

Rail은 run artifact와 reviewed learning state를 분리합니다.

- run artifact는 한 task에서 실제로 무슨 일이 일어났는지를 설명함
- learning state는 검토된 결과 이후 시스템이 무엇을 기억해야 하는지를 설명함

`v2`에서는 운영자가 feedback이나 learning decision 같은 review 파일을 편집합니다. 이후 Rail이 그 reviewed file로부터 queue와 evidence state를 다시 생성해 derived state를 재현 가능하게 유지합니다.

Approved memory는 family 단위입니다. promote는 숨겨진 mutable state 체인을 만드는 대신 특정 task family의 활성 approved memory를 갱신합니다.

## V1 과 V2 의 경계

`v1`은 bounded core supervisor gate입니다. deterministic execution, corrective loop, explicit terminal outcome에 집중합니다.

`v2`는 `v1` 위에 다음을 추가합니다.

- pass 이후의 explicit integration handoff
- explicit learning 및 hardening review flow

이 경계는 release gate를 집중된 상태로 유지하면서, 개선 루프는 별도의 reviewable layer로 다룰 수 있게 합니다.

## 소스 저장소 구조

기여자 관점에서 이 저장소는 제품 빌드와 패키지 기본값 중심으로 구성됩니다.

- `cmd/rail/`는 제품 entrypoint를 담습니다
- `internal/`은 runtime, request, validation, install, reporting 패키지를 담습니다
- `assets/defaults/`는 embedded default harness asset을 담습니다
- `assets/skill/`은 번들 Rail skill source를 담습니다
- `packaging/`은 Homebrew formula 같은 release packaging material을 담습니다
- `docs/`는 architecture, release, operator 문서를 담습니다
