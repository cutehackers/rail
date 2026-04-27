# Browser Codex Auth for Rail Actors Design

**Date:** 2026-04-27

**Goal**

Replace Rail's pre-release API-key auth file flow with a browser-based Codex login flow while preserving the sealed actor runtime boundary.

Rail should ask a local user to authenticate once through Codex's normal browser login. That login should persist across terminal sessions and target repositories, but actor execution must continue to run in artifact-local sealed `CODEX_HOME` directories without inheriting the user's normal Codex home, skills, plugins, hooks, MCP configuration, or rules.

## 쉬운 설명

Rail은 사용자가 매번 API key를 붙여 넣게 만들면 안 된다. 그리고 Rail actor가 사용자의 평소 Codex 환경 전체를 그대로 물려받아도 안 된다.

원하는 동작은 다음과 같다.

```text
rail auth login
  Rail 전용 Codex auth home을 만든다.
  그 환경에서 codex login을 실행한다.
  사용자는 브라우저에서 한 번 인증한다.

다른 세션 또는 다른 target repo
  rail auth doctor가 같은 Rail auth home을 확인한다.
  로그인 상태가 유효하면 다시 로그인하지 않는다.

actor 실행
  artifact-local sealed CODEX_HOME을 만든다.
  Rail 전용 auth home에서 실행에 필요한 인증 material만 전달한다.
  일반 사용자 Codex home은 읽지 않는다.
```

핵심은 인증은 사용자 단위로 지속되지만, actor 실행 환경은 run 단위로 계속 격리된다는 점이다.

## Problem Statement

현재 pre-release 구현은 `rail auth login`이 OpenAI API key를 입력받아 Rail 전용 YAML 파일에 저장한다. 파일은 private permission으로 보호되고 secret value를 출력하지 않지만, 제품 계약으로 보기에는 다음 문제가 있다.

- 사용자가 장기 API key를 직접 다뤄야 한다.
- Rail이 별도 secret file format을 소유하게 된다.
- local browser login이 제공하는 account-level controls, revocation, token lifecycle을 활용하지 못한다.
- actor runtime은 결국 `OPENAI_API_KEY`를 환경변수로 주입받는다.

이 기능은 아직 배포되지 않았으므로 이전 API-key 저장 방식과 호환성을 유지할 필요가 없다. 출시 auth 계약은 browser-based Codex login을 기준으로 설계한다.

## Design Principles

- `rail auth login`은 사용자가 API key를 입력하는 명령이 아니다.
- Rail auth state는 사용자 단위로 지속된다.
- Rail auth state는 target repository나 `.harness/artifacts/` 아래에 저장하지 않는다.
- Rail은 사용자의 normal Codex home을 actor에게 상속하지 않는다.
- actor별 sealed `CODEX_HOME`은 계속 artifact-local이어야 한다.
- actor에게 전달되는 것은 인증에 필요한 최소 material뿐이다.
- `rail auth doctor`는 actor 실행 전에 같은 auth source를 검증해야 한다.
- 문서와 예시는 home-directory path를 노출하지 않는다.

## Target User Contract

Local users authenticate once:

```bash
rail auth login
rail auth doctor
```

`rail auth login` opens or initiates Codex's browser login flow. After it succeeds, Rail stores Codex login state in a Rail-owned user-scoped auth home under the platform user config location.

Users should not need to log in again when they:

- start a new terminal session
- switch to another target repository
- run another Rail request in the same machine account
- resume a harness workflow from an existing artifact

Users should need to log in again when:

- they ran `rail auth logout`
- Codex login status reports that the stored credential is expired or revoked
- the Rail auth home was deleted
- auth home permissions or integrity checks fail
- the installed Codex CLI can no longer read the stored auth format

`rail auth status` reports whether Rail actor auth is configured. `rail auth doctor` fails closed and prints the next step when actor auth is not ready.

## CLI Behavior

### `rail auth login`

`rail auth login` should:

1. Resolve the user-scoped Rail Codex auth home.
2. Create it with private permissions.
3. Reject symlinked or group/world-writable auth paths.
4. Execute `codex login` with `CODEX_HOME` pointing at the Rail auth home.
5. Preserve enough terminal/browser behavior for Codex's login flow to complete.
6. Run `codex login status` with the same `CODEX_HOME` after login.
7. Report success without printing tokens, secret values, or concrete home-directory paths.

The command should not accept or prompt for an API key by default. A non-browser auth mode is outside this design unless a later CI or headless runner design adds it explicitly.

### `rail auth status`

`rail auth status` should:

- run `codex login status` with the Rail auth home
- report configured or not configured
- avoid exposing token details or auth file internals
- avoid reading the user's normal Codex home

### `rail auth doctor`

`rail auth doctor` should be the preflight command used by the Rail skill before standard actor execution.

It should:

- validate Rail auth home path safety
- verify `codex` is available on the trusted actor command path
- run `codex login status` with the Rail auth home
- fail closed when login is missing, expired, unreadable, or ambiguous
- tell the user to run `rail auth login`

### `rail auth logout`

`rail auth logout` should:

- execute `codex logout` with `CODEX_HOME` pointing at the Rail auth home when supported
- remove Rail-owned auth material if Codex logout does not fully clean it up
- leave the user's normal Codex login untouched
- report that Rail actor auth was removed

## Auth Storage Model

Rail owns a user-scoped Codex auth home. This is not the target repository and not the artifact directory.

The storage boundary is:

