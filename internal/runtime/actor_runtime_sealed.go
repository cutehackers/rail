package runtime

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"rail/internal/auth"

	"gopkg.in/yaml.v3"
)

const sealedActorRuntimeVersion = 1
const internalTestCodexOverrideValue = "rail-internal-tests-only"
const internalTestCodexMarker = ".rail-internal-test-codex"

var internalTestCodexOverrideEnabled string

type sealedActorRuntime struct {
	Root           string
	CodexHome      string
	Home           string
	XDGConfigHome  string
	XDGDataHome    string
	XDGCacheHome   string
	Temp           string
	ProvenancePath string
	CommandPath    string
	Env            []string
	SanitizedPath  string
	AuthSource     string
	AuthSecret     string
}

func prepareSealedActorRuntime(backend ActorBackendConfig, spec ActorCommandSpec, parent []string) (sealedActorRuntime, error) {
	if strings.TrimSpace(spec.ArtifactDirectory) == "" {
		return sealedActorRuntime{}, backendPolicyViolation("missing_artifact_directory_for_sealed_actor")
	}
	if strings.TrimSpace(spec.ActorRunID) == "" {
		return sealedActorRuntime{}, backendPolicyViolation("missing_actor_run_id_for_sealed_actor")
	}

	artifactDirectory, err := filepath.Abs(spec.ArtifactDirectory)
	if err != nil {
		return sealedActorRuntime{}, fmt.Errorf("resolve artifact directory for sealed actor: %w", err)
	}
	workingDirectory, err := filepath.Abs(spec.WorkingDirectory)
	if err != nil {
		return sealedActorRuntime{}, fmt.Errorf("resolve working directory for sealed actor: %w", err)
	}

	runID := sanitizeActorRunID(spec.ActorRunID)
	root := filepath.Join(artifactDirectory, "runtime", runID)
	sealed := sealedActorRuntime{
		Root:           root,
		CodexHome:      filepath.Join(root, "codex-home"),
		Home:           filepath.Join(root, "home"),
		XDGConfigHome:  filepath.Join(root, "xdg", "config"),
		XDGDataHome:    filepath.Join(root, "xdg", "data"),
		XDGCacheHome:   filepath.Join(root, "xdg", "cache"),
		Temp:           filepath.Join(root, "tmp"),
		ProvenancePath: filepath.Join(root, "actor_environment.yaml"),
	}

	parentMap := envSliceMap(parent)
	forbiddenRoots := actorForbiddenRoots(parentMap, artifactDirectory, workingDirectory)
	pathValue := strings.TrimSpace(parentMap["PATH"])
	if pathValue == "" {
		return sealedActorRuntime{}, backendPolicyViolation("unsafe_codex_path: missing PATH for sealed actor")
	}
	pathEntries, err := sanitizeActorPATH(pathValue, forbiddenRoots)
	if err != nil {
		return sealedActorRuntime{}, err
	}
	commandPath, err := resolveSealedCodexCommand(backend.Command, pathEntries, forbiddenRoots, parentMap)
	if err != nil {
		return sealedActorRuntime{}, err
	}
	sealed.CommandPath = commandPath
	sealed.SanitizedPath = strings.Join(pathEntries, string(os.PathListSeparator))

	apiKey, authSource, err := auth.ResolveOpenAIAPIKey(parentMap)
	if err != nil {
		return sealedActorRuntime{}, backendPolicyViolation("actor_auth_resolution_failed: " + err.Error())
	}
	if strings.TrimSpace(apiKey) == "" {
		return sealedActorRuntime{}, backendPolicyViolation("missing_env_auth_for_sealed_actor: run `rail auth login` or set OPENAI_API_KEY before running sealed actors")
	}
	parentMap["OPENAI_API_KEY"] = apiKey
	sealed.AuthSource = authSource
	sealed.AuthSecret = apiKey
	for _, path := range []string{
		sealed.Root,
		sealed.CodexHome,
		sealed.Home,
		sealed.XDGConfigHome,
		sealed.XDGDataHome,
		sealed.XDGCacheHome,
		sealed.Temp,
	} {
		if err := ensureSealedDirectory(artifactDirectory, path); err != nil {
			return sealedActorRuntime{}, err
		}
	}
	sealed.Env = buildSealedActorEnvironment(sealed, parentMap)
	if err := writeSealedActorProvenance(sealed, spec, backend); err != nil {
		return sealedActorRuntime{}, err
	}
	return sealed, nil
}

