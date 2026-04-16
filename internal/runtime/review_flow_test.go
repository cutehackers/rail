package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rail/internal/contracts"
	"rail/internal/project"
)

func TestRunnerInitUserOutcomeFeedbackWritesDraftFromArtifact(t *testing.T) {
	projectRoot := prepareReviewFlowProject(t)
	artifactPath := copyStandardRouteFixtureForRuntime(t, projectRoot, "tighten_validation")

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	outputPath, err := runner.InitUserOutcomeFeedback(artifactPath, "")
	if err != nil {
		t.Fatalf("InitUserOutcomeFeedback returned error: %v", err)
	}

	if !strings.Contains(filepath.ToSlash(outputPath), ".harness/learning/feedback/") {
		t.Fatalf("unexpected output path: %s", outputPath)
	}
	validator, err := contracts.NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}
	value, err := validator.ValidateArtifactFile(filepath.ToSlash(mustRel(projectRoot, outputPath)), "user_outcome_feedback")
	if err != nil {
		t.Fatalf("draft did not validate: %v", err)
	}
	if got := stringValue(value["feedback_classification"]); got != "unresolved" {
		t.Fatalf("unexpected feedback_classification: got %q want unresolved", got)
	}
}

func TestRunnerInitLearningReviewWritesDraftFromQualityCandidate(t *testing.T) {
	projectRoot := prepareReviewFlowProject(t)
	candidateFile := writeQualityCandidateFixture(t, projectRoot, "quality-init", "")

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	candidateRef := filepath.ToSlash(mustRel(projectRoot, candidateFile))
	outputPath, err := runner.InitLearningReview(candidateRef, "")
	if err != nil {
		t.Fatalf("InitLearningReview returned error: %v", err)
	}

	validator, err := contracts.NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}
	value, err := validator.ValidateArtifactFile(filepath.ToSlash(mustRel(projectRoot, outputPath)), "learning_review_decision")
	if err != nil {
		t.Fatalf("draft did not validate: %v", err)
	}
	if got := stringValue(value["candidate_ref"]); got != candidateRef {
		t.Fatalf("unexpected candidate_ref: got %q want %q", got, candidateRef)
	}
	if got := stringValue(value["reviewer_decision"]); got != "hold" {
		t.Fatalf("unexpected reviewer_decision: got %q want hold", got)
	}
}

func TestRunnerInitHardeningReviewWritesDraftFromCandidate(t *testing.T) {
	projectRoot := prepareReviewFlowProject(t)
	candidateFile := writeHardeningCandidateFixture(t, projectRoot, "hardening-init")

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	candidateRef := filepath.ToSlash(mustRel(projectRoot, candidateFile))
	outputPath, err := runner.InitHardeningReview(candidateRef, "")
	if err != nil {
		t.Fatalf("InitHardeningReview returned error: %v", err)
	}

	validator, err := contracts.NewValidator(projectRoot)
	if err != nil {
		t.Fatalf("NewValidator returned error: %v", err)
	}
	value, err := validator.ValidateArtifactFile(filepath.ToSlash(mustRel(projectRoot, outputPath)), "hardening_review_decision")
	if err != nil {
		t.Fatalf("draft did not validate: %v", err)
	}
	if got := stringValue(value["hardening_candidate_ref"]); got != candidateRef {
		t.Fatalf("unexpected hardening_candidate_ref: got %q want %q", got, candidateRef)
	}
	if got := stringValue(value["reviewer_decision"]); got != "hold" {
		t.Fatalf("unexpected reviewer_decision: got %q want hold", got)
	}
}

func TestRunnerApplyUserOutcomeFeedbackUpdatesMatchingQualityCandidate(t *testing.T) {
	projectRoot := prepareReviewFlowProject(t)
	candidateFile := writeQualityCandidateFixture(t, projectRoot, "apply-feedback", "")
	candidateRef := filepath.ToSlash(mustRel(projectRoot, candidateFile))
	feedbackFile := writeUserOutcomeFeedbackFixture(t, projectRoot, "feedback.yaml", candidateRef, "accepted")
	feedbackRef := filepath.ToSlash(mustRel(projectRoot, feedbackFile))

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	if _, err := runner.ApplyUserOutcomeFeedback(feedbackRef); err != nil {
		t.Fatalf("ApplyUserOutcomeFeedback returned error: %v", err)
	}

	value, err := contracts.ReadYAMLFile(candidateFile)
	if err != nil {
		t.Fatalf("failed to read updated candidate: %v", err)
	}
	candidate, err := contracts.AsMap(value, candidateFile)
	if err != nil {
		t.Fatalf("failed to decode updated candidate: %v", err)
	}
	userOutcome := mapValue(candidate["user_outcome_signal"])
	refs := stringList(userOutcome["supporting_feedback_refs"])
	if len(refs) != 1 || refs[0] != feedbackRef {
		t.Fatalf("unexpected supporting_feedback_refs: %v", refs)
	}
}

