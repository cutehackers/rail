# Rail Examples

## Bug fix

User request:

`프로필 화면에서 pull-to-refresh 후 가끔 로딩 인디케이터가 사라지지 않는 문제를 고쳐줘`

Suggested command shape:

```bash
dart run bin/rail.dart compose-request \
  --task-type bug_fix \
  --goal "프로필 화면에서 pull-to-refresh 후 가끔 로딩 인디케이터가 사라지지 않는 문제 수정" \
  --feature profile \
  --dod "refresh 완료 후 loading state가 정상 해제된다" \
  --dod "관련 테스트 또는 영향 범위 검토가 가능하다" \
  --dod "analyze 기준을 충족한다"
```

## Feature addition

User request:

`사용자 프로필 조회 결과를 5분간 메모리 캐시해줘`

Suggested command shape:

```bash
dart run bin/rail.dart compose-request \
  --task-type feature_addition \
  --goal "사용자 프로필 조회 결과를 5분간 메모리 캐시한다" \
  --feature profile \
  --constraint "domain interface 변경 금지" \
  --constraint "새 패키지 추가 금지"
```

## Safe refactor

User request:

`club details 페이지의 build 로직을 섹션 단위로 분리해줘. 동작은 바뀌면 안 돼`

Suggested command shape:

```bash
dart run bin/rail.dart compose-request \
  --task-type safe_refactor \
  --goal "club details 화면의 build 로직을 섹션 단위로 분리" \
  --feature club_details \
  --risk-tolerance medium \
  --constraint "동작 변경 금지"
```

