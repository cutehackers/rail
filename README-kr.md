# rail

`rail`은 자연어 작업 요청을 안전한 하네스 실행으로 바꾸는 Python-first control plane입니다.

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
