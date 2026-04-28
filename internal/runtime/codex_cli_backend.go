package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"rail/internal/contracts"

	"gopkg.in/yaml.v3"
)

type CodexCLIBackend struct{}

type RuntimeEvidence struct {
	SchemaVersion    int                      `yaml:"schema_version"`
	BackendType      string                   `yaml:"backend_type"`
	Actor            string                   `yaml:"actor"`
	ActorRunID       string                   `yaml:"actor_run_id"`
	Status           string                   `yaml:"status"`
	RawEventLog      string                   `yaml:"raw_event_log"`
	Provenance       string                   `yaml:"provenance"`
	Policy           RuntimeEvidencePolicy    `yaml:"policy"`
	Redaction        RuntimeEvidenceRedaction `yaml:"redaction"`
	PolicyViolations []string                 `yaml:"policy_violations"`
}

type RuntimeEvidencePolicy struct {
	Sandbox string `yaml:"sandbox"`
}

type RuntimeEvidenceRedaction struct {
	SecretValuesWritten bool `yaml:"secret_values_written"`
}

func (CodexCLIBackend) RunActor(ctx context.Context, invocation ActorInvocation) (ActorResult, error) {
	profile, err := normalizeActorProfile(invocation.ActorName, invocation.Profile)
	if err != nil {
		return ActorResult{}, err
	}
	invocation.Profile = profile
	if invocation.Policy.CaptureJSONEvents && strings.TrimSpace(invocation.EventsPath) == "" {
		return ActorResult{}, fmt.Errorf("capture JSON events requires events path")
	}
	artifactDirectory, err := filepath.Abs(invocation.ArtifactDirectory)
	if err != nil {
		return ActorResult{}, fmt.Errorf("resolve artifact directory for %s: %w", invocation.ActorName, err)
	}
	if err := validateActorInvocationPaths(artifactDirectory, invocation); err != nil {
		return ActorResult{}, err
	}

	callerCtx := ctx
	runCtx, cancel := context.WithCancel(callerCtx)
	defer cancel()

	spec := actorCommandSpecFromInvocation(invocation)
	sealed, err := prepareSealedActorRuntime(invocation.Policy, spec, os.Environ())
	if err != nil {
		return ActorResult{}, err
	}

	cmd := exec.CommandContext(runCtx, sealed.CommandPath, buildCodexCLIArgsForInvocation(invocation)...)
	cmd.Dir = invocation.WorkingDirectory
	cmd.Env = sealed.Env

	output := &synchronizedBuffer{}
	watchdog := newActorWatchdog(invocation.ActorName, defaultActorWatchdogConfig)
	progressWriter := watchdog.ProgressWriter()
	stdoutWriters := []io.Writer{output, progressWriter}
	var eventsFile *os.File
	if invocation.Policy.CaptureJSONEvents {
		eventsFile, err = os.OpenFile(invocation.EventsPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
		if err != nil {
			return ActorResult{}, fmt.Errorf("create JSON events log for %s: %w", invocation.ActorName, err)
		}
		defer func() {
			if eventsFile != nil {
				_ = eventsFile.Close()
			}
		}()
		stdoutWriters = append(stdoutWriters, eventsFile)
	}
	cmd.Stdout = io.MultiWriter(stdoutWriters...)
	cmd.Stderr = io.MultiWriter(output, progressWriter)

	watchdog.Start(cancel)
	err = cmd.Run()
	watchdog.Stop()
	if eventsFile != nil {
		if closeErr := eventsFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close JSON events log for %s: %w", invocation.ActorName, closeErr)
		}
		eventsFile = nil
	}
	if callerErr := callerCtx.Err(); callerErr != nil {
		return ActorResult{}, callerErr
	}
	if policyErr := postflightSealedActorRuntime(sealed); policyErr != nil {
		return ActorResult{}, policyErr
	}
	if invocation.Policy.CaptureJSONEvents {
		if auditErr := auditCodexEvents(invocation.EventsPath); auditErr != nil {
			return ActorResult{}, auditErr
		}
	}
	if expiration, expired := watchdog.Expiration(); expired {
		return ActorResult{}, fmt.Errorf("actor `%s` failed: actor_watchdog_expired: no command progress observed for %s", expiration.ActorName, expiration.QuietWindow)
	}
	if err != nil {
		return ActorResult{}, fmt.Errorf("actor `%s` failed: %s", invocation.ActorName, strings.TrimSpace(redactActorOutput(output.String(), sealedActorRedactionSecrets(sealed)...)))
	}

	data, err := os.ReadFile(invocation.LastMessagePath)
	if err != nil {
		return ActorResult{}, fmt.Errorf("read %s output: %w", invocation.ActorName, err)
	}
	var response map[string]any
	if err := json.Unmarshal(data, &response); err != nil {
		return ActorResult{}, fmt.Errorf("decode %s actor response: %w", invocation.ActorName, err)
	}
	evidencePath, err := writeRuntimeEvidence(artifactDirectory, invocation, sealed, "passed")
	if err != nil {
		return ActorResult{}, err
	}
	return ActorResult{
		StructuredOutput:    response,
		LastMessagePath:     invocation.LastMessagePath,
		EventsPath:          invocation.EventsPath,
		ProvenancePath:      sealed.ProvenancePath,
		RuntimeEvidencePath: evidencePath,
	}, nil
}

