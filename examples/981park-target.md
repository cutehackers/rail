# Example target bootstrap

예시용 목표 저장소:

- `/absolute/path/to/target-flutter-app`

공통 부트스트랩 예시:

```bash
cd /absolute/path/to/rail

dart run bin/rail.dart compose-request \
  --task-type bug_fix \
  --goal "설명 가능한 버그를 재현 가능한 수정 단위로 정의" \
  --feature app_feature

dart run bin/rail.dart run \
  --request .harness/requests/<generated>.yaml \
  --project-root /absolute/path/to/target-flutter-app
```

또는 요청이 준비되어 있다면 검증만 바로 수행할 수 있습니다.

```bash
dart run bin/rail.dart validate-request \
  --request .harness/requests/<request-file>.yaml
```
