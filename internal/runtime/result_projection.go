package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rail/internal/contracts"
)

const harnessResultSchemaVersion = 1

type HarnessResult struct {
	SchemaVersion       int               `json:"schema_version"`
	ArtifactDir         string            `json:"artifact_dir"`
	Status              string            `json:"status"`
	RawStatus           string            `json:"raw_status,omitempty"`
	Phase               string            `json:"phase"`
	CurrentActor        string            `json:"current_actor,omitempty"`
	LastSuccessfulActor string            `json:"last_successful_actor,omitempty"`
	InterruptionKind    string            `json:"interruption_kind,omitempty"`
	Message             string            `json:"message,omitempty"`
	Terminal            bool              `json:"terminal"`
	HumanSummary        string            `json:"human_summary"`
	RecommendedNextStep string            `json:"recommended_next_step,omitempty"`
	Evidence            []string          `json:"evidence"`
	SourceArtifacts     map[string]string `json:"source_artifacts"`
	UpdatedAt           string            `json:"updated_at,omitempty"`
}

func ProjectHarnessResultForArtifact(projectRoot, artifactPath string) (HarnessResult, error) {
	router, err := NewRouter(projectRoot)
	if err != nil {
		return HarnessResult{}, err
	}
	artifactDirectory, err := router.resolveArtifactDirectory(artifactPath)
	if err != nil {
		return HarnessResult{}, err
	}

	runStatusExists, err := artifactRefExistsChecked(artifactDirectory, runStatusFileName)
	if err != nil {
		return HarnessResult{}, err
	}
	runStatus, err := ReadRunStatus(artifactDirectory)
	if err != nil {
		return HarnessResult{}, err
	}

	rawStatus := strings.TrimSpace(runStatus.Status)
	projectedStatus := projectRunStatus(rawStatus, runStatus.InterruptionKind)
	terminal := isTerminalHarnessResult(rawStatus)
	evidence, sources := projectResultEvidence(artifactDirectory, runStatus, terminal, runStatusExists)

	result := HarnessResult{
		SchemaVersion:       harnessResultSchemaVersion,
		ArtifactDir:         artifactDirectory,
		Status:              projectedStatus,
		Phase:               fallbackString(strings.TrimSpace(runStatus.Phase), "unknown"),
		CurrentActor:        strings.TrimSpace(runStatus.CurrentActor),
		LastSuccessfulActor: strings.TrimSpace(runStatus.LastSuccessfulActor),
		InterruptionKind:    strings.TrimSpace(runStatus.InterruptionKind),
		Message:             compactResultText(runStatus.Message),
		Terminal:            terminal,
		RecommendedNextStep: recommendedResultNextStep(runStatus, artifactDirectory, projectedStatus, rawStatus, terminal),
		Evidence:            evidence,
		SourceArtifacts:     sources,
		UpdatedAt:           strings.TrimSpace(runStatus.UpdatedAt),
	}
	if rawStatus != "" && rawStatus != projectedStatus {
		result.RawStatus = rawStatus
	}
	result.HumanSummary = humanResultSummary(result)
	return result, nil
}

func ProjectLatestHarnessResult(projectRoot string) (HarnessResult, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return HarnessResult{}, fmt.Errorf("resolve project root: %w", err)
	}

	artifactsRoot := filepath.Join(root, ".harness", "artifacts")
	candidates, err := filepath.Glob(filepath.Join(artifactsRoot, "*", runStatusFileName))
	if err != nil {
		return HarnessResult{}, fmt.Errorf("scan run_status.yaml candidates: %w", err)
	}

	var selected latestHarnessResultCandidate
	for _, candidate := range candidates {
		artifactDirectory, err := contracts.ResolvePathWithinRoot(root, filepath.Dir(candidate))
		if err != nil {
			continue
		}
		runStatusPath, err := contracts.ResolvePathWithinRoot(root, filepath.Join(artifactDirectory, runStatusFileName))
		if err != nil {
			continue
		}
		runStatusInfo, err := os.Stat(runStatusPath)
		if err != nil {
			continue
		}
		runStatus, err := ReadRunStatus(artifactDirectory)
		if err != nil {
			continue
		}
		updatedAt, err := parseResultUpdatedAt(runStatus.UpdatedAt)
		if err != nil {
			continue
		}

		current := latestHarnessResultCandidate{
			artifactDirectory: artifactDirectory,
			updatedAt:         updatedAt,
			runStatusMTime:    runStatusInfo.ModTime(),
		}
		if current.preferredOver(selected) {
			selected = current
		}
	}

	if selected.artifactDirectory == "" {
		return HarnessResult{}, fmt.Errorf("no valid run_status.yaml candidates found under %s", artifactsRoot)
	}
	return ProjectHarnessResultForArtifact(root, selected.artifactDirectory)
}

