# rail

`rail`은 Codex를 위한 skill-first harness control-plane입니다.

제품의 핵심 계약은 단순합니다.

- 사용자는 Rail skill에 자연어로 작업을 설명합니다
- Rail skill이 복잡한 harness request 형식을 대신 작성합니다
- Rail이 별도의 대상 저장소에 대해 bounded, reviewable workflow를 실행합니다

즉 Rail의 핵심은 사용자가 `.harness/requests/request.yaml`을 손으로 쓰지 않게 만드는 것입니다. 자연어 요청을 안전하고 일관된 harness 형식으로 바꾸는 것이 Rail의 본질입니다.

## 사용자는 실제로 무엇을 하나

설치는 한 번만 합니다.

```bash
brew install cutehackers/rail/rail
```

대상 저장소에서는 한 번만 초기화합니다.

```bash
cd /absolute/path/to/target-repo
rail init
```

`rail init`은 번들된 Rail skill을 활성 Codex 사용자 skill 루트에
일반 파일 형태로 등록합니다. 따라서 소스 체크아웃이 없어도 Codex가
해당 skill을 발견할 수 있고, 심볼릭 링크 디렉터리에도 의존하지 않습니다.
등록 상태가 꼬였다면 다음으로 복구할 수 있습니다.

```bash
rail doctor
rail install-codex-skill --repair
```

그 다음의 일반적인 진입점은 Codex의 번들 Rail skill입니다.

예시 프롬프트:

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Fix the intermittent profile refresh loading issue.
Constraint: Do not change the API contract.
Definition of done: refresh completes reliably and related tests still pass.
```

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Split the club details screen build logic into smaller sections.
Constraint: Preserve behavior exactly.
Definition of done: behavior is unchanged and focused validation still passes.
```

```text
Use the Rail skill.
Target repo: /absolute/path/to/target-repo
Goal: Verify the Rail harness wiring only.
Constraint: Smoke mode. Do not modify application source files.
Definition of done: the harness flow completes and leaves smoke evidence.
```

Rail skill은 request 필드를 추론하고, 안전하지 않은 경우에만 최소한으로 질문하고, 사용자가 YAML을 직접 작성하지 않아도 request를 물질화해야 합니다.

## 두 가지 실행 모드

### Real mode

`real` 모드는 기본 제품 경로입니다.

- 실제 제품 작업에 사용합니다
- Rail이 실제 actor 경로를 실행합니다
- generator가 대상 저장소 파일을 수정할 수 있습니다
- executor가 execution plan의 focused validation 명령을 실제로 실행합니다

내부 저장값은 `validation_profile: standard` 입니다.

### Smoke mode

`smoke` 모드는 빠른 control-plane 검증 경로입니다.

- harness 연결 확인
- release gate 증빙
- 빠른 wiring 검증

같은 용도에 적합합니다.

- workflow가 더 결정적이고 가볍게 동작합니다
- 일반적인 제품 변경 경로로 쓰면 안 됩니다
- 보통 “앱 소스는 수정하지 마” 같은 명시적 제약과 함께 사용합니다

내부 저장값은 `validation_profile: smoke` 입니다.

기준은 단순합니다.

- 대상 저장소를 실제로 바꾸고 검증해야 하면 `real`
- harness 경로 자체를 빨리 증명하면 되면 `smoke`

## Project-local `.harness`

각 대상 저장소는 자기 자신의 `.harness/`를 가집니다.

Rail은 다음 프로젝트별 상태를 여기에 둡니다.

- project identity
- requests
- artifacts
- reviewed learning state

이 상태는 항상 대상 저장소 로컬 상태입니다. Rail은 제품 사용을 위해 별도 공용 체크아웃을 요구하지 않습니다.

## Advanced Overrides

대부분의 사용자는 번들 기본값과 Rail skill만 사용하면 됩니다.

고급 사용자는 일부 기본값을 프로젝트 로컬 파일로 override할 수 있습니다.

- `.harness/supervisor/`
- `.harness/actors/`
- `.harness/rules/`
- `.harness/rubrics/`
- `.harness/templates/`

override 규칙은 단순합니다.