func TestRunnerApplyLearningReviewPersistsDecisionAndApprovedMemory(t *testing.T) {
	projectRoot := prepareReviewFlowProject(t)
	candidateFile := writeQualityCandidateFixture(t, projectRoot, "apply-learning", "")
	candidateRef := filepath.ToSlash(mustRel(projectRoot, candidateFile))
	feedbackFile := writeUserOutcomeFeedbackFixture(t, projectRoot, "feedback-promote.yaml", candidateRef, "accepted")
	feedbackRef := filepath.ToSlash(mustRel(projectRoot, feedbackFile))
	decisionFile := writeLearningReviewFixture(t, projectRoot, candidateRef, "promote", []string{feedbackRef}, ".harness/learning/approved/bug_fix.yaml")
	decisionRef := filepath.ToSlash(mustRel(projectRoot, decisionFile))

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	if _, err := runner.ApplyLearningReview(decisionRef); err != nil {
		t.Fatalf("ApplyLearningReview returned error: %v", err)
	}

	storedDecision := filepath.Join(projectRoot, ".harness", "learning", "learning_review_decisions", sanitizeLearningStoreComponent(candidateRef)+".yaml")
	if _, err := os.Stat(storedDecision); err != nil {
		t.Fatalf("expected persisted learning review decision: %v", err)
	}
	approvedPath := filepath.Join(projectRoot, ".harness", "learning", "approved", "bug_fix.yaml")
	if _, err := os.Stat(approvedPath); err != nil {
		t.Fatalf("expected canonical approved memory: %v", err)
	}
}

func TestRunnerApplyHardeningReviewPersistsDecision(t *testing.T) {
	projectRoot := prepareReviewFlowProject(t)
	candidateFile := writeHardeningCandidateFixture(t, projectRoot, "apply-hardening")
	candidateRef := filepath.ToSlash(mustRel(projectRoot, candidateFile))
	decisionFile := writeHardeningReviewFixture(t, projectRoot, candidateRef)
	decisionRef := filepath.ToSlash(mustRel(projectRoot, decisionFile))

	runner, err := NewRunner(projectRoot)
	if err != nil {
		t.Fatalf("NewRunner returned error: %v", err)
	}

	if _, err := runner.ApplyHardeningReview(decisionRef); err != nil {
		t.Fatalf("ApplyHardeningReview returned error: %v", err)
	}

	storedDecision := filepath.Join(projectRoot, ".harness", "learning", "hardening_review_decisions", sanitizeLearningStoreComponent(candidateRef)+".yaml")
	if _, err := os.Stat(storedDecision); err != nil {
		t.Fatalf("expected persisted hardening review decision: %v", err)
	}
}

func prepareReviewFlowProject(t *testing.T) string {
	t.Helper()
	projectRoot := t.TempDir()
	if err := project.Init(projectRoot); err != nil {
		t.Fatalf("project.Init returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectRoot, ".git"), []byte("gitdir: test\n"), 0o644); err != nil {
		t.Fatalf("failed to write git marker: %v", err)
	}
	return projectRoot
}

func copyStandardRouteFixtureForRuntime(t *testing.T, projectRoot, fixtureName string) string {
	t.Helper()
	sourceRoot := filepath.Join(testRepoRoot(t), "test", "fixtures", "standard_route", fixtureName)
	targetRoot := filepath.Join(projectRoot, ".harness", "artifacts", fixtureName)
	if err := filepath.Walk(sourceRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(sourceRoot, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(targetRoot, relative)
		if info.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, info.Mode())
	}); err != nil {
		t.Fatalf("failed to copy standard-route fixture %q: %v", fixtureName, err)
	}
	return targetRoot
}

