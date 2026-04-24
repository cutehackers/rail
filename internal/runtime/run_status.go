package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const runStatusFileName = "run_status.yaml"

type RunStatus struct {
	Status              string   `yaml:"status"`
	Phase               string   `yaml:"phase"`
	CurrentActor        string   `yaml:"current_actor,omitempty"`
	LastSuccessfulActor string   `yaml:"last_successful_actor,omitempty"`
	InterruptionKind    string   `yaml:"interruption_kind,omitempty"`
	Message             string   `yaml:"message,omitempty"`
	ArtifactDir         string   `yaml:"artifact_dir"`
	Evidence            []string `yaml:"evidence"`
	NextStep            string   `yaml:"next_step,omitempty"`
	UpdatedAt           string   `yaml:"updated_at"`
}

func initialRunStatus(workflow Workflow, artifactDirectory string) RunStatus {
	currentActor := ""
	if len(workflow.Actors) > 0 {
		currentActor = workflow.Actors[0]
	}
	return RunStatus{
		Status:       "initialized",
		Phase:        "bootstrap",
		CurrentActor: currentActor,
		ArtifactDir:  artifactDirectory,
		Evidence: []string{
			"request.yaml",
			"workflow.json",
			"execution_plan.json",
			"state.json",
			workLedgerFileName,
			nextActionFileName,
		},
		NextStep: "Run rail execute --artifact " + artifactDirectory + " to continue the harness workflow.",
	}
}

func runStatusAfterActor(artifactDirectory string, actorName string, state State) RunStatus {
	return RunStatus{
		Status:              "in_progress",
		Phase:               "actor_execution",
		CurrentActor:        actorLabel(state.CurrentActor),
		LastSuccessfulActor: actorName,
		ArtifactDir:         artifactDirectory,
		Evidence: []string{
			artifactFileName(canonicalOutputForActor(actorName)),
			"state.json",
			workLedgerFileName,
			nextActionFileName,
		},
		NextStep: "Continue with next_action.yaml and the next actor brief.",
	}
}

func runStatusAfterEvaluation(artifactDirectory string, state State) RunStatus {
	status := "in_progress"
	phase := "evaluation_routing"
	nextStep := "Continue with next_action.yaml and the selected actor brief."
	evidence := []string{
		"evaluation_result.yaml",
		"execution_report.yaml",
		"supervisor_trace.md",
		"state.json",
		workLedgerFileName,
		nextActionFileName,
	}
	if shouldTerminate(state) {
		status = state.Status
		phase = "terminal"
		nextStep = "Read terminal_summary.md before reporting the result to the user."
		evidence = append(evidence, "terminal_summary.md")
	}
	return RunStatus{
		Status:              status,
		Phase:               phase,
		CurrentActor:        actorLabel(state.CurrentActor),
		LastSuccessfulActor: "evaluator",
		ArtifactDir:         artifactDirectory,
		Evidence:            evidence,
		NextStep:            nextStep,
	}
}

func interruptedRunStatus(artifactDirectory string, phase string, actorName string, state State, err error) RunStatus {
	return RunStatus{
		Status:              "interrupted",
		Phase:               phase,
		CurrentActor:        fallbackString(actorName, actorLabel(state.CurrentActor)),
		LastSuccessfulActor: lastSuccessfulActor(state),
		InterruptionKind:    classifyInterruption(err),
		Message:             strings.TrimSpace(err.Error()),
		ArtifactDir:         artifactDirectory,
		Evidence: []string{
			"state.json",
			workLedgerFileName,
			nextActionFileName,
			"runs/",
		},
		NextStep: "Inspect run_status.yaml, work_ledger.md, next_action.yaml, and runs/ logs; fix the blocker, then rerun rail execute --artifact " + artifactDirectory + ".",
	}
}

func writeRunStatus(artifactDirectory string, status RunStatus) error {
	status.ArtifactDir = artifactDirectory
	status.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	return writeYAML(filepath.Join(artifactDirectory, runStatusFileName), status)
}

func ReadRunStatus(artifactDirectory string) (RunStatus, error) {
	data, err := os.ReadFile(filepath.Join(artifactDirectory, runStatusFileName))
	if err != nil {
		if os.IsNotExist(err) {
			return synthesizeRunStatusFromState(artifactDirectory)
		}
		return RunStatus{}, fmt.Errorf("read %s: %w", runStatusFileName, err)
	}
	var status RunStatus
	if err := yaml.Unmarshal(data, &status); err != nil {
		return RunStatus{}, fmt.Errorf("decode %s: %w", runStatusFileName, err)
	}
	return status, nil
}

