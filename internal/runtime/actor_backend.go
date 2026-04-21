package runtime

import (
	"fmt"
	"strings"

	"rail/internal/assets"

	"gopkg.in/yaml.v3"
)

type ActorBackendPolicy struct {
	Version              int                                `yaml:"version"`
	ExecutionEnvironment string                             `yaml:"execution_environment"`
	DefaultBackendName   string                             `yaml:"default_backend"`
	Backends             map[string]ActorBackendConfig      `yaml:"backends"`
	Environments         map[string]ActorBackendEnvironment `yaml:"execution_environments"`
}

type ActorBackendConfig struct {
	Command           string `yaml:"command"`
	Subcommand        string `yaml:"subcommand"`
	Sandbox           string `yaml:"sandbox"`
	ApprovalPolicy    string `yaml:"approval_policy"`
	SessionMode       string `yaml:"session_mode"`
	Ephemeral         bool   `yaml:"ephemeral"`
	CaptureJSONEvents bool   `yaml:"capture_json_events"`
	SkipGitRepoCheck  bool   `yaml:"skip_git_repo_check"`
}

type ActorBackendEnvironment struct {
	AllowedSandboxes []string `yaml:"allowed_sandboxes"`
}

func loadActorBackendPolicy(projectRoot string) (ActorBackendPolicy, error) {
	data, source, err := assets.Resolve(projectRoot, ".harness/supervisor/actor_backend.yaml")
	if err != nil {
		return ActorBackendPolicy{}, fmt.Errorf("resolve actor backend policy: %w", err)
	}

	var policy ActorBackendPolicy
	if err := yaml.Unmarshal(data, &policy); err != nil {
		return ActorBackendPolicy{}, fmt.Errorf("decode actor backend policy from %s policy: %w", source, err)
	}

	if policy.Version != 1 {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy version must be 1, got %d", policy.Version)
	}
	if strings.TrimSpace(policy.ExecutionEnvironment) == "" {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy must define execution_environment")
	}
	if policy.ExecutionEnvironment != "local" {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy execution_environment %q is not supported; only local is supported until execution environment selection is trusted", policy.ExecutionEnvironment)
	}
	if strings.TrimSpace(policy.DefaultBackendName) == "" {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy must define default_backend")
	}
	if policy.DefaultBackendName != "codex_cli" {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy only supports codex_cli, got %q", policy.DefaultBackendName)
	}
	if len(policy.Backends) == 0 {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy must define backends")
	}
	if len(policy.Environments) == 0 {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy must define execution_environments")
	}
	if len(policy.Backends) != 1 {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy only supports codex_cli")
	}

	backend, ok := policy.Backends["codex_cli"]
	if !ok {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy only supports codex_cli")
	}
	if err := validateActorBackendConfig(backend); err != nil {
		return ActorBackendPolicy{}, err
	}

	env, ok := policy.Environments[policy.ExecutionEnvironment]
	if !ok {
		return ActorBackendPolicy{}, fmt.Errorf("actor backend policy execution_environment %q is not defined", policy.ExecutionEnvironment)
	}
	if err := validateActorBackendEnvironment(policy.ExecutionEnvironment, env, backend.Sandbox); err != nil {
		return ActorBackendPolicy{}, err
	}

	return policy, nil
}

func (p ActorBackendPolicy) DefaultBackend() (ActorBackendConfig, error) {
	if strings.TrimSpace(p.DefaultBackendName) == "" {
		return ActorBackendConfig{}, fmt.Errorf("actor backend policy must define default_backend")
	}
	backend, ok := p.Backends[p.DefaultBackendName]
	if !ok {
		return ActorBackendConfig{}, fmt.Errorf("actor backend policy default_backend %q is not defined", p.DefaultBackendName)
	}
	return backend, nil
}

func validateActorBackendConfig(config ActorBackendConfig) error {
	if config.Command != "codex" {
		return fmt.Errorf("actor backend command must be codex, got %q", config.Command)
	}
	if config.Subcommand != "exec" {
		return fmt.Errorf("actor backend subcommand must be exec, got %q", config.Subcommand)
	}
	switch config.Sandbox {
	case "read-only", "workspace-write", "danger-full-access":
	default:
		return fmt.Errorf("actor backend sandbox must be read-only, workspace-write, or danger-full-access, got %q", config.Sandbox)
	}
	switch config.ApprovalPolicy {
	case "untrusted", "on-request", "never":
	default:
		return fmt.Errorf("actor backend approval_policy must be untrusted, on-request, or never, got %q", config.ApprovalPolicy)
	}
	if config.SessionMode != "per_actor" {
		return fmt.Errorf("actor backend session_mode must be per_actor, got %q", config.SessionMode)
	}
	return nil
}

func validateActorBackendEnvironment(environmentName string, environment ActorBackendEnvironment, sandbox string) error {
	for _, allowedSandbox := range environment.AllowedSandboxes {
		if allowedSandbox == sandbox {
			return nil
		}
	}
	return fmt.Errorf("actor backend sandbox %s is not allowed in execution_environment %q", sandbox, environmentName)
}