func writeQualityCandidateFixture(t *testing.T, projectRoot, runName, candidateIdentifier string) string {
	t.Helper()
	if candidateIdentifier == "" {
		candidateIdentifier = "quality/candidate@1"
	}
	artifactDir := filepath.Join(projectRoot, ".harness", "artifacts", runName)
	candidateDir := filepath.Join(artifactDir, "quality_learning_candidates")
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		t.Fatalf("failed to create quality candidate directory: %v", err)
	}
	runRef := filepath.ToSlash(mustRel(projectRoot, artifactDir))
	candidateFile := filepath.Join(candidateDir, "candidate.yaml")
	candidate := map[string]any{
		"originating_run_artifact_identity": map[string]any{
			"run_ref":      runRef,
			"artifact_ref": filepath.ToSlash(filepath.Join(runRef, "evaluation_result.yaml")),
		},
		"candidate_identifier":     candidateIdentifier,
		"task_family":              "bug_fix",
		"task_family_source":       "task_type",
		"quality_outcome_summary":  "Validation evidence suggests the fix should be reusable.",
		"user_outcome_signal":      map[string]any{"status": "provisional", "summary": "Awaiting explicit user confirmation.", "supporting_feedback_refs": []string{}},
		"effective_context_signal": map[string]any{"summary": "Context stayed stable.", "helped_context_factors": []string{}, "failed_context_factors": []string{}, "neutral_context_factors": []string{"Baseline repo context was sufficient."}, "evidence_refs": []string{filepath.ToSlash(filepath.Join(runRef, "execution_report.yaml"))}, "context_factor_refs": []string{"state.contextRefreshCount"}},
		"effective_validation_signal": map[string]any{
			"summary": "Validation evidence is supportive.",
			"materially_supporting_validation_evidence": []string{"Static analysis passed."},
			"failed_to_support_validation_evidence":     []string{},
			"contradicting_validation_evidence":         []string{},
			"evidence_refs":                             []string{filepath.ToSlash(filepath.Join(runRef, "execution_report.yaml"))},
			"validation_step_refs":                      []string{"execution_report.analyze"},
		},
		"evaluator_support_signal": map[string]any{
			"quality_confidence":                0.6,
			"final_reason_codes":                []string{"requirements_coverage_resolved"},
			"validation_sufficiency_assessment": "sufficient",
			"terminal_outcome_class":            "passed",
			"supporting_evaluator_notes":        []string{"Evaluator pass was bounded and explicit."},
		},
		"candidate_claim":          "This change looks reusable for the same task family.",
		"supporting_evidence_refs": []string{filepath.ToSlash(filepath.Join(runRef, "evaluation_result.yaml"))},
		"guardrail_cost":           map[string]any{"summary": "No excessive intervention cost was recorded.", "intervention_count": 1, "intervention_refs": []string{filepath.ToSlash(filepath.Join(runRef, "state.json"))}},
		"runtime_recommendation":   "hold",
	}
	if err := writeYAML(candidateFile, candidate); err != nil {
		t.Fatalf("failed to write quality candidate fixture: %v", err)
	}
	return candidateFile
}

func writeHardeningCandidateFixture(t *testing.T, projectRoot, runName string) string {
	t.Helper()
	artifactDir := filepath.Join(projectRoot, ".harness", "artifacts", runName)
	candidateDir := filepath.Join(artifactDir, "hardening_candidates")
	if err := os.MkdirAll(candidateDir, 0o755); err != nil {
		t.Fatalf("failed to create hardening candidate directory: %v", err)
	}
	runRef := filepath.ToSlash(mustRel(projectRoot, artifactDir))
	candidateFile := filepath.Join(candidateDir, "candidate.yaml")
	candidate := map[string]any{
		"originating_run_artifact_identity": map[string]any{
			"run_ref":      runRef,
			"artifact_ref": filepath.ToSlash(filepath.Join(runRef, "evaluation_result.yaml")),
		},
		"candidate_identifier":                          "hardening/candidate@1",
		"task_family":                                   "bug_fix",
		"task_family_source":                            "task_type",
		"policy_affecting_observation":                  "A policy-sensitive regression note needs human hardening review.",
		"why_it_must_not_become_reusable_family_memory": "This observation would alter policy rather than reusable execution guidance.",
		"supporting_evidence_refs":                      []string{filepath.ToSlash(filepath.Join(runRef, "evaluation_result.yaml"))},
		"hardening_recommendation":                      "hold",
	}
	if err := writeYAML(candidateFile, candidate); err != nil {
		t.Fatalf("failed to write hardening candidate fixture: %v", err)
	}
	return candidateFile
}