func buildCodexCLIArgsForInvocation(invocation ActorInvocation) []string {
	return buildCodexCLIArgs(invocation.Policy, actorCommandSpecFromInvocation(invocation))
}

func validateActorInvocationPaths(artifactDirectory string, invocation ActorInvocation) error {
	requiredPaths := map[string]string{
		"output schema path": invocation.OutputSchemaPath,
		"last message path":  invocation.LastMessagePath,
	}
	if invocation.Policy.CaptureJSONEvents {
		requiredPaths["events path"] = invocation.EventsPath
	}
	for label, path := range requiredPaths {
		if strings.TrimSpace(path) == "" {
			return fmt.Errorf("%s is required for actor %s", label, invocation.ActorName)
		}
		if _, err := contracts.ResolvePathWithinRoot(artifactDirectory, path); err != nil {
			return fmt.Errorf("%s for actor %s escapes artifact directory: %w", label, invocation.ActorName, err)
		}
	}
	return nil
}

func writeRuntimeEvidence(artifactDirectory string, invocation ActorInvocation, sealed sealedActorRuntime, status string) (string, error) {
	runID := sanitizeActorRunID(invocation.ActorRunID)
	evidencePath := filepath.Join(artifactDirectory, "runs", runID+"-runtime-evidence.yaml")
	evidence := RuntimeEvidence{
		SchemaVersion: 1,
		BackendType:   "codex_cli",
		Actor:         invocation.ActorName,
		ActorRunID:    runID,
		Status:        status,
		RawEventLog:   relativePathFromArtifactDirectory(artifactDirectory, invocation.EventsPath),
		Provenance:    relativePathFromArtifactDirectory(artifactDirectory, sealed.ProvenancePath),
		Policy: RuntimeEvidencePolicy{
			Sandbox: invocation.Policy.Sandbox,
		},
		Redaction: RuntimeEvidenceRedaction{
			SecretValuesWritten: false,
		},
		PolicyViolations: []string{},
	}
	data, err := yaml.Marshal(evidence)
	if err != nil {
		return "", fmt.Errorf("marshal runtime evidence for %s: %w", invocation.ActorName, err)
	}
	if err := os.MkdirAll(filepath.Dir(evidencePath), 0o755); err != nil {
		return "", fmt.Errorf("create runtime evidence directory for %s: %w", invocation.ActorName, err)
	}
	if err := os.WriteFile(evidencePath, data, 0o644); err != nil {
		return "", fmt.Errorf("write runtime evidence for %s: %w", invocation.ActorName, err)
	}
	return evidencePath, nil
}

func relativePathFromArtifactDirectory(artifactDirectory string, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	relativePath, err := filepath.Rel(artifactDirectory, absolutePath)
	if err != nil {
		return ""
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) || filepath.IsAbs(relativePath) {
		return ""
	}
	return filepath.ToSlash(relativePath)
}
