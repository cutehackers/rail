# rail

`rail`은 자연어 작업 요청을 안전한 하네스 실행으로 바꾸는 Python-first control plane입니다.

정식 제품 계약 문서는 `docs/SPEC.md`입니다.

핵심 계약:

- 사용자는 Rail skill을 통해 작업을 자연어로 설명한다.
- Rail skill은 구조화된 request draft를 만든다.
- Python Rail Harness Runtime은 request를 정규화하고 artifact handle을 할당한다.
- supervisor는 Actor Runtime을 통해 bounded actor graph를 실행한다.
- 결과는 artifact만 읽어 projection한다.

## 주요 API

```python
import rail

handle = rail.start_task(draft)
rail.supervise(handle)
result = rail.result(handle)
```

새 목표는 항상 새 artifact를 만들고, 기존 작업 재개는 artifact handle이 있을 때만 진행한다.

## Actor Runtime

Actor Runtime은 SDK 기반 actor 실행 경계다. Rail은 prompt, output schema, policy, evidence, routing을 소유한다. actor는 target repository를 직접 수정하지 않는다.

## Mutation Boundary

target 변경은 외부 sandbox에서 만들어진 patch bundle을 Rail이 검증하고 적용할 때만 발생한다.

## 검증

```bash
uv run --python 3.12 pytest -q
uv run --python 3.12 ruff check src tests
```

## 설치

Python 패키지 배포 이름은 `rail-sdk`입니다. 이 패키지는 Rail Python API,
번들된 Rail skill asset, 설치 보조 명령을 제공합니다. 작업 실행 자체는
CLI 제품 계약이 아니라 Rail skill과 Python API 계약을 따릅니다.

패키지를 설치하고 operator SDK credential을 설정합니다:

```bash
export OPENAI_API_KEY=...
uv tool install rail-sdk
```

로컬 Rail skill을 설치하거나 갱신한 뒤 준비 상태를 확인합니다:

```bash
rail migrate
rail doctor
```

기존 Homebrew `rail` binary가 `PATH`에서 먼저 잡히면 패키지 이름 entrypoint를
먼저 사용합니다:

```bash
rail-sdk migrate
rail-sdk doctor
```

그 다음 `rail doctor`가 안내하는 기존 formula 제거 명령을 실행합니다.
일반적인 정리 명령은 다음과 같습니다:

```bash
brew uninstall rail
brew cleanup rail
```

설치가 끝나면 target repository에서 Rail skill에 자연어로 요청합니다:

```text
Use the Rail skill.
프로필 로딩 버그를 수정해줘.
```

이미 target repository 안에서 작업 중이라면 일반 사용자는 runtime feature flag나
target path를 따로 지정할 필요가 없습니다.

## 릴리스 배포(운영자)

공지/체크포인트는 단일 소스로 `CHANGELOG.md`만 사용합니다.

- [CHANGELOG.md](/absolute/path/to/rail/CHANGELOG.md)

릴리스는 이제 업로드 커맨드가 아니라 태그 푸시로 트리거됩니다.
Homebrew는 더 이상 배포 실행 경로가 아니며, 기존 설치 정리는 종료용으로만 필요합니다.

릴리스 트리거 권장 절차:

1) publish script를 실행합니다.

```bash
./publish.sh v0.6.1
```

해당 버전의 `CHANGELOG.md` 최상단 항목이 없으면 스크립트는 이전 릴리스
태그 이후 변경점을 바탕으로 항목을 생성한 뒤 changelog 품질을 검증합니다.
이미 항목이 있으면 그대로 보존하고 품질만 검증합니다. 이후 `pyproject.toml`과
`uv.lock`을 갱신하고, `scripts/release_gate.sh`를 실행하고, 필요한 release
metadata 변경분을 커밋한 뒤 `main`과 release tag를 push합니다.

`v*` 태그 푸시가 `.github/workflows/publish.yml`을 실행합니다.
워크플로우는 다음을 검증합니다.

- 태그 버전과 `pyproject.toml` 버전이 일치
- `CHANGELOG.md` 최상단 항목 버전이 태그와 일치
- 로컬 릴리스 게이트 통과
- `uv build` 결과 생성
- PyPI Trusted Publishing으로 PyPI 업로드
- 같은 `CHANGELOG.md` 항목으로 GitHub 릴리스 노트 생성

사용자 대상 공지와 체크포인트는 항상 `CHANGELOG.md`만 반영합니다. 배포 후 사용자 설치는 다음과 같이 진행합니다.

```bash
uv tool install rail-sdk==${VERSION}
```