type latestHarnessResultCandidate struct {
	artifactDirectory string
	updatedAt         time.Time
	runStatusMTime    time.Time
}

func (candidate latestHarnessResultCandidate) preferredOver(selected latestHarnessResultCandidate) bool {
	if selected.artifactDirectory == "" {
		return true
	}
	if !candidate.updatedAt.Equal(selected.updatedAt) {
		return candidate.updatedAt.After(selected.updatedAt)
	}
	if !candidate.runStatusMTime.Equal(selected.runStatusMTime) {
		return candidate.runStatusMTime.After(selected.runStatusMTime)
	}
	return candidate.artifactDirectory > selected.artifactDirectory
}

func FormatHarnessResult(result HarnessResult) string {
	var builder strings.Builder
	builder.WriteString("Rail result: " + fallbackString(result.Status, "unknown") + "\n\n")
	builder.WriteString("Phase: " + fallbackString(result.Phase, "unknown") + "\n")
	if result.CurrentActor != "" {
		builder.WriteString("Current actor: " + result.CurrentActor + "\n")
	}
	if result.LastSuccessfulActor != "" {
		builder.WriteString("Last successful actor: " + result.LastSuccessfulActor + "\n")
	}
	if result.InterruptionKind != "" {
		builder.WriteString("Reason: " + result.InterruptionKind + "\n")
	}
	if result.HumanSummary != "" {
		builder.WriteString("\nWhat happened:\n" + result.HumanSummary + "\n")
	}
	if result.RecommendedNextStep != "" {
		builder.WriteString("\nNext step:\n" + result.RecommendedNextStep + "\n")
	}
	if len(result.Evidence) > 0 {
		builder.WriteString("\nEvidence: " + strings.Join(result.Evidence, ", ") + "\n")
	}
	return builder.String()
}

func parseResultUpdatedAt(value string) (time.Time, error) {
	updatedAt := strings.TrimSpace(value)
	if updatedAt == "" {
		return time.Time{}, fmt.Errorf("updated_at is required")
	}
	if parsed, err := time.Parse(time.RFC3339Nano, updatedAt); err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, updatedAt)
}

func projectRunStatus(rawStatus string, interruptionKind string) string {
	switch rawStatus {
	case "blocked_environment", "split_required":
		return "blocked"
	case "revise_exhausted", "evolution_exhausted":
		return "rejected"
	case "passed", "rejected", "retrying", "interrupted":
		return rawStatus
	case "initialized", "in_progress":
		if strings.TrimSpace(interruptionKind) != "" {
			return "interrupted"
		}
		return rawStatus
	}
	if strings.TrimSpace(interruptionKind) != "" {
		return "interrupted"
	}
	if rawStatus != "" {
		return rawStatus
	}
	return "unknown"
}

func isTerminalHarnessResult(rawStatus string) bool {
	return isTerminalRunStatus(RunStatus{Status: rawStatus})
}

func projectResultEvidence(artifactDirectory string, runStatus RunStatus, terminal bool, runStatusExists bool) ([]string, map[string]string) {
	evidence := []string{}
	seen := map[string]struct{}{}
	sources := map[string]string{}

	addEvidence := func(ref string) {
		normalized := normalizeEvidenceRef(ref)
		if normalized == "" {
			return
		}
		if _, ok := seen[normalized]; ok {
			return
		}
		seen[normalized] = struct{}{}
		evidence = append(evidence, normalized)
	}
	addSource := func(key string, ref string) {
		normalized := normalizeEvidenceRef(ref)
		if key == "" || normalized == "" {
			return
		}
		sources[key] = normalized
		addEvidence(normalized)
	}

	if runStatusExists {
		addSource("run_status", runStatusFileName)
	} else {
		addSource("state", "state.json")
	}
	for _, ref := range runStatus.Evidence {
		if normalizeEvidenceRef(ref) == "terminal_summary.md" && !(terminal && artifactRefExists(artifactDirectory, ref)) {
			continue
		}
		addEvidence(ref)
		if key := sourceArtifactKey(ref); key != "" && normalizeEvidenceRef(ref) != "terminal_summary.md" {
			addSource(key, ref)
		}
	}

	if terminal {
		for _, ref := range []string{"evaluation_result.yaml", "execution_report.yaml", "supervisor_trace.md"} {
			if artifactRefExists(artifactDirectory, ref) {
				addSource(sourceArtifactKey(ref), ref)
			}
		}
		if artifactRefExists(artifactDirectory, "terminal_summary.md") {
			addSource("terminal_summary", "terminal_summary.md")
		}
	}

	return evidence, sources
}

