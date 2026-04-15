# Project Rules

- This harness defaults to a Flutter + Riverpod project profile.
- Preserve the target repository's current architecture and existing provider wiring unless the task explicitly allows broader change.
- Keep repository implementations in the data layer and avoid public interface changes unless the task explicitly permits them.
- Use package imports and follow the target repository's lint rules.
- Do not manually edit generated files such as `*.g.dart` or `*.freezed.dart`.
- Prefer modern Dart and Flutter patterns and avoid deprecated APIs.
- Do not introduce callback names starting with `handle` or `_handle`; use `on*` or `_on*`.
- Do not use `_build*` helper methods from `build()`. Inline the widget tree or extract dedicated widget classes instead.
- Keep UI, data, and test changes scoped to the requested feature. Avoid drive-by refactors.
