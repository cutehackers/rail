package runtime

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"rail/internal/contracts"
)

var learningStoreSanitizer = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func (r *Runner) InitUserOutcomeFeedback(artifactPath, outputPath string) (string, error) {
	artifactDirectory, err := r.router.resolveArtifactDirectory(artifactPath)
	if err != nil {
		return "", err
	}

	state, err := readState(filepath.Join(artifactDirectory, "state.json"))
	if err != nil {
		return "", err
	}

	executionPath := filepath.Join(artifactDirectory, "execution_report.yaml")
	evaluationPath := filepath.Join(artifactDirectory, "evaluation_result.yaml")
	integrationPath := filepath.Join(artifactDirectory, "integration_result.yaml")
	hasExecution := fileExists(executionPath)
	hasEvaluation := fileExists(evaluationPath)
	if !hasExecution && !hasEvaluation {
		return "", fmt.Errorf("artifact directory must include execution_report.yaml or evaluation_result.yaml: %s", artifactPath)
	}

	runRef := filepath.ToSlash(mustRel(r.projectRoot, artifactDirectory))
	artifactRef := filepath.ToSlash(mustRel(r.projectRoot, evaluationPath))
	if !hasEvaluation {
		artifactRef = filepath.ToSlash(mustRel(r.projectRoot, executionPath))
	}

	executionMap := map[string]any{}
	if hasExecution {
		value, err := contracts.ReadYAMLFile(executionPath)
		if err != nil {
			return "", err
		}
		executionMap, err = contracts.AsMap(value, executionPath)
		if err != nil {
			return "", err
		}
	}

	qualityCandidateRefs := stringList(executionMap["quality_learning_candidate_refs"])
	evidenceRefs := []string{}
	if hasExecution {
		evidenceRefs = append(evidenceRefs, filepath.ToSlash(mustRel(r.projectRoot, executionPath)))
	}
	if hasEvaluation {
		evidenceRefs = append(evidenceRefs, filepath.ToSlash(mustRel(r.projectRoot, evaluationPath)))
	}
	if fileExists(integrationPath) {
		evidenceRefs = append(evidenceRefs, filepath.ToSlash(mustRel(r.projectRoot, integrationPath)))
	}

	draft := map[string]any{
		"originating_run_artifact_identity": map[string]any{
			"run_ref":      runRef,
			"artifact_ref": artifactRef,
		},
		"task_family":             state.TaskFamily,
		"task_family_source":      state.TaskFamilySource,
		"feedback_classification": "unresolved",
		"feedback_summary":        "TODO: summarize the direct user outcome for this run before applying feedback.",
		"submitted_at":            nowUTC(),
		"evidence_refs": map[string]any{
			"run_refs":       evidenceRefs,
			"follow_up_refs": []string{},
		},
	}
	if len(qualityCandidateRefs) == 1 {
		draft["candidate_ref_hint"] = qualityCandidateRefs[0]
	}

	outputRef := strings.TrimSpace(outputPath)
	if outputRef == "" {
		outputRef = defaultLearningDraftPath("feedback", runRef)
	}
	if err := r.writeAndValidateArtifact(outputRef, draft, "user_outcome_feedback"); err != nil {
		return "", err
	}
	resolved, err := contracts.ResolvePathWithinRoot(r.projectRoot, outputRef)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (r *Runner) InitLearningReview(candidateRef, outputPath string) (string, error) {
	candidateRelativePath, candidate, err := r.loadAndValidateArtifact(candidateRef, "quality_learning_candidate")
	if err != nil {
		return "", err
	}
	userOutcomeSignal := mapValue(candidate["user_outcome_signal"])

	draft := map[string]any{
		"candidate_ref": candidateRelativePath,
		"candidate_user_outcome_status_at_review": fallbackString(stringValue(userOutcomeSignal["status"]), "unavailable"),
		"reviewer_decision":                       "hold",
		"reviewer_identity":                       "TODO: reviewer",
		"decision_timestamp":                      nowUTC(),
		"decision_reason":                         "TODO: explain whether this candidate should be promoted, held, or rejected.",
		"considered_user_outcome_feedback_refs":   stringList(userOutcomeSignal["supporting_feedback_refs"]),
		"guardrail_cost_predicate": map[string]any{
			"assessment": "not_assessed",
			"rationale":  "TODO: assess whether intervention cost explains any apparent improvement.",
		},
	}

	outputRef := strings.TrimSpace(outputPath)
	if outputRef == "" {
		outputRef = defaultLearningDraftPath("reviews", candidateRelativePath)
	}
	if err := r.writeAndValidateArtifact(outputRef, draft, "learning_review_decision"); err != nil {
		return "", err
	}
	resolved, err := contracts.ResolvePathWithinRoot(r.projectRoot, outputRef)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (r *Runner) InitHardeningReview(candidateRef, outputPath string) (string, error) {
	candidateRelativePath, candidate, err := r.loadAndValidateArtifact(candidateRef, "hardening_candidate")
	if err != nil {
		return "", err
	}

	draft := map[string]any{
		"hardening_candidate_ref": candidateRelativePath,
		"reviewer_decision":       "hold",
		"reviewer_identity":       "TODO: reviewer",
		"decision_timestamp":      nowUTC(),
		"decision_reason":         "TODO: explain whether this hardening observation should be accepted, held, or rejected.",
		"reviewed_observation_refs": stringList(
			candidate["supporting_evidence_refs"],
		),
	}

	outputRef := strings.TrimSpace(outputPath)
	if outputRef == "" {
		outputRef = defaultLearningDraftPath("hardening-reviews", candidateRelativePath)
	}
	if err := r.writeAndValidateArtifact(outputRef, draft, "hardening_review_decision"); err != nil {
		return "", err
	}
	resolved, err := contracts.ResolvePathWithinRoot(r.projectRoot, outputRef)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

func (r *Runner) ApplyUserOutcomeFeedback(feedbackPath string) (string, error) {
	feedbackRef, feedback, err := r.loadAndValidateArtifact(feedbackPath, "user_outcome_feedback")
	if err != nil {
		return "", err
	}
	feedbackIdentity := mapValue(feedback["originating_run_artifact_identity"])
	taskFamily := stringValue(feedback["task_family"])
	taskFamilySource := fallbackString(stringValue(feedback["task_family_source"]), "task_type")
	runRef := stringValue(feedbackIdentity["run_ref"])
	artifactRef := stringValue(feedbackIdentity["artifact_ref"])
	candidateRefHint := stringValue(feedback["candidate_ref_hint"])
	evidenceRefs := flattenFeedbackEvidenceRefs(feedback)

	candidateFiles, err := qualityCandidateFiles(r.projectRoot)
	if err != nil {
		return "", err
	}
	matched := []string{}
	for _, candidateFile := range candidateFiles {
		candidateRef := filepath.ToSlash(mustRel(r.projectRoot, candidateFile))
		value, err := contracts.ReadYAMLFile(candidateFile)
		if err != nil {
			return "", err
		}
		candidate, err := contracts.AsMap(value, candidateFile)
		if err != nil {
			return "", err
		}
		candidateIdentity := mapValue(candidate["originating_run_artifact_identity"])
		if stringValue(candidateIdentity["run_ref"]) != runRef ||
			stringValue(candidateIdentity["artifact_ref"]) != artifactRef ||
			stringValue(candidate["task_family"]) != taskFamily ||
			fallbackString(stringValue(candidate["task_family_source"]), "task_type") != taskFamilySource {
			continue
		}
		if candidateRefHint != "" && candidateRef != candidateRefHint {
			continue
		}
		matched = append(matched, candidateRef)
	}

	if len(matched) > 1 && candidateRefHint == "" {
		return "", fmt.Errorf("feedback `%s` matches multiple candidates. Provide `candidate_ref_hint` so the evidence attaches to exactly one candidate.", feedbackRef)
	}

	for _, candidateRef := range matched {
		candidatePath, err := contracts.ResolvePathWithinRoot(r.projectRoot, candidateRef)
		if err != nil {
			return "", err
		}
		value, err := contracts.ReadYAMLFile(candidatePath)
		if err != nil {
			return "", err
		}
		candidate, err := contracts.AsMap(value, candidatePath)
		if err != nil {
			return "", err
		}
		userOutcomeSignal := mapValue(candidate["user_outcome_signal"])
		userOutcomeSignal["supporting_feedback_refs"] = mergeDistinctStrings(
			stringList(userOutcomeSignal["supporting_feedback_refs"]),
			[]string{feedbackRef},
		)
		if stringValue(userOutcomeSignal["status"]) != "confirmed" {
			userOutcomeSignal["status"] = "provisional"
			userOutcomeSignal["summary"] = fmt.Sprintf(
				"Direct user outcome evidence was appended from `%s` and remains provisional until an explicit learning review reconciles it.",
				feedbackRef,
			)
		}
		candidate["supporting_evidence_refs"] = mergeDistinctStrings(
			stringList(candidate["supporting_evidence_refs"]),
			evidenceRefs,
		)
		if err := r.writeAndValidateArtifact(candidateRef, candidate, "quality_learning_candidate"); err != nil {
			return "", err
		}
	}

	if err := r.refreshLearningViews(); err != nil {
		return "", err
	}
	if len(matched) == 0 {
		return fmt.Sprintf("Applied user outcome feedback at %s (no matching quality candidate was updated; queue/index views were refreshed with a reviewable trail).", feedbackRef), nil
	}
	return fmt.Sprintf("Applied user outcome feedback at %s and updated %d matching quality candidate(s) with reviewable feedback refs.", feedbackRef, len(matched)), nil
}

func (r *Runner) ApplyLearningReview(decisionPath string) (string, error) {
	decisionRef, decision, err := r.loadAndValidateArtifact(decisionPath, "learning_review_decision")
	if err != nil {
		return "", err
	}
	candidateRef := stringValue(decision["candidate_ref"])
	candidateRelativePath, candidate, err := r.loadAndValidateArtifact(candidateRef, "quality_learning_candidate")
	if err != nil {
		return "", err
	}

	rawDecision := stringValue(decision["reviewer_decision"])
	reviewStatusAtDecision := stringValue(decision["candidate_user_outcome_status_at_review"])
	guardrailPredicate := mapValue(decision["guardrail_cost_predicate"])
	guardrailAssessment := stringValue(guardrailPredicate["assessment"])
	guardrailRationale := stringValue(guardrailPredicate["rationale"])

	userOutcomeSignal := mapValue(candidate["user_outcome_signal"])
	actualStatusAtReview := fallbackString(stringValue(userOutcomeSignal["status"]), "unavailable")
	if actualStatusAtReview != reviewStatusAtDecision {
		return "", fmt.Errorf("learning review `%s` expects candidate user_outcome_signal.status=`%s`, but the candidate is currently `%s`", decisionRef, reviewStatusAtDecision, actualStatusAtReview)
	}

	candidateIdentity := mapValue(candidate["originating_run_artifact_identity"])
	expectedRunRef := stringValue(candidateIdentity["run_ref"])
	expectedTaskFamily := stringValue(candidate["task_family"])
	expectedTaskFamilySource := fallbackString(stringValue(candidate["task_family_source"]), "task_type")

	feedbackRefs := mergeDistinctStrings(
		stringList(userOutcomeSignal["supporting_feedback_refs"]),
		stringList(decision["considered_user_outcome_feedback_refs"]),
	)
	feedbackEntries, err := r.loadFeedbackArtifacts(feedbackRefs, expectedRunRef, expectedTaskFamily, expectedTaskFamilySource)
	if err != nil {
		return "", err
	}
	acceptedFeedbackEntries := []mapEntry{}
	for _, entry := range feedbackEntries {
		if stringValue(entry.value["feedback_classification"]) == "accepted" {
			acceptedFeedbackEntries = append(acceptedFeedbackEntries, entry)
		}
	}
	acceptedFeedbackRefs := make([]string, 0, len(acceptedFeedbackEntries))
	feedbackEvidenceRefs := []string{}
	for _, entry := range acceptedFeedbackEntries {
		acceptedFeedbackRefs = append(acceptedFeedbackRefs, entry.key)
		feedbackEvidenceRefs = append(feedbackEvidenceRefs, flattenFeedbackEvidenceRefs(entry.value)...)
	}

	if rawDecision == "promote" && guardrailAssessment != "intervention_cost_does_not_explain_improvement" {
		return "", fmt.Errorf("promote decision `%s` is invalid because the guardrail-cost predicate does not clear promotion", decisionRef)
	}

	candidateUpdated := cloneMap(candidate)
	userOutcomeSignalUpdated := mapValue(candidateUpdated["user_outcome_signal"])
	if len(acceptedFeedbackEntries) > 0 {
		userOutcomeSignalUpdated["status"] = "confirmed"
		userOutcomeSignalUpdated["supporting_feedback_refs"] = acceptedFeedbackRefs
		userOutcomeSignalUpdated["summary"] = fmt.Sprintf(
			"Confirmed during explicit review by `%s` after considering appended direct user outcome feedback.",
			decisionRef,
		)
		candidateUpdated["supporting_evidence_refs"] = mergeDistinctStrings(
			stringList(candidateUpdated["supporting_evidence_refs"]),
			feedbackEvidenceRefs,
		)
	}

	finalUserOutcomeStatus := fallbackString(stringValue(userOutcomeSignalUpdated["status"]), actualStatusAtReview)
	if rawDecision == "promote" && finalUserOutcomeStatus != "confirmed" {
		return "", fmt.Errorf("promote decision `%s` requires confirmed direct user outcome evidence or accepted feedback refs considered during review", decisionRef)
	}

	approvedMemoryRef := approvedMemoryRefForFamily(expectedTaskFamily)
	var approvedMemory map[string]any
	if rawDecision == "promote" {
		requestedApprovedRef := stringValue(decision["resulting_approved_memory_ref"])
		if requestedApprovedRef != approvedMemoryRef {
			return "", fmt.Errorf("promote decision `%s` must target `%s`, but requested `%s`", decisionRef, approvedMemoryRef, requestedApprovedRef)
		}

		var existingApprovedMemory map[string]any
		if fileExists(filepath.Join(r.projectRoot, filepath.FromSlash(approvedMemoryRef))) {
			_, existingApprovedMemory, err = r.loadAndValidateArtifact(approvedMemoryRef, "approved_family_memory")
			if err != nil {
				return "", err
			}
		}
		approvedMemory, err = r.buildApprovedFamilyMemory(candidateRelativePath, candidateUpdated, decisionRef, decision, acceptedFeedbackEntries, existingApprovedMemory)
		if err != nil {
			return "", err
		}
	}

	if len(acceptedFeedbackEntries) > 0 {
		if err := r.writeAndValidateArtifact(candidateRelativePath, candidateUpdated, "quality_learning_candidate"); err != nil {
			return "", err
		}
	}

	if rawDecision == "promote" {
		if err := r.writeAndValidateArtifact(approvedMemoryRef, approvedMemory, "approved_family_memory"); err != nil {
			return "", err
		}
	}

	if err := r.persistReviewDecision("learning_review_decisions", candidateRef, decision, "learning_review_decision"); err != nil {
		return "", err
	}
	if err := r.refreshLearningViews(); err != nil {
		return "", err
	}
	return fmt.Sprintf(
		"Applied learning review `%s` for `%s` with disposition `%s` (guardrail assessment: %s, rationale: %s).",
		decisionRef, candidateRelativePath, rawDecision, guardrailAssessment, guardrailRationale,
	), nil
}

func (r *Runner) ApplyHardeningReview(decisionPath string) (string, error) {
	decisionRef, decision, err := r.loadAndValidateArtifact(decisionPath, "hardening_review_decision")
	if err != nil {
		return "", err
	}
	candidateRef := stringValue(decision["hardening_candidate_ref"])
	if strings.TrimSpace(candidateRef) != "" {
		resolved, err := contracts.ResolvePathWithinRoot(r.projectRoot, candidateRef)
		if err != nil {
			return "", err
		}
		if fileExists(resolved) {
			if _, err := contracts.ReadYAMLFile(resolved); err != nil {
				return "", err
			}
		}
	}
	if err := r.persistReviewDecision("hardening_review_decisions", candidateRef, decision, "hardening_review_decision"); err != nil {
		return "", err
	}
	if err := r.refreshLearningViews(); err != nil {
		return "", err
	}
	return fmt.Sprintf("Applied hardening review `%s` for `%s`; reusable family memory was not created.", decisionRef, candidateRef), nil
}

func (r *Runner) loadAndValidateArtifact(path, schemaName string) (string, map[string]any, error) {
	validator, err := contracts.NewValidator(r.projectRoot)
	if err != nil {
		return "", nil, err
	}
	resolved, err := contracts.ResolvePathWithinRoot(r.projectRoot, path)
	if err != nil {
		return "", nil, err
	}
	relative := relativeWithinProjectRoot(r.projectRoot, path, resolved)
	value, err := validator.ValidateArtifactFile(relative, schemaName)
	if err != nil {
		return "", nil, err
	}
	return relative, value, nil
}

func (r *Runner) writeAndValidateArtifact(relativePath string, value map[string]any, schemaName string) error {
	resolved, err := contracts.ResolvePathWithinRoot(r.projectRoot, relativePath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}
	if err := writeYAML(resolved, value); err != nil {
		return err
	}
	validator, err := contracts.NewValidator(r.projectRoot)
	if err != nil {
		return err
	}
	_, err = validator.ValidateArtifactFile(filepath.ToSlash(mustRel(r.projectRoot, resolved)), schemaName)
	return err
}

func (r *Runner) persistReviewDecision(categoryDirectory, sourceRef string, decision map[string]any, schemaName string) error {
	storeRef := filepath.ToSlash(filepath.Join(".harness", "learning", categoryDirectory, sanitizeLearningStoreComponent(sourceRef)+".yaml"))
	return r.writeAndValidateArtifact(storeRef, decision, schemaName)
}

func defaultLearningDraftPath(categoryDirectory, sourceRef string) string {
	return filepath.ToSlash(filepath.Join(".harness", "learning", categoryDirectory, sanitizeLearningStoreComponent(sourceRef)+".yaml"))
}

func sanitizeLearningStoreComponent(value string) string {
	sanitized := learningStoreSanitizer.ReplaceAllString(value, "_")
	if sanitized == "" {
		return "decision"
	}
	return sanitized
}

func (r *Runner) refreshLearningViews() error {
	derived, err := deriveLearningViews(r.projectRoot)
	if err != nil {
		return err
	}
	snapshots := []struct {
		path   string
		value  map[string]any
		schema string
	}{
		{path: ".harness/learning/review_queue.yaml", value: derived.reviewQueue, schema: "learning_review_queue"},
		{path: ".harness/learning/hardening_queue.yaml", value: derived.hardeningQueue, schema: "hardening_review_queue"},
		{path: ".harness/learning/family_evidence_index.yaml", value: derived.familyEvidenceIndex, schema: "family_evidence_index"},
	}
	for _, snapshot := range snapshots {
		if err := r.writeAndValidateArtifact(snapshot.path, snapshot.value, snapshot.schema); err != nil {
			return err
		}
	}
	return nil
}

func qualityCandidateFiles(projectRoot string) ([]string, error) {
	artifactRoot := filepath.Join(projectRoot, ".harness", "artifacts")
	files, err := sortedYAMLFilesStrict(artifactRoot)
	if err != nil {
		return nil, err
	}
	result := []string{}
	for _, file := range files {
		if filepath.Base(filepath.Dir(file)) == "quality_learning_candidates" && !isOrdinalCandidateAlias(file) {
			result = append(result, file)
		}
	}
	return result, nil
}

func (r *Runner) loadFeedbackArtifacts(feedbackRefs []string, expectedRunRef, expectedTaskFamily, expectedTaskFamilySource string) ([]mapEntry, error) {
	result := []mapEntry{}
	for _, ref := range feedbackRefs {
		relative, feedback, err := r.loadAndValidateArtifact(ref, "user_outcome_feedback")
		if err != nil {
			return nil, err
		}
		identity := mapValue(feedback["originating_run_artifact_identity"])
		if stringValue(identity["run_ref"]) != expectedRunRef ||
			stringValue(feedback["task_family"]) != expectedTaskFamily ||
			fallbackString(stringValue(feedback["task_family_source"]), "task_type") != expectedTaskFamilySource {
			return nil, fmt.Errorf("feedback `%s` does not match the candidate run/family identity", relative)
		}
		result = append(result, mapEntry{key: relative, value: feedback})
	}
	return result, nil
}

func flattenFeedbackEvidenceRefs(feedback map[string]any) []string {
	evidenceRefs := mapValue(feedback["evidence_refs"])
	return mergeDistinctStrings(
		stringList(evidenceRefs["run_refs"]),
		stringList(evidenceRefs["follow_up_refs"]),
	)
}

func (r *Runner) buildApprovedFamilyMemory(candidateRef string, candidate map[string]any, decisionRef string, decision map[string]any, acceptedFeedbackEntries []mapEntry, existingApprovedMemory map[string]any) (map[string]any, error) {
	contextContract, err := r.bootstrapper.loadMap(".harness/supervisor/context_contract.yaml")
	if err != nil {
		return nil, err
	}
	policy, err := r.bootstrapper.loadMap(".harness/supervisor/policy.yaml")
	if err != nil {
		return nil, err
	}

	taskFamily := fallbackString(stringValue(candidate["task_family"]), "unknown")
	taskFamilySource := fallbackString(stringValue(candidate["task_family_source"]), "task_type")
	lookupKey := familySourceKey(taskFamily, taskFamilySource)
	effectiveContextSignal := mapValue(candidate["effective_context_signal"])
	guardrailCost := mapValue(candidate["guardrail_cost"])
	guardrailPredicate := mapValue(decision["guardrail_cost_predicate"])

	acceptedFeedbackRefs := []string{}
	acceptedFeedbackEvidenceRefs := []string{}
	for _, entry := range acceptedFeedbackEntries {
		acceptedFeedbackRefs = append(acceptedFeedbackRefs, entry.key)
		acceptedFeedbackEvidenceRefs = append(acceptedFeedbackEvidenceRefs, flattenFeedbackEvidenceRefs(entry.value)...)
	}

	previousFreshnessSequence := 0
	if len(existingApprovedMemory) > 0 {
		previousFreshnessSequence = intValue(mapValue(existingApprovedMemory["freshness_marker"])["freshness_sequence"])
	}
	dispositionHistory := []map[string]any{
		{
			"disposition":       "approved",
			"decided_at":        stringValue(decision["decision_timestamp"]),
			"decision_ref":      decisionRef,
			"reviewer_identity": stringValue(decision["reviewer_identity"]),
			"reason":            stringValue(decision["decision_reason"]),
		},
	}

	approvedMemory := map[string]any{
		"task_family":          taskFamily,
		"task_family_source":   taskFamilySource,
		"approved_observation": fallbackString(stringValue(candidate["candidate_claim"]), stringValue(candidate["quality_outcome_summary"])),
		"applicability_conditions": mergeDistinctStrings(
			stringList(effectiveContextSignal["helped_context_factors"]),
			stringList(effectiveContextSignal["neutral_context_factors"]),
		),
		"evidence_basis": mergeDistinctStrings(
			stringList(candidate["supporting_evidence_refs"]),
			acceptedFeedbackEvidenceRefs,
		),
		"guardrail_note": fmt.Sprintf(
			"%s | Review predicate: %s. %s",
			stringValue(guardrailCost["summary"]),
			stringValue(guardrailPredicate["assessment"]),
			stringValue(guardrailPredicate["rationale"]),
		),
		"request_compatibility": map[string]any{
			"required_context_features":   []string{taskFamily},
			"goal_must_include_any":       []string{strings.ReplaceAll(taskFamily, "_", "-")},
			"goal_must_exclude_any":       []string{"policy override", "override supervisor policy"},
			"constraint_must_include_any": []string{},
			"constraint_must_exclude_any": []string{"policy override"},
		},
		"repository_compatibility": map[string]any{
			"required_paths_exist":  []string{".harness/project.yaml"},
			"required_paths_absent": []string{},
		},
		"latest_family_evidence_expectations": map[string]any{
			"lookup_key":                   lookupKey,
			"baseline_approved_memory_ref": approvedMemoryRefForFamily(taskFamily),
			"required_latest_success_ref":  latestAcceptedFeedbackRef(acceptedFeedbackRefs),
			"required_latest_failure_ref":  nil,
			"forbid_any_latest_failure":    true,
		},
		"freshness_marker": map[string]any{
			"contract_version":           intValue(contextContract["contract_version"]),
			"policy_version":             intValue(policy["policy_version"]),
			"memory_schema_version":      3,
			"repository_assumptions_ref": filepath.ToSlash(filepath.Join(stringValue(mapValue(candidate["originating_run_artifact_identity"])["run_ref"]), "request.yaml")),
			"repository_state_ref":       filepath.ToSlash(filepath.Join(stringValue(mapValue(candidate["originating_run_artifact_identity"])["run_ref"]), "state.json")),
			"refreshed_at":               stringValue(decision["decision_timestamp"]),
			"freshness_sequence":         previousFreshnessSequence + 1,
		},
		"disposition_history":                 dispositionHistory,
		"originating_candidate_refs":          mergeDistinctStrings(stringList(existingApprovedMemory["originating_candidate_refs"]), []string{candidateRef}),
		"reviewed_user_outcome_feedback_refs": mergeDistinctStrings(stringList(existingApprovedMemory["reviewed_user_outcome_feedback_refs"]), acceptedFeedbackRefs),
	}
	if len(stringList(approvedMemory["applicability_conditions"])) == 0 {
		approvedMemory["applicability_conditions"] = []string{"Baseline repo context was sufficient."}
	}
	if len(stringList(approvedMemory["evidence_basis"])) == 0 {
		approvedMemory["evidence_basis"] = []string{candidateRef}
	}
	return approvedMemory, nil
}

func latestAcceptedFeedbackRef(refs []string) any {
	if len(refs) == 0 {
		return nil
	}
	return refs[len(refs)-1]
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func cloneMap(source map[string]any) map[string]any {
	return normalizeForJSON(source).(map[string]any)
}

func relativeWithinProjectRoot(projectRoot, originalPath, resolvedPath string) string {
	if !filepath.IsAbs(originalPath) {
		return filepath.ToSlash(filepath.Clean(originalPath))
	}

	canonicalRoot := projectRoot
	if value, err := filepath.EvalSymlinks(projectRoot); err == nil {
		canonicalRoot = value
	}
	canonicalResolved := resolvedPath
	if value, err := filepath.EvalSymlinks(resolvedPath); err == nil {
		canonicalResolved = value
	}
	if relative, err := filepath.Rel(canonicalRoot, canonicalResolved); err == nil {
		return filepath.ToSlash(relative)
	}
	return filepath.ToSlash(mustRel(projectRoot, resolvedPath))
}
