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
