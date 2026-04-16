# Project Rules

- This harness defaults to a Go control-plane project profile.
- Preserve the target repository's current architecture and package boundaries unless the task explicitly allows broader change.
- Keep CLI entrypoints, runtime orchestration, policy files, and user-facing skills separated by responsibility.
- Avoid public interface changes unless the task explicitly permits them.
- Follow the target repository's formatting, build, and test conventions.
- Do not manually edit generated files such as `*.pb.go`, `*.gen.go`, or generated mocks unless the task explicitly requires regeneration.
- Keep implementation, policy, and test changes scoped to the requested feature. Avoid drive-by refactors.
