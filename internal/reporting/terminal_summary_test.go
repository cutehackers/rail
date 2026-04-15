package reporting

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"rail/internal/runtime"
)

func TestExecuteProducesTerminalSummaryAndState(t *testing.T) {
	projectRoot, requestPath := prepareReportingSmokeProject(t)

	runner, err := runtime.NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	artifactPath, err := runner.Run(requestPath, "reporting-smoke")
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, err := runner.Execute(artifactPath); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	state, err := LoadState(filepath.Join(artifactPath, "state.json"))
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}
	if state.Status != "passed" {
		t.Fatalf("unexpected status: got %q want %q", state.Status, "passed")
	}
	if state.CurrentActor != nil {
		t.Fatalf("expected terminal state to clear current actor, got %v", state.CurrentActor)
	}

	summary, err := ReadTerminalSummary(filepath.Join(artifactPath, "terminal_summary.md"))
	if err != nil {
		t.Fatalf("ReadTerminalSummary returned error: %v", err)
	}
	for _, fragment := range []string{
		"# Terminal Outcome",
		"- status: `passed`",
		"## Recommended Next Step",
	} {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("expected terminal summary to contain %q, got:\n%s", fragment, summary)
		}
	}
}

func TestWriteStatePreservesRuntimeMetadata(t *testing.T) {
	currentActor := "evaluator"
	lastDecision := "revise"
	lastContextRefreshTrigger := "reason_codes"
	lastContextRefreshReasonFamily := "context"
	lastInterventionTriggerCategory := "validation"
	pendingContextRefreshTrigger := "next_action"
	pendingContextRefreshReasonFamily := "context"

	original := State{
		TaskID:                             "task-123",
		TaskFamily:                         "test_repair",
		TaskFamilySource:                   "task_type",
		Status:                             "rebuilding_context",
		CurrentActor:                       &currentActor,
		CompletedActors:                    []string{"planner", "context_builder"},
		GeneratorRetriesRemaining:          1,
		ContextRebuildsRemaining:           2,
		ValidationTighteningsRemaining:     3,
		LastDecision:                       &lastDecision,
		LastReasonCodes:                    []string{"context_missing"},
		ActionHistory:                      []string{"rebuild_context"},
		GeneratorRevisionsUsed:             4,
		ContextRefreshCount:                5,
		LastContextRefreshTrigger:          &lastContextRefreshTrigger,
		LastContextRefreshReasonFamily:     &lastContextRefreshReasonFamily,
		LastInterventionTriggerReasonCodes: []string{"validation_scope_missing"},
		LastInterventionTriggerCategory:    &lastInterventionTriggerCategory,
		PendingContextRefreshTrigger:       &pendingContextRefreshTrigger,
		PendingContextRefreshReasonFamily:  &pendingContextRefreshReasonFamily,
		ValidationTighteningsUsed:          6,
	}

	path := filepath.Join(t.TempDir(), "state.json")
	if err := WriteState(path, original); err != nil {
		t.Fatalf("WriteState returned error: %v", err)
	}

	reloaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState returned error: %v", err)
	}

	if reloaded.TaskFamily != original.TaskFamily {
		t.Fatalf("expected task family to round-trip, got %q want %q", reloaded.TaskFamily, original.TaskFamily)
	}
	if reloaded.TaskFamilySource != original.TaskFamilySource {
		t.Fatalf("expected task family source to round-trip, got %q want %q", reloaded.TaskFamilySource, original.TaskFamilySource)
	}
	if reloaded.LastContextRefreshTrigger == nil || *reloaded.LastContextRefreshTrigger != *original.LastContextRefreshTrigger {
		t.Fatalf("expected last context refresh trigger to round-trip, got %v want %v", reloaded.LastContextRefreshTrigger, original.LastContextRefreshTrigger)
	}
	if reloaded.LastContextRefreshReasonFamily == nil || *reloaded.LastContextRefreshReasonFamily != *original.LastContextRefreshReasonFamily {
		t.Fatalf("expected last context refresh reason family to round-trip, got %v want %v", reloaded.LastContextRefreshReasonFamily, original.LastContextRefreshReasonFamily)
	}
	if !slices.Equal(reloaded.LastInterventionTriggerReasonCodes, original.LastInterventionTriggerReasonCodes) {
		t.Fatalf("expected last intervention trigger reason codes to round-trip, got %v want %v", reloaded.LastInterventionTriggerReasonCodes, original.LastInterventionTriggerReasonCodes)
	}
	if reloaded.LastInterventionTriggerCategory == nil || *reloaded.LastInterventionTriggerCategory != *original.LastInterventionTriggerCategory {
		t.Fatalf("expected last intervention trigger category to round-trip, got %v want %v", reloaded.LastInterventionTriggerCategory, original.LastInterventionTriggerCategory)
	}
	if reloaded.PendingContextRefreshTrigger == nil || *reloaded.PendingContextRefreshTrigger != *original.PendingContextRefreshTrigger {
		t.Fatalf("expected pending context refresh trigger to round-trip, got %v want %v", reloaded.PendingContextRefreshTrigger, original.PendingContextRefreshTrigger)
	}
	if reloaded.PendingContextRefreshReasonFamily == nil || *reloaded.PendingContextRefreshReasonFamily != *original.PendingContextRefreshReasonFamily {
		t.Fatalf("expected pending context refresh reason family to round-trip, got %v want %v", reloaded.PendingContextRefreshReasonFamily, original.PendingContextRefreshReasonFamily)
	}
}

func prepareReportingSmokeProject(t *testing.T) (string, string) {
	t.Helper()

	projectRoot := t.TempDir()
	for _, relPath := range []string{
		filepath.Join(".harness", "requests"),
		filepath.Join(".harness", "artifacts"),
		"test",
	} {
		if err := os.MkdirAll(filepath.Join(projectRoot, relPath), 0o755); err != nil {
			t.Fatalf("failed to create %q: %v", relPath, err)
		}
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, "pubspec.yaml"), []byte("name: smoke_project\n"), 0o644); err != nil {
		t.Fatalf("failed to write pubspec.yaml: %v", err)
	}

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	requestBody, err := os.ReadFile(filepath.Join(repoRoot, ".harness", "requests", "rail-bootstrap-smoke.yaml"))
	if err != nil {
		t.Fatalf("failed to read smoke request fixture: %v", err)
	}
	requestPath := filepath.Join(projectRoot, ".harness", "requests", "rail-bootstrap-smoke.yaml")
	if err := os.WriteFile(requestPath, requestBody, 0o644); err != nil {
		t.Fatalf("failed to write smoke request fixture: %v", err)
	}

	return projectRoot, requestPath
}