func sanitizeActorRunID(value string) string {
	value = strings.TrimSpace(value)
	var builder strings.Builder
	for _, r := range value {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '_', r == '-', r == '.':
			builder.WriteRune(r)
		default:
			builder.WriteByte('_')
		}
	}
	sanitized := strings.Trim(builder.String(), ".")
	if sanitized == "" {
		return "actor"
	}
	return sanitized
}

func actorRunIDFromLogPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, "-last-message.txt")
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return sanitizeActorRunID(base)
}

func ensureSealedDirectory(artifactDirectory string, path string) error {
	if err := ensureContainedPath(artifactDirectory, path); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return backendPolicyViolation("unsafe_sealed_runtime_path: symlink is not allowed at " + path)
		}
		if !info.IsDir() {
			return backendPolicyViolation("unsafe_sealed_runtime_path: non-directory exists at " + path)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect sealed runtime directory %s: %w", path, err)
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create sealed runtime directory %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("chmod sealed runtime directory %s: %w", path, err)
	}
	return nil
}

func ensureContainedPath(root string, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve containment root: %w", err)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve contained path: %w", err)
	}
	relative, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return fmt.Errorf("relativize contained path: %w", err)
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return backendPolicyViolation("unsafe_sealed_runtime_path: path escapes artifact directory: " + path)
	}
	return nil
}

func actorForbiddenRoots(parent map[string]string, artifactDirectory string, workingDirectory string) []string {
	roots := []string{artifactDirectory, workingDirectory}
	if value := strings.TrimSpace(parent["HOME"]); value != "" {
		roots = append(roots, value)
	}
	if userHome, err := os.UserHomeDir(); err == nil && strings.TrimSpace(userHome) != "" {
		roots = append(roots, userHome)
	}
	return roots
}

func sanitizeActorPATH(pathValue string, forbiddenRoots []string) ([]string, error) {
	rawEntries := filepath.SplitList(pathValue)
	if len(rawEntries) == 0 {
		return nil, backendPolicyViolation("unsafe_codex_path: empty PATH for sealed actor")
	}
	seen := map[string]struct{}{}
	safeEntries := []string{}
	for _, rawEntry := range rawEntries {
		entry := strings.TrimSpace(rawEntry)
		if entry == "" {
			continue
		}
		if !filepath.IsAbs(entry) {
			continue
		}
		cleanEntry := filepath.Clean(entry)
		if isForbiddenCodexPath(cleanEntry, forbiddenRoots) {
			continue
		}
		if !isTrustedCodexPath(cleanEntry) {
			continue
		}
		info, err := os.Stat(cleanEntry)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("stat PATH segment %s: %w", cleanEntry, err)
		}
		if !info.IsDir() {
			continue
		}
		if info.Mode().Perm()&0o022 != 0 {
			continue
		}
		resolvedEntry, err := filepath.EvalSymlinks(cleanEntry)
		if err != nil {
			return nil, fmt.Errorf("resolve PATH segment %s: %w", cleanEntry, err)
		}
		resolvedEntry = filepath.Clean(resolvedEntry)
		if isForbiddenCodexPath(resolvedEntry, forbiddenRoots) {
			continue
		}
		if !isTrustedCodexPath(resolvedEntry) {
			continue
		}
		if _, exists := seen[resolvedEntry]; exists {
			continue
		}
		seen[resolvedEntry] = struct{}{}
		safeEntries = append(safeEntries, resolvedEntry)
	}
	if len(safeEntries) == 0 {
		safeEntries = defaultTrustedPATHEntries()
	}
	if len(safeEntries) == 0 {
		return nil, backendPolicyViolation("unsafe_codex_path: no trusted PATH entries for sealed actor")
	}
	return safeEntries, nil
}

