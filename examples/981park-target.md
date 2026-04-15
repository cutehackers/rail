# Example target bootstrap

예시용 목표 저장소:

- `/absolute/path/to/target-flutter-app`

공통 요청 작성 예시:

```bash
cat >/absolute/path/to/request-draft.json <<'JSON'
{
  "request_version": "1",
  "project_root": "/absolute/path/to/target-flutter-app",
  "task_type": "bug_fix",
  "goal": "설명 가능한 버그를 재현 가능한 수정 단위로 정의",
  "context": {
    "feature": "app_feature",
    "suspected_files": [],
    "related_files": [],
    "validation_roots": [],
    "validation_targets": []
  },
  "constraints": [],
  "definition_of_done": [
    "관찰 가능한 수정 완료 상태가 정의된다",
    "관련 테스트 또는 영향 범위 검토가 가능하다",
    "analyze 기준을 충족한다"
  ]
}
JSON

rail compose-request --input /absolute/path/to/request-draft.json
```

이 단계의 Go CLI는 요청 초안 materialization에 집중합니다. 검증과 실행은 이후 workflow 단계에서 이어집니다.