func artifactRefExistsChecked(artifactDirectory string, ref string) (bool, error) {
	normalized := normalizeEvidenceRef(ref)
	if normalized == "" {
		return false, nil
	}
	_, err := os.Stat(filepath.Join(artifactDirectory, filepath.FromSlash(strings.TrimSuffix(normalized, "/"))))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func normalizeEvidenceRef(ref string) string {
	normalized := filepath.ToSlash(strings.TrimSpace(ref))
	normalized = strings.TrimPrefix(normalized, "./")
	return normalized
}

func sourceArtifactKey(ref string) string {
	switch normalizeEvidenceRef(ref) {
	case runStatusFileName:
		return "run_status"
	case "terminal_summary.md":
		return "terminal_summary"
	case "evaluation_result.yaml":
		return "evaluation_result"
	case "execution_report.yaml":
		return "execution_report"
	case "supervisor_trace.md":
		return "supervisor_trace"
	case "state.json":
		return "state"
	case workLedgerFileName:
		return "work_ledger"
	case nextActionFileName:
		return "next_action"
	case "runs/":
		return "runs"
	default:
		return ""
	}
}

func artifactRefExists(artifactDirectory string, ref string) bool {
	exists, err := artifactRefExistsChecked(artifactDirectory, ref)
	return err == nil && exists
}

func recommendedResultNextStep(runStatus RunStatus, artifactDirectory string, projectedStatus string, rawStatus string, terminal bool) string {
	if nextStep := compactResultText(runStatus.NextStep); nextStep != "" {
		return nextStep
	}
	if terminal {
		if rawStatus != "" {
			return terminalOutcomeNextStep(rawStatus)
		}
		return "Inspect terminal_summary.md and supervisor_trace.md before deciding the next action."
	}
	switch projectedStatus {
	case "initialized":
		return "Run rail supervise --artifact " + artifactDirectory + " to continue the harness workflow."
	case "interrupted":
		return "Inspect run_status.yaml and runs/ evidence, fix the blocker, then rerun rail supervise."
	case "in_progress", "retrying":
		return "Continue from next_action.yaml with the selected actor brief."
	default:
		return ""
	}
}

func humanResultSummary(result HarnessResult) string {
	if result.Terminal && result.RawStatus != "" {
		return terminalOutcomeSummary(result.RawStatus)
	}
	if result.Terminal {
		return terminalOutcomeSummary(result.Status)
	}

	phase := fallbackString(result.Phase, "unknown")
	actor := strings.TrimSpace(result.CurrentActor)
	switch result.Status {
	case "initialized":
		if actor != "" {
			return fmt.Sprintf("Rail initialized the artifact and is ready to run %s.", actor)
		}
		return "Rail initialized the artifact and is ready to continue."
	case "in_progress":
		if actor != "" {
			return fmt.Sprintf("Rail is in progress during %s with %s as the current actor.", phase, actor)
		}
		return fmt.Sprintf("Rail is in progress during %s.", phase)
	case "retrying":
		if actor != "" {
			return fmt.Sprintf("Rail is retrying during %s with %s as the current actor.", phase, actor)
		}
		return fmt.Sprintf("Rail is retrying during %s.", phase)
	case "interrupted":
		if actor != "" && result.InterruptionKind != "" {
			return fmt.Sprintf("Rail stopped during %s while handling %s (%s).", phase, actor, result.InterruptionKind)
		}
		if actor != "" {
			return fmt.Sprintf("Rail stopped during %s while handling %s.", phase, actor)
		}
		if result.InterruptionKind != "" {
			return fmt.Sprintf("Rail stopped during %s (%s).", phase, result.InterruptionKind)
		}
		return fmt.Sprintf("Rail stopped during %s before reaching a terminal outcome.", phase)
	case "blocked":
		return fmt.Sprintf("Rail is blocked during %s and needs operator action before continuing.", phase)
	case "failed":
		return fmt.Sprintf("Rail failed during %s before a valid terminal result was accepted.", phase)
	default:
		return fmt.Sprintf("Rail reported status %s during %s.", fallbackString(result.Status, "unknown"), phase)
	}
}

func compactResultText(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	const maxLength = 500
	runes := []rune(value)
	if len(runes) <= maxLength {
		return value
	}
	return strings.TrimSpace(string(runes[:maxLength])) + "..."
}