func resolveSealedCodexCommand(command string, pathEntries []string, forbiddenRoots []string, parent map[string]string) (string, error) {
	if command != "codex" {
		return "", backendPolicyViolation(fmt.Sprintf("unsafe_codex_path: unsupported actor backend command %q", command))
	}
	if testCommand, ok, err := resolveTestCodexCommand(parent); ok || err != nil {
		return testCommand, err
	}
	for _, dir := range pathEntries {
		candidate := filepath.Join(dir, command)
		info, err := os.Stat(candidate)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return "", fmt.Errorf("stat codex command %s: %w", candidate, err)
		}
		if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			return "", fmt.Errorf("resolve codex command %s: %w", candidate, err)
		}
		resolved = filepath.Clean(resolved)
		if isForbiddenCodexPath(resolved, forbiddenRoots) {
			return "", backendPolicyViolation("unsafe_codex_path: forbidden codex command " + resolved)
		}
		if !isTrustedCodexPath(resolved) {
			return "", backendPolicyViolation("unsafe_codex_path: untrusted codex command " + resolved)
		}
		return resolved, nil
	}
	return "", backendPolicyViolation("unsafe_codex_path: codex command not found on sanitized PATH")
}

func resolveTestCodexCommand(parent map[string]string) (string, bool, error) {
	if !internalTestCodexOverrideAllowed() {
		return "", false, nil
	}
	if strings.TrimSpace(parent["RAIL_INTERNAL_TEST_ALLOW_UNTRUSTED_CODEX_PATH"]) != internalTestCodexOverrideValue {
		return "", false, nil
	}
	rawPath := strings.TrimSpace(parent["RAIL_INTERNAL_TEST_CODEX_PATH"])
	if rawPath == "" {
		return "", true, backendPolicyViolation("unsafe_codex_path: internal test codex path is required")
	}
	if !filepath.IsAbs(rawPath) {
		return "", true, backendPolicyViolation("unsafe_codex_path: internal test codex path must be absolute")
	}
	resolved, err := filepath.EvalSymlinks(rawPath)
	if err != nil {
		return "", true, fmt.Errorf("resolve internal test codex command %s: %w", rawPath, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", true, fmt.Errorf("stat internal test codex command %s: %w", resolved, err)
	}
	if info.IsDir() || info.Mode().Perm()&0o111 == 0 {
		return "", true, backendPolicyViolation("unsafe_codex_path: internal test codex command is not executable")
	}
	markerPath := filepath.Join(filepath.Dir(resolved), internalTestCodexMarker)
	marker, err := os.ReadFile(markerPath)
	if err != nil {
		return "", true, backendPolicyViolation("unsafe_codex_path: internal test codex marker is missing")
	}
	if strings.TrimSpace(string(marker)) != filepath.Clean(resolved) {
		return "", true, backendPolicyViolation("unsafe_codex_path: internal test codex marker does not match command")
	}
	return filepath.Clean(resolved), true, nil
}

func internalTestCodexOverrideAllowed() bool {
	if internalTestCodexOverrideEnabled == internalTestCodexOverrideValue {
		return true
	}
	return strings.HasSuffix(filepath.Base(os.Args[0]), ".test") && flag.Lookup("test.v") != nil
}

func defaultTrustedPATHEntries() []string {
	entries := []string{}
	seen := map[string]struct{}{}
	for _, candidate := range []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
	} {
		info, err := os.Stat(candidate)
		if err != nil || !info.IsDir() || info.Mode().Perm()&0o022 != 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(candidate)
		if err != nil {
			continue
		}
		resolved = filepath.Clean(resolved)
		if !isTrustedCodexPath(resolved) {
			continue
		}
		if _, exists := seen[resolved]; exists {
			continue
		}
		seen[resolved] = struct{}{}
		entries = append(entries, resolved)
	}
	return entries
}