func writeUserOutcomeFeedbackFixture(t *testing.T, projectRoot, fileName, candidateRef, classification string) string {
	t.Helper()
	if classification == "" {
		classification = "accepted"
	}
	candidateValue, err := contracts.ReadYAMLFile(filepath.Join(projectRoot, filepath.FromSlash(candidateRef)))
	if err != nil {
		t.Fatalf("failed to read candidate fixture: %v", err)
	}
	candidate, err := contracts.AsMap(candidateValue, candidateRef)
	if err != nil {
		t.Fatalf("failed to decode candidate fixture: %v", err)
	}
	identity := mapValue(candidate["originating_run_artifact_identity"])
	runRef := stringValue(identity["run_ref"])
	feedbackDir := filepath.Join(projectRoot, ".harness", "learning", "feedback")
	if err := os.MkdirAll(feedbackDir, 0o755); err != nil {
		t.Fatalf("failed to create feedback directory: %v", err)
	}
	feedbackFile := filepath.Join(feedbackDir, fileName)
	feedback := map[string]any{
		"originating_run_artifact_identity": map[string]any{
			"run_ref":      runRef,
			"artifact_ref": stringValue(identity["artifact_ref"]),
		},
		"candidate_ref_hint":      candidateRef,
		"task_family":             stringValue(candidate["task_family"]),
		"task_family_source":      fallbackString(stringValue(candidate["task_family_source"]), "task_type"),
		"feedback_classification": classification,
		"feedback_summary":        "The user confirmed the fix works in the target flow.",
		"submitted_at":            "2026-04-16T00:00:00Z",
		"evidence_refs": map[string]any{
			"run_refs":       []string{filepath.ToSlash(filepath.Join(runRef, "execution_report.yaml"))},
			"follow_up_refs": []string{},
		},
	}
	if err := writeYAML(feedbackFile, feedback); err != nil {
		t.Fatalf("failed to write feedback fixture: %v", err)
	}
	return feedbackFile
}

func writeLearningReviewFixture(t *testing.T, projectRoot, candidateRef, reviewerDecision string, feedbackRefs []string, approvedMemoryRef string) string {
	t.Helper()
	if reviewerDecision == "" {
		reviewerDecision = "hold"
	}
	decisionDir := filepath.Join(projectRoot, ".harness", "learning", "reviews")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		t.Fatalf("failed to create learning review directory: %v", err)
	}
	decisionFile := filepath.Join(decisionDir, "decision.yaml")
	decision := map[string]any{
		"candidate_ref": candidateRef,
		"candidate_user_outcome_status_at_review": "provisional",
		"reviewer_decision":                       reviewerDecision,
		"reviewer_identity":                       "test reviewer",
		"decision_timestamp":                      "2026-04-16T00:00:00Z",
		"decision_reason":                         "Promote the reviewed same-family candidate into canonical approved memory.",
		"considered_user_outcome_feedback_refs":   feedbackRefs,
		"guardrail_cost_predicate": map[string]any{
			"assessment": "intervention_cost_does_not_explain_improvement",
			"rationale":  "The result was not caused by excess intervention cost.",
		},
	}
	if approvedMemoryRef != "" {
		decision["resulting_approved_memory_ref"] = approvedMemoryRef
	}
	if err := writeYAML(decisionFile, decision); err != nil {
		t.Fatalf("failed to write learning review fixture: %v", err)
	}
	return decisionFile
}

func writeHardeningReviewFixture(t *testing.T, projectRoot, candidateRef string) string {
	t.Helper()
	decisionDir := filepath.Join(projectRoot, ".harness", "learning", "hardening-reviews")
	if err := os.MkdirAll(decisionDir, 0o755); err != nil {
		t.Fatalf("failed to create hardening review directory: %v", err)
	}
	decisionFile := filepath.Join(decisionDir, "decision.yaml")
	decision := map[string]any{
		"hardening_candidate_ref": candidateRef,
		"reviewer_decision":       "accept_for_hardening",
		"reviewer_identity":       "test reviewer",
		"decision_timestamp":      "2026-04-16T00:00:00Z",
		"decision_reason":         "This policy-affecting observation should be tracked as accepted hardening guidance.",
		"reviewed_observation_refs": []string{
			filepath.ToSlash(filepath.Join(filepath.Dir(filepath.Dir(candidateRef)), "evaluation_result.yaml")),
		},
		"follow_up_hardening_note_ref": ".harness/learning/hardening_notes/follow_up.md",
	}
	if err := writeYAML(decisionFile, decision); err != nil {
		t.Fatalf("failed to write hardening review fixture: %v", err)
	}
	return decisionFile
}