```text
Platform user config location
  rail/
    codex-auth-home/
      Codex-managed auth material
      Rail-managed marker/provenance files
```

The exact files inside `codex-auth-home` are Codex-managed implementation details. Rail should not parse tokens directly unless there is no stable CLI status mechanism. The preferred contract is subprocess-based:

```text
CODEX_HOME=<rail-codex-auth-home> codex login
CODEX_HOME=<rail-codex-auth-home> codex login status
CODEX_HOME=<rail-codex-auth-home> codex logout
```

Rail may write a small marker file next to the Codex-managed files to record Rail auth-home version, created time, and owning Rail version. That marker must not contain secrets.

## Sealed Actor Runtime Data Flow

Actor execution remains sealed and artifact-local:

```text
Rail user-scoped auth home
  persistent browser login state

Rail run artifact
  runtime/<actor-run-id>/codex-home/
    temporary actor CODEX_HOME
    minimal copied or mounted auth material
    no user skills
    no user plugins
    no hooks
    no MCP config
```

Before launching `codex exec`, Rail should materialize the actor's sealed `CODEX_HOME` from an allowlist:

- required Codex auth files or directories
- minimal Rail-generated config needed for the actor backend
- no user config copied from the normal Codex home
- no user skill, plugin, hook, MCP, or rule directories

The implementation must discover the Codex auth material shape empirically against the supported Codex CLI version and encode it as an allowlist. It must not copy the entire Rail auth home blindly if that home can accumulate non-auth files over time.

Actor provenance should record:

- `auth_source: rail_codex_login`
- Rail auth home status as ready or unavailable
- whether auth material was copied into the sealed runtime
- no token values
- no concrete user home path in generated user-facing reports

## Runtime Failure Modes

When auth is unavailable, actor preparation should fail before creating or running an actor command.

Expected errors:

- `rail_actor_auth_not_configured`: `rail auth login` has not succeeded.
- `rail_actor_auth_expired`: Codex login status reports an expired or invalid login.
- `rail_actor_auth_home_unsafe`: auth home path is symlinked, writable by group/world, or otherwise unsafe.
- `rail_actor_auth_materialization_failed`: Rail could not copy the required auth material into the sealed actor runtime.
- `rail_actor_codex_login_status_failed`: `codex login status` failed in a way Rail cannot classify safely.

All failures must avoid printing secrets.

## Request And Skill Contract

The Rail skill should continue to run `rail auth doctor` before standard actor execution.

Skill text should change from API-key language to browser login language:

- do not ask users to pass API keys in prompts
- if `rail auth doctor` fails, run `rail auth login` once
- explain that login persists for this machine account across target repositories
- do not run `rail auth login` on every skill trigger

The bundled skill under `assets/skill/Rail/` must stay aligned with `skills/Rail/`.

## Documentation Changes

Update operator docs to describe:

- browser-based `rail auth login`
- user-scoped Rail auth persistence
- sealed actor runtime still using artifact-local `CODEX_HOME`
- `rail auth logout` affects only Rail actor auth
- no API key prompt in the normal local flow

Documentation must use placeholders such as `/absolute/path/to/target-repo` when examples need paths. It must not include machine-specific home-directory paths.

## Tests And Validation

Unit tests should cover:

- Rail auth home path resolution and permission checks
- symlink rejection for auth home and copied auth material
- `rail auth doctor` using the Rail auth home rather than normal Codex home
- `rail auth logout` not touching normal Codex home
- actor runtime provenance records `rail_codex_login` without secrets
- actor runtime refuses to run when browser login status is unavailable
- actor runtime does not copy skills, plugins, hooks, MCP config, or rules into sealed `CODEX_HOME`

Integration validation should cover the smallest real flow:

1. Build Rail.
2. Run `rail auth login` on a machine with a supported Codex CLI.
3. Run `rail auth doctor`.
4. Compose or use an existing standard request.
5. Run a standard actor flow.
6. Inspect the produced actor runtime directory and provenance.
7. Confirm no user Codex home content unrelated to auth appears in the actor runtime.

Automated tests should use a fake `codex` command for login/status/logout behavior. Real browser login should remain a manual validation path.

## Out Of Scope

- Backward compatibility with the pre-release Rail API-key YAML auth file.
- CI or headless non-browser auth.
- Parsing or refreshing Codex tokens directly.
- Reusing the user's normal Codex home for actor execution.
- Changing actor routing, evaluator authority, or request schemas.

## Implementation Plan Shape

The follow-up implementation plan should be split into these units:

1. Replace the auth package API-key bundle with a Rail Codex auth-home abstraction.
2. Wrap `codex login`, `codex login status`, and `codex logout` in CLI auth commands.
3. Add auth materialization into sealed actor runtime from an allowlist.
4. Update provenance and error taxonomy.
5. Update Rail skill copies and docs.
6. Add unit tests with a fake Codex binary.
7. Run focused auth tests, sealed runtime tests, and `go test ./...`.

## Open Implementation Questions

- Which exact Codex auth files must be copied into a sealed actor `CODEX_HOME` for the supported Codex CLI version?
- Does `codex logout` fully clean the custom `CODEX_HOME`, or should Rail remove the Rail auth home after invoking it?
- Should Rail expose an advanced `RAIL_CODEX_AUTH_HOME` override for tests and operators, or keep it internal?
- Should `rail auth doctor` verify only login status, or also perform a minimal non-mutating `codex exec` smoke check in a temp sealed home?