func isTrustedCodexPath(path string) bool {
	cleanPath := filepath.Clean(path)
	for _, trustedRoot := range []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/opt/homebrew/Caskroom/codex",
		"/usr/local/bin",
		"/usr/local/sbin",
		"/usr/local/Caskroom/codex",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
	} {
		if pathIsWithin(cleanPath, trustedRoot) {
			return true
		}
	}
	return false
}

func isForbiddenCodexPath(path string, forbiddenRoots []string) bool {
	normalized := filepathLikeSlash(filepath.Clean(path))
	if strings.Contains(normalized, "/.codex/") || strings.HasSuffix(normalized, "/.codex") {
		return true
	}
	for _, root := range forbiddenRoots {
		if strings.TrimSpace(root) == "" {
			continue
		}
		if pathIsWithin(path, root) {
			return true
		}
	}
	return false
}

func pathIsWithin(path string, root string) bool {
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, pathAbs)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) && !filepath.IsAbs(relative))
}

func buildSealedActorEnvironment(sealed sealedActorRuntime, parent map[string]string) []string {
	env := []string{
		"PATH=" + sealed.SanitizedPath,
		"CODEX_HOME=" + sealed.CodexHome,
		"HOME=" + sealed.Home,
		"XDG_CONFIG_HOME=" + sealed.XDGConfigHome,
		"XDG_DATA_HOME=" + sealed.XDGDataHome,
		"XDG_CACHE_HOME=" + sealed.XDGCacheHome,
		"TMPDIR=" + sealed.Temp,
		"TMP=" + sealed.Temp,
		"TEMP=" + sealed.Temp,
	}
	for _, key := range allowedSealedActorEnvKeys() {
		if value, ok := parent[key]; ok && strings.TrimSpace(value) != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}

func sealedActorRedactionSecrets(sealed sealedActorRuntime) []string {
	seen := map[string]struct{}{}
	secrets := []string{}
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		secrets = append(secrets, value)
	}
	add(sealed.AuthSecret)
	for _, entry := range sealed.Env {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || !isSecretBearingActorEnvKey(key) {
			continue
		}
		add(value)
	}
	sort.Slice(secrets, func(i, j int) bool {
		return len(secrets[i]) > len(secrets[j])
	})
	return secrets
}