func ReadRunStatusForArtifact(projectRoot string, artifactPath string) (RunStatus, error) {
	router, err := NewRouter(projectRoot)
	if err != nil {
		return RunStatus{}, err
	}
	artifactDirectory, err := router.resolveArtifactDirectory(artifactPath)
	if err != nil {
		return RunStatus{}, err
	}
	return ReadRunStatus(artifactDirectory)
}

func synthesizeRunStatusFromState(artifactDirectory string) (RunStatus, error) {
	statePath := filepath.Join(artifactDirectory, "state.json")
	state, err := readState(statePath)
	if err != nil {
		return RunStatus{}, fmt.Errorf("read %s or synthesize status from state.json: %w", runStatusFileName, err)
	}

	status := state.Status
	phase := "actor_execution"
	nextStep := "Continue with next_action.yaml and the selected actor brief."
	if state.Status == "initialized" && len(state.CompletedActors) == 0 {
		phase = "bootstrap"
		nextStep = "Run rail execute --artifact " + artifactDirectory + " to continue the harness workflow."
	} else if shouldTerminate(state) {
		phase = "terminal"
		nextStep = "Read terminal_summary.md before reporting the result to the user."
	}

	evidence := []string{"state.json", workLedgerFileName, nextActionFileName}
	if shouldTerminate(state) || len(state.ActionHistory) > 0 {
		evidence = append(evidence, "evaluation_result.yaml", "execution_report.yaml", "supervisor_trace.md")
	}
	if _, err := os.Stat(filepath.Join(artifactDirectory, "terminal_summary.md")); err == nil {
		evidence = append(evidence, "terminal_summary.md")
	}

	updatedAt := ""
	if info, statErr := os.Stat(statePath); statErr == nil {
		updatedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	return RunStatus{
		Status:              status,
		Phase:               phase,
		CurrentActor:        actorLabel(state.CurrentActor),
		LastSuccessfulActor: lastSuccessfulActor(state),
		ArtifactDir:         artifactDirectory,
		Evidence:            evidence,
		NextStep:            nextStep,
		UpdatedAt:           updatedAt,
	}, nil
}

func FormatRunStatusSummary(status RunStatus) string {
	var builder strings.Builder
	builder.WriteString("Harness status\n")
	builder.WriteString("- status: " + fallbackString(status.Status, "unknown") + "\n")
	builder.WriteString("- phase: " + fallbackString(status.Phase, "unknown") + "\n")
	if status.CurrentActor != "" {
		builder.WriteString("- current actor: " + status.CurrentActor + "\n")
	}
	if status.LastSuccessfulActor != "" {
		builder.WriteString("- last successful actor: " + status.LastSuccessfulActor + "\n")
	}
	if status.InterruptionKind != "" {
		builder.WriteString("- interruption: " + status.InterruptionKind + "\n")
	}
	if status.Message != "" {
		builder.WriteString("- message: " + status.Message + "\n")
	}
	builder.WriteString("- artifact: " + fallbackString(status.ArtifactDir, "unknown") + "\n")
	if len(status.Evidence) > 0 {
		builder.WriteString("- evidence: " + strings.Join(status.Evidence, ", ") + "\n")
	}
	if status.NextStep != "" {
		builder.WriteString("- next step: " + status.NextStep + "\n")
	}
	if status.UpdatedAt != "" {
		builder.WriteString("- updated at: " + status.UpdatedAt + "\n")
	}
	return builder.String()
}

func classifyInterruption(err error) string {
	if err == nil {
		return "unknown"
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "actor_watchdog_expired"):
		return "actor_watchdog_expired"
	case strings.Contains(message, "backend_policy_violation"):
		return "backend_policy_violation"
	case strings.Contains(message, "unknown actor in state"):
		return "execution_error"
	case strings.Contains(message, "validate"):
		return "artifact_validation_failed"
	case strings.Contains(message, "codex") || strings.Contains(message, "actor"):
		return "actor_failed"
	default:
		return "execution_error"
	}
}

func lastSuccessfulActor(state State) string {
	if len(state.CompletedActors) == 0 {
		return ""
	}
	return state.CompletedActors[len(state.CompletedActors)-1]
}
