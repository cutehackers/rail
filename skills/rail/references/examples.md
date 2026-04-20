# Rail Examples

## Bug fix

User request:

`프로필 화면에서 pull-to-refresh 후 가끔 로딩 인디케이터가 사라지지 않는 문제를 고쳐줘`

Suggested draft:

```json
{
  "request_version": "1",
  "project_root": "/absolute/path/to/target-repo",
  "task_type": "bug_fix",
  "goal": "프로필 화면에서 pull-to-refresh 후 가끔 로딩 인디케이터가 사라지지 않는 문제 수정",
  "context": {
    "feature": "profile",
    "suspected_files": [
      "internal/profile/service.go"
    ],
    "related_files": [],
    "validation_roots": [],
    "validation_targets": []
  },
  "definition_of_done": [
    "refresh 완료 후 loading state가 정상 해제된다",
    "관련 테스트 또는 영향 범위 검토가 가능하다",
    "analyze 기준을 충족한다"
  ]
}
```

Materialize it:

```bash
cat /absolute/path/to/request-draft.json | rail compose-request --stdin
```

Execution mode:

- omit `validation_profile` for the normal `real` path
- use `validation_profile: smoke` only for harness-only verification

## Feature addition

User request:

`사용자 프로필 조회 결과를 5분간 메모리 캐시해줘`

Suggested draft:

```json
{
  "request_version": "1",
  "project_root": "/absolute/path/to/target-repo",
  "task_type": "feature_addition",
  "goal": "사용자 프로필 조회 결과를 5분간 메모리 캐시한다",
  "context": {
    "feature": "profile",
    "suspected_files": [],
    "related_files": [],
    "validation_roots": [],
    "validation_targets": []
  },
  "constraints": [
    "domain interface 변경 금지",
    "새 패키지 추가 금지"
  ],
  "definition_of_done": [
    "프로필 조회 결과가 5분 동안 재사용된다",
    "관련 테스트 또는 영향 범위 검토가 가능하다",
    "analyze 기준을 충족한다"
  ]
}
```

## Safe refactor

User request:

`club details 페이지의 build 로직을 섹션 단위로 분리해줘. 동작은 바뀌면 안 돼`

Suggested draft:

```json
{
  "request_version": "1",
  "project_root": "/absolute/path/to/target-repo",
  "task_type": "safe_refactor",
  "goal": "club details 화면의 build 로직을 섹션 단위로 분리",
  "context": {
    "feature": "club_details",
    "suspected_files": [],
    "related_files": [],
    "validation_roots": [],
    "validation_targets": []
  },
  "constraints": [
    "동작 변경 금지"
  ],
  "definition_of_done": [
    "동작이 바뀌지 않는다",
    "관련 테스트 또는 영향 범위 검토가 가능하다",
    "analyze 기준을 충족한다"
  ]
}
```

Materialize from a saved draft file:

```bash
rail compose-request --input /absolute/path/to/request-draft.json
```

## Smoke mode

User request:

`이 저장소의 Rail harness 연결만 빠르게 검증해줘. smoke mode로 실행하고 앱 소스는 수정하지 마`

Suggested draft:

```json
{
  "request_version": "1",
  "project_root": "/absolute/path/to/target-repo",
  "task_type": "test_repair",
  "goal": "Rail harness wiring only 빠르게 검증",
  "context": {
    "feature": "harness",
    "suspected_files": [],
    "related_files": [],
    "validation_roots": [],
    "validation_targets": []
  },
  "constraints": [
    "앱 소스 수정 금지",
    "harness wiring 확인에만 집중"
  ],
  "definition_of_done": [
    "smoke actor flow가 끝까지 실행된다",
    "앱 소스는 바뀌지 않는다",
    "smoke validation evidence가 남는다"
  ],
  "risk_tolerance": "low",
  "validation_profile": "smoke"
}
```
