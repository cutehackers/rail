# 981park Target Example

Example target repository:

- `/Users/junhyounglee/workspace/981park-flutter-app`

Bootstrap example:

```bash
cd /Users/junhyounglee/workspace/rail

dart run bin/rail.dart compose-request \
  --task-type bug_fix \
  --goal "프로필 화면에서 pull-to-refresh 후 간헐적으로 로딩 인디케이터가 사라지지 않는 문제 수정" \
  --feature profile

dart run bin/rail.dart run \
  --request .harness/requests/<generated>.yaml \
  --project-root /Users/junhyounglee/workspace/981park-flutter-app
```

