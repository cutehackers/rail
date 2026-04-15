# Architecture Rules (Flutter + Riverpod / feature-based)

- Preserve existing feature-based module boundaries (`lib/ui`, `lib/data`, `lib/core`, `lib/utils`).
- Keep Riverpod provider patterns consistent with current project.
- `AsyncNotifier`, `AutoDisposeAsyncNotifier`, `StateNotifier`, `StateProvider`, `Provider` based on existing usage.
- `AutoDisposeAsyncNotifier` or `AsyncNotifier` are prefered for new Controllers. Use other type of Notifiers, if it's necessary to use other notifers for particulary case.
- Repository implementations remain in the data layer.
- Do not move files across architecture layers without explicit permission.
- Domain layer must not import Flutter UI framework APIs.
- Keep dependency direction (UI -> Controller -> Repository -> API) unless explicitly required.
- Prefer existing provider wiring and extension points; avoid re-plumbing DI at the app shell level.
- Reuse existing test patterns in adjacent feature tests.
- Keep shared shell UI (global tab navigation) unchanged unless requested.
- Do not introduce generated code edits; avoid touching `.g.dart` / `.freezed.dart` directly.
- Prefer immutable state and minimal diff for low-risk tasks.
- For list pages, keep `AppSmartRefresher` usage when pagination exists.
- Do not add helper methods named `_build*` inside `build()` to satisfy legacy constraints from repo guidance.