1. 프로젝트 로컬 파일이 있으면 그것을 사용합니다
2. 없으면 embedded default를 사용합니다
3. override는 deep merge가 아니라 file-level override입니다
4. `.harness/artifacts/`, `.harness/requests/`, `.harness/learning/` 같은 상태 디렉터리는 항상 로컬 상태입니다

고급 사용자가 알아둘 점:

- request 파일의 모드 값은 `validation_profile: standard|smoke`
- draft composition 단계에서는 `real`도 기본 `standard`의 alias로 허용됩니다
- `smoke`는 기본값이 아니라 명시적 opt-in으로 취급해야 합니다
- 이제 모든 task family는 `planner -> context_builder -> critic -> generator -> executor -> evaluator` 경로를 통과합니다
- `critic` 는 선택적 보조 단계가 아니라 필수 graph 단계입니다
- actor의 모델과 추론 강도는 체크인된 `.harness/supervisor/actor_profiles.yaml` 에서만 옵니다
- 환경 변수는 더 이상 기본 actor 품질 계약이 아닙니다
- actor command run은 actor-level timeout을 사용하지 않습니다
- `ActorWatchdog`은 command progress를 감시하고, actor가 관찰 가능한 진행을 멈추면 `actor_watchdog_expired`를 보고합니다

## 이 소스 저장소의 역할

이 저장소는 Rail 기여자와 릴리즈 엔지니어를 위한 저장소입니다.

여기에는 다음의 원본이 있습니다.

- Go runtime / CLI
- embedded default harness 자산
- 번들 Rail skill 소스
- 릴리즈 도구와 기여자 문서

최종 사용자는 Rail 제품을 쓰기 위해 이 저장소를 체크아웃해 둘 필요가 없습니다.

## 릴리즈 체크

Rail 자체를 다루는 기여자 기준:

- 자동 smoke gate: `./tool/v2_release_gate.sh`
- release gate workflow: `.github/workflows/release-gate.yml` (`main` 대상 PR에서 실행)
- 릴리즈 workflow: `.github/workflows/release.yml`

smoke gate는 request 생성, 실행, 통합, artifact 검증, learning-state 검증을
포함한 빠른 control-plane 경로를 증명합니다. Real actor command wiring은
profile에서 선택된 model과 reasoning 인자를 검증하는 runtime test로
확인하며, live helper script에 의존하지 않습니다.

`main` merge 시점에는 `v1` 또는 `v2` release gate를 다시 실행하지 않습니다.
해당 gate는 PR에서 이미 실행됩니다.

## 배포

현재 주 배포 채널은 `cutehackers/rail` Homebrew tap입니다.

```bash
brew install cutehackers/rail/rail
```

태그 릴리즈는 `https://github.com/cutehackers/rail`에서 GitHub 릴리즈 산출물을
배포한 뒤 Homebrew tap formula를 갱신합니다. GoReleaser 기반 릴리즈 파이프라인에서
`rail` 바이너리, 번들된 Rail Codex skill, checksum, provenance attestation을
같은 릴리즈 단위로 발행합니다.

Homebrew 패키지는 product prefix에 canonical skill asset을 설치하고,
`rail init`은 활성 Codex 사용자-facing copy를 materialize 합니다. 이후
`rail doctor`는 해당 복사본의 설치 여부를 정상/누락/오래됨/심볼릭 링크 기반
각 상태로 알려줍니다.

`homebrew/core`는 초기 채널이 아니라 뒤이은 배포 대상입니다. Rail이 공개
릴리즈 운용과 패키지 관리자 인지도를 안정화하는 동안 tap가 권장 install 경로입니다.

## 추가 문서

- 구조 문서: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- 한국어 구조 문서: [docs/ARCHITECTURE-kr.md](docs/ARCHITECTURE-kr.md)
- v1 릴리즈 계약: [docs/releases/v1-core-supervisor-gate.md](docs/releases/v1-core-supervisor-gate.md)
- v2 릴리즈 계약: [docs/releases/v2-integrator-and-learning-gate.md](docs/releases/v2-integrator-and-learning-gate.md)