func isSecretBearingActorEnvKey(key string) bool {
	normalized := strings.ToUpper(strings.TrimSpace(key))
	if normalized == "OPENAI_BASE_URL" {
		return true
	}
	for _, fragment := range []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PROXY"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}

func allowedSealedActorEnvKeys() []string {
	return []string{
		"OPENAI_API_KEY",
		"OPENAI_BASE_URL",
		"OPENAI_ORG_ID",
		"OPENAI_PROJECT",
		"HTTPS_PROXY",
		"HTTP_PROXY",
		"NO_PROXY",
		"ALL_PROXY",
		"https_proxy",
		"http_proxy",
		"no_proxy",
		"all_proxy",
		"SSL_CERT_FILE",
		"SSL_CERT_DIR",
		"RAIL_TEST_INVOCATION_PATH",
		"RAIL_TEST_CODEX_FAIL_ACTOR",
		"RAIL_TEST_CODEX_FAIL_ONCE_ACTOR",
		"RAIL_TEST_CODEX_SKIP_OUTPUT_ONCE_ACTOR",
		"RAIL_TEST_CODEX_VIOLATION_ACTOR",
	}
}

func envSliceMap(parent []string) map[string]string {
	values := make(map[string]string, len(parent))
	for _, entry := range parent {
		key, value, ok := strings.Cut(entry, "=")
		if ok {
			values[key] = value
		}
	}
	return values
}

func writeSealedActorProvenance(sealed sealedActorRuntime, spec ActorCommandSpec, backend ActorBackendConfig) error {
	envKeys := make([]string, 0, len(sealed.Env))
	for _, entry := range sealed.Env {
		key, _, ok := strings.Cut(entry, "=")
		if ok {
			envKeys = append(envKeys, key)
		}
	}
	sort.Strings(envKeys)
	value := map[string]any{
		"version":      sealedActorRuntimeVersion,
		"actor":        spec.ActorName,
		"actor_run_id": spec.ActorRunID,
		"auth_source":  sealed.AuthSource,
		"command_path": sealed.CommandPath,
		"sealed_root":  sealed.Root,
		"directories": map[string]string{
			"codex_home":      sealed.CodexHome,
			"home":            sealed.Home,
			"xdg_config_home": sealed.XDGConfigHome,
			"xdg_data_home":   sealed.XDGDataHome,
			"xdg_cache_home":  sealed.XDGCacheHome,
			"temp":            sealed.Temp,
		},
		"env_keys": envKeys,
		"backend_flags": map[string]any{
			"subcommand":             backend.Subcommand,
			"sandbox":                backend.Sandbox,
			"approval_policy":        backend.ApprovalPolicy,
			"ephemeral":              backend.Ephemeral,
			"capture_json_events":    backend.CaptureJSONEvents,
			"skip_git_repo_check":    backend.SkipGitRepoCheck,
			"ignore_user_config":     backend.IgnoreUserConfig,
			"ignore_rules":           backend.IgnoreRules,
			"sanitized_path_entries": len(filepath.SplitList(sealed.SanitizedPath)),
		},
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal sealed actor provenance: %w", err)
	}
	if err := os.WriteFile(sealed.ProvenancePath, data, 0o600); err != nil {
		return fmt.Errorf("write sealed actor provenance: %w", err)
	}
	return nil
}

func postflightSealedActorRuntime(sealed sealedActorRuntime) error {
	checks := []struct {
		path string
		code string
	}{
		{filepath.Join(sealed.CodexHome, "skills"), "unexpected_skill_materialization"},
		{filepath.Join(sealed.CodexHome, "superpowers"), "unexpected_skill_materialization"},
		{filepath.Join(sealed.CodexHome, "plugins"), "unexpected_plugin_materialization"},
		{filepath.Join(sealed.CodexHome, ".tmp", "plugins"), "unexpected_plugin_materialization"},
		{filepath.Join(sealed.CodexHome, "hooks"), "unexpected_hook_materialization"},
		{filepath.Join(sealed.CodexHome, "mcp"), "unexpected_mcp_materialization"},
		{filepath.Join(sealed.Home, ".codex", "skills"), "unexpected_skill_materialization"},
		{filepath.Join(sealed.Home, ".codex", "superpowers"), "unexpected_skill_materialization"},
		{filepath.Join(sealed.Home, ".codex", "plugins"), "unexpected_plugin_materialization"},
		{filepath.Join(sealed.Home, ".codex", "hooks"), "unexpected_hook_materialization"},
	}
	for _, check := range checks {
		hasEntries, err := directoryHasEntries(check.path)
		if err != nil {
			return err
		}
		if hasEntries {
			return backendPolicyViolation(check.code + " in " + check.path)
		}
	}
	return nil
}

func directoryHasEntries(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		if os.IsPermission(err) {
			return true, nil
		}
		return false, fmt.Errorf("inspect sealed runtime path %s: %w", path, err)
	}
	return len(entries) > 0, nil
}

func backendPolicyViolation(message string) error {
	return fmt.Errorf("backend_policy_violation: %s", message)
}
