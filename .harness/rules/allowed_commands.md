# Allowed Commands

The executor should use these commands in this order for standard validation:

1. `dart format <files>` (minimal scope first, then broader if needed)
2. `flutter analyze`
3. `flutter test <target-test-path>`
4. `flutter test` (when broad validation is necessary)

Disallow by default:
- `flutter pub global` operations
- destructive git commands (`git reset --hard`, `git checkout -- <path>`)
- unrelated dependency updates

