package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"rail/internal/contracts"
)

type derivedLearningViews struct {
	reviewQueue         map[string]any
	hardeningQueue      map[string]any
	familyEvidenceIndex map[string]any
}

func (r *Runner) VerifyLearningState() (string, error) {
	failures := []string{}
	learningDirectory := filepath.Join(r.projectRoot, ".harness", "learning")
	approvedDirectory := filepath.Join(learningDirectory, "approved")

	approvedFiles, err := sortedYAMLFilesStrict(approvedDirectory)
	if err != nil {
		return "", errors.New(formatLearningVerificationFailure([]string{fmt.Sprintf("Could not scan approved memory directory: %v", err)}))
	}
	for _, file := range approvedFiles {
		ref := filepath.ToSlash(mustRel(r.projectRoot, file))
		approvedMemory := validateVerificationYAMLFile(r.projectRoot, file, ref, "approved_family_memory", "Approved memory", &failures)
		if approvedMemory != nil {
			taskFamily, _ := approvedMemory["task_family"].(string)
			expectedRef := approvedMemoryRefForFamily(taskFamily)
			if ref != expectedRef {
				failures = append(failures, fmt.Sprintf("Approved memory `%s` must use the canonical family path `%s`.", ref, expectedRef))
			}
		}
	}

	artifactRoot := filepath.Join(r.projectRoot, ".harness", "artifacts")
	artifactFiles, err := sortedYAMLFilesStrict(artifactRoot)
	if err != nil {
		return "", errors.New(formatLearningVerificationFailure([]string{fmt.Sprintf("Could not scan artifact directory: %v", err)}))
	}
	for _, file := range artifactFiles {
		parent := filepath.Base(filepath.Dir(file))
		ref := filepath.ToSlash(mustRel(r.projectRoot, file))
		switch {
		case parent == "quality_learning_candidates" && !isOrdinalCandidateAlias(file):
			validateVerificationYAMLFile(r.projectRoot, file, ref, "quality_learning_candidate", "Quality learning candidate", &failures)
		case parent == "hardening_candidates":
			validateVerificationYAMLFile(r.projectRoot, file, ref, "hardening_candidate", "Hardening candidate", &failures)
		}
	}

	sourceDirectories := []struct {
		dir    string
		schema string
		label  string
	}{
		{dir: filepath.Join(learningDirectory, "learning_review_decisions"), schema: "learning_review_decision", label: "Learning review decision"},
		{dir: filepath.Join(learningDirectory, "hardening_review_decisions"), schema: "hardening_review_decision", label: "Hardening review decision"},
		{dir: filepath.Join(learningDirectory, "feedback"), schema: "user_outcome_feedback", label: "User outcome feedback"},
	}
	for _, source := range sourceDirectories {
		files, err := sortedYAMLFilesStrict(source.dir)
		if err != nil {
			failures = append(failures, fmt.Sprintf("Could not scan %s directory: %v", source.label, err))
			continue
		}
		for _, file := range files {
			ref := filepath.ToSlash(mustRel(r.projectRoot, file))
			validateVerificationYAMLFile(r.projectRoot, file, ref, source.schema, source.label, &failures)
		}
	}

	if len(failures) > 0 {
		return "", errors.New(formatLearningVerificationFailure(failures))
	}

	derived, err := deriveLearningViews(r.projectRoot)
	if err != nil {
		failures = append(failures, fmt.Sprintf("Could not derive expected learning snapshots from the current reviewed state: %v", err))
		return "", errors.New(formatLearningVerificationFailure(failures))
	}

	reviewQueueSnapshot := loadLearningSnapshot(r.projectRoot, ".harness/learning/review_queue.yaml", "learning_review_queue", &failures)
	hardeningQueueSnapshot := loadLearningSnapshot(r.projectRoot, ".harness/learning/hardening_queue.yaml", "hardening_review_queue", &failures)
	familyEvidenceSnapshot := loadLearningSnapshot(r.projectRoot, ".harness/learning/family_evidence_index.yaml", "family_evidence_index", &failures)
	if len(failures) > 0 {
		return "", errors.New(formatLearningVerificationFailure(failures))
	}

	verifyLearningSnapshotDrift(".harness/learning/review_queue.yaml", reviewQueueSnapshot, derived.reviewQueue, &failures)
	verifyLearningSnapshotDrift(".harness/learning/hardening_queue.yaml", hardeningQueueSnapshot, derived.hardeningQueue, &failures)
	verifyLearningSnapshotDrift(".harness/learning/family_evidence_index.yaml", familyEvidenceSnapshot, derived.familyEvidenceIndex, &failures)
	if len(failures) > 0 {
		return "", errors.New(formatLearningVerificationFailure(failures))
	}

	return "Learning state verification passed.", nil
}

func validateVerificationYAMLFile(projectRoot, file, relativePath, schemaName, label string, failures *[]string) map[string]any {
	validator, err := contracts.NewValidator(projectRoot)
	if err != nil {
		*failures = append(*failures, fmt.Sprintf("%s `%s` could not initialize validation: %v", label, relativePath, err))
		return nil
	}
	value, err := validator.ValidateArtifactFile(file, schemaName)
	if err != nil {
		*failures = append(*failures, fmt.Sprintf("%s `%s` is invalid for `%s`: %v", label, relativePath, schemaName, err))
		return nil
	}
	return value
}

func loadLearningSnapshot(projectRoot, relativePath, schemaName string, failures *[]string) map[string]any {
	filePath := filepath.Join(projectRoot, filepath.FromSlash(relativePath))
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		*failures = append(*failures, fmt.Sprintf("Derived learning snapshot `%s` is missing. Only `apply-*` commands should regenerate it.", relativePath))
		return map[string]any{}
	}
	return validateVerificationYAMLFile(projectRoot, filePath, relativePath, schemaName, "Derived learning snapshot", failures)
}

func verifyLearningSnapshotDrift(relativePath string, actual, expected map[string]any, failures *[]string) {
	if canonicalJSON(actual) == canonicalJSON(expected) {
		return
	}

	mismatched := map[string]struct{}{}
	for key := range actual {
		if canonicalJSON(actual[key]) != canonicalJSON(expected[key]) {
			mismatched[key] = struct{}{}
		}
	}
	for key := range expected {
		if canonicalJSON(actual[key]) != canonicalJSON(expected[key]) {
			mismatched[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(mismatched))
	for key := range mismatched {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	summary := "snapshot content mismatch"
	if len(keys) > 0 {
		summary = "mismatched field(s): " + strings.Join(keys, ", ")
	}
	*failures = append(*failures, fmt.Sprintf(
		"Derived learning snapshot `%s` has drifted from the current reviewed state (%s). Regenerate it through the appropriate `apply-*` command instead of editing snapshots directly.",
		relativePath,
		summary,
	))
}

func formatLearningVerificationFailure(failures []string) string {
	var builder strings.Builder
	builder.WriteString("Learning state verification failed:\n")
	for _, failure := range failures {
		builder.WriteString("- ")
		builder.WriteString(failure)
		builder.WriteByte('\n')
	}
	return strings.TrimRight(builder.String(), "\n")
}

func sortedYAMLFiles(directory string) []string {
	info, err := os.Stat(directory)
	if err != nil || !info.IsDir() {
		return []string{}
	}
	files := []string{}
	_ = filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return err
		}
		if strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	})
	sort.Strings(files)
	return files
}

func sortedYAMLFilesStrict(directory string) ([]string, error) {
	info, err := os.Stat(directory)
	switch {
	case err == nil && !info.IsDir():
		return nil, fmt.Errorf("%s is not a directory", directory)
	case err == nil:
		// continue
	case os.IsNotExist(err):
		return []string{}, nil
	default:
		return nil, err
	}

	files := []string{}
	if err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info == nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".yaml") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

var ordinalCandidatePattern = regexp.MustCompile(`^\d+\.yaml$`)

func isOrdinalCandidateAlias(path string) bool {
	return ordinalCandidatePattern.MatchString(filepath.Base(path))
}

func approvedMemoryRefForFamily(taskFamily string) string {
	return filepath.ToSlash(filepath.Join(".harness", "learning", "approved", taskFamily+".yaml"))
}

func mustRel(root, path string) string {
	relative, err := filepath.Rel(root, path)
	if err != nil {
		return path
	}
	return relative
}

func deriveLearningViews(projectRoot string) (derivedLearningViews, error) {
	learningDirectory := filepath.Join(projectRoot, ".harness", "learning")
	artifactRoot := filepath.Join(projectRoot, ".harness", "artifacts")
	learningReviewDecisionDirectory := filepath.Join(learningDirectory, "learning_review_decisions")
	hardeningReviewDecisionDirectory := filepath.Join(learningDirectory, "hardening_review_decisions")
	feedbackDirectory := filepath.Join(learningDirectory, "feedback")
	approvedDirectory := filepath.Join(learningDirectory, "approved")

	qualityCandidates := map[string]map[string]any{}
	qualityCandidateMarkers := map[string]string{}
	for _, file := range sortedYAMLFiles(artifactRoot) {
		if filepath.Base(filepath.Dir(file)) != "quality_learning_candidates" || isOrdinalCandidateAlias(file) {
			continue
		}
		ref := filepath.ToSlash(mustRel(projectRoot, file))
		value, err := contracts.ReadYAMLFile(file)
		if err != nil {
			return derivedLearningViews{}, err
		}
		candidate, err := contracts.AsMap(value, file)
		if err != nil {
			return derivedLearningViews{}, err
		}
		qualityCandidates[ref] = candidate
		qualityCandidateMarkers[ref] = candidateOrderingMarker(candidate)
	}
	activeQualityCandidates := selectActiveCandidates(qualityCandidates, qualityCandidateMarkers)

	hardeningCandidates := map[string]map[string]any{}
	hardeningCandidateMarkers := map[string]string{}
	for _, file := range sortedYAMLFiles(artifactRoot) {
		if filepath.Base(filepath.Dir(file)) != "hardening_candidates" {
			continue
		}
		ref := filepath.ToSlash(mustRel(projectRoot, file))
		value, err := contracts.ReadYAMLFile(file)
		if err != nil {
			return derivedLearningViews{}, err
		}
		candidate, err := contracts.AsMap(value, file)
		if err != nil {
			return derivedLearningViews{}, err
		}
		hardeningCandidates[ref] = candidate
		hardeningCandidateMarkers[ref] = hardeningCandidateOrderingMarker(candidate)
	}
	activeHardeningCandidates := selectActiveCandidates(hardeningCandidates, hardeningCandidateMarkers)

	learningReviewDecisions, learningReviewDecisionTimes, err := loadNamedMaps(projectRoot, learningReviewDecisionDirectory, learningDecisionOrderingMarker)
	if err != nil {
		return derivedLearningViews{}, err
	}
	hardeningReviewDecisions, hardeningReviewDecisionTimes, err := loadNamedMaps(projectRoot, hardeningReviewDecisionDirectory, learningDecisionOrderingMarker)
	if err != nil {
		return derivedLearningViews{}, err
	}
	userOutcomeFeedbacks, userOutcomeFeedbackTimes, err := loadNamedMaps(projectRoot, feedbackDirectory, feedbackOrderingMarker)
	if err != nil {
		return derivedLearningViews{}, err
	}
	approvedMemories, approvedMemoryTimes, err := loadNamedMaps(projectRoot, approvedDirectory, approvedMemoryOrderingMarker)
	if err != nil {
		return derivedLearningViews{}, err
	}

	reviewDecisionsByCandidate := map[string]mapEntry{}
	for ref, decision := range learningReviewDecisions {
		candidateRef := stringValue(decision["candidate_ref"])
		if candidateRef == "" {
			continue
		}
		timestamp := stringValue(decision["decision_timestamp"])
		existing, ok := reviewDecisionsByCandidate[candidateRef]
		if !ok || timestamp >= stringValue(existing.value["decision_timestamp"]) {
			reviewDecisionsByCandidate[candidateRef] = mapEntry{key: ref, value: decision}
		}
	}

	hardeningDecisionsByCandidate := map[string]mapEntry{}
	for ref, decision := range hardeningReviewDecisions {
		candidateRef := stringValue(decision["hardening_candidate_ref"])
		if candidateRef == "" {
			continue
		}
		timestamp := stringValue(decision["decision_timestamp"])
		existing, ok := hardeningDecisionsByCandidate[candidateRef]
		if !ok || timestamp >= stringValue(existing.value["decision_timestamp"]) {
			hardeningDecisionsByCandidate[candidateRef] = mapEntry{key: ref, value: decision}
		}
	}

	latestFamilyDecisionByGroup := map[string]map[string]string{}
	for candidateRef, candidate := range activeQualityCandidates {
		decisionEntry, ok := reviewDecisionsByCandidate[candidateRef]
		if !ok {
			continue
		}
		family := stringValue(candidate["task_family"])
		if family == "" {
			family = "unknown"
		}
		familySource := fallbackString(stringValue(candidate["task_family_source"]), "task_type")
		groupKey := familySourceKey(family, familySource)
		timestamp := stringValue(decisionEntry.value["decision_timestamp"])
		existing := latestFamilyDecisionByGroup[groupKey]
		if existing == nil || timestamp >= existing["timestamp"] {
			latestFamilyDecisionByGroup[groupKey] = map[string]string{
				"ref":       decisionEntry.key,
				"timestamp": timestamp,
				"decision":  fallbackString(stringValue(decisionEntry.value["reviewer_decision"]), "pending"),
			}
		}
	}

	feedbackRefsByFamilyGroup := map[string][]string{}
	feedbackRefsByRunAndFamily := map[string][]string{}
	latestFeedbackByFamilyGroup := map[string]mapEntry{}
	for ref, feedback := range userOutcomeFeedbacks {
		identity := mapValue(feedback["originating_run_artifact_identity"])
		taskFamily := fallbackString(stringValue(feedback["task_family"]), "unknown")
		taskFamilySource := fallbackString(stringValue(feedback["task_family_source"]), "task_type")
		familyGroupKey := familySourceKey(taskFamily, taskFamilySource)
		feedbackRefsByFamilyGroup[familyGroupKey] = mergeDistinctStrings(feedbackRefsByFamilyGroup[familyGroupKey], []string{ref})
		runFamilyKey := fmt.Sprintf("%s::%s::%s", stringValue(identity["run_ref"]), taskFamily, taskFamilySource)
		feedbackRefsByRunAndFamily[runFamilyKey] = mergeDistinctStrings(feedbackRefsByRunAndFamily[runFamilyKey], []string{ref})
		existing, ok := latestFeedbackByFamilyGroup[familyGroupKey]
		if !ok || feedbackOrderingMarker(feedback) >= feedbackOrderingMarker(existing.value) {
			latestFeedbackByFamilyGroup[familyGroupKey] = mapEntry{key: ref, value: feedback}
		}
	}

	families := map[string]map[string]any{}
	for candidateRef, candidate := range activeQualityCandidates {
		family := fallbackString(stringValue(candidate["task_family"]), "unknown")
		familySource := fallbackString(stringValue(candidate["task_family_source"]), "task_type")
		groupKey := familySourceKey(family, familySource)
		group := ensureFamilyGroup(families, groupKey, family, familySource)

		userOutcomeSignal := mapValue(candidate["user_outcome_signal"])
		userOutcomeStatus := fallbackString(stringValue(userOutcomeSignal["status"]), "unavailable")
		runtimeRecommendation := fallbackString(stringValue(candidate["runtime_recommendation"]), "hold")
		identity := mapValue(candidate["originating_run_artifact_identity"])
		runRef := stringValue(identity["run_ref"])
		feedbackRefsForCandidate := mergeDistinctStrings(
			stringList(userOutcomeSignal["supporting_feedback_refs"]),
			feedbackRefsByRunAndFamily[fmt.Sprintf("%s::%s::%s", runRef, family, familySource)],
		)

		mappedUserOutcome := "unknown"
		switch userOutcomeStatus {
		case "confirmed":
			if runtimeRecommendation == "promote" {
				mappedUserOutcome = "accepted"
			} else {
				mappedUserOutcome = "corrected"
			}
		case "provisional":
			mappedUserOutcome = "unresolved"
		}

		candidateTime := qualityCandidateMarkers[candidateRef]
		latestCandidateAt := stringValue(group["_latest_candidate_at"])
		if candidateTime >= latestCandidateAt {
			group["direct_user_outcome_status"] = mappedUserOutcome
			group["_latest_candidate_at"] = candidateTime
		}

		decisionEntry, hasDecision := reviewDecisionsByCandidate[candidateRef]
		latestFamilyDecision := latestFamilyDecisionByGroup[groupKey]
		shouldExpire := !hasDecision &&
			userOutcomeStatus != "confirmed" &&
			len(feedbackRefsForCandidate) == 0 &&
			latestFamilyDecision != nil &&
			candidateTime <= latestFamilyDecision["timestamp"]

		if !hasDecision && !shouldExpire {
			group["pending_candidate_refs"] = mergeDistinctStrings(anyStringSlice(group["pending_candidate_refs"]), []string{candidateRef})
		}

		dispositionState := "pending"
		if hasDecision {
			dispositionState = fallbackString(stringValue(decisionEntry.value["reviewer_decision"]), "pending")
		} else if shouldExpire {
			dispositionState = "expired"
		}
		reviewWindowStatus := "awaiting_feedback"
		switch dispositionState {
		case "expired":
			reviewWindowStatus = "expired"
		case "pending":
			if len(feedbackRefsForCandidate) > 0 || userOutcomeStatus == "confirmed" {
				reviewWindowStatus = "ready_for_review"
			}
		default:
			reviewWindowStatus = "reviewed"
		}

		disposition := map[string]any{
			"candidate_ref":                       candidateRef,
			"user_outcome_status":                 userOutcomeStatus,
			"appended_user_outcome_feedback_refs": feedbackRefsForCandidate,
			"disposition_state":                   dispositionState,
			"latest_review_decision_ref":          nil,
			"review_window_status":                reviewWindowStatus,
		}
		if hasDecision {
			disposition["latest_review_decision_ref"] = decisionEntry.key
			if ts := stringValue(decisionEntry.value["decision_timestamp"]); ts != "" {
				disposition["latest_review_decision_timestamp"] = ts
			}
		} else if shouldExpire {
			disposition["latest_review_decision_ref"] = latestFamilyDecision["ref"]
			if ts := latestFamilyDecision["timestamp"]; ts != "" {
				disposition["latest_review_decision_timestamp"] = ts
			}
		}
		group["reviewable_candidate_dispositions"] = append(anyMapSlice(group["reviewable_candidate_dispositions"]), disposition)
	}

	for groupKey, refs := range feedbackRefsByFamilyGroup {
		parts := strings.Split(groupKey, "::")
		family := parts[0]
		familySource := "task_type"
		if len(parts) > 1 {
			familySource = parts[1]
		}
		group := ensureFamilyGroup(families, groupKey, family, familySource)
		group["appended_user_outcome_feedback_refs"] = mergeDistinctStrings(anyStringSlice(group["appended_user_outcome_feedback_refs"]), refs)
		if latestFeedback, ok := latestFeedbackByFamilyGroup[groupKey]; ok {
			switch fallbackString(stringValue(latestFeedback.value["feedback_classification"]), "unresolved") {
			case "accepted":
				group["direct_user_outcome_status"] = "accepted"
			case "corrected":
				group["direct_user_outcome_status"] = "corrected"
			case "unresolved":
				group["direct_user_outcome_status"] = "unresolved"
			default:
				group["direct_user_outcome_status"] = "unknown"
			}
		}
	}

	groupKeys := make([]string, 0, len(families))
	for key := range families {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	pendingCandidateGroups := make([]map[string]any, 0, len(groupKeys))
	for _, groupKey := range groupKeys {
		group := families[groupKey]
		pendingRefs := anyStringSlice(group["pending_candidate_refs"])
		sort.Strings(pendingRefs)
		candidateDispositions := anyMapSlice(group["reviewable_candidate_dispositions"])
		sort.Slice(candidateDispositions, func(i, j int) bool {
			return stringValue(candidateDispositions[i]["candidate_ref"]) < stringValue(candidateDispositions[j]["candidate_ref"])
		})
		latestDecision := latestFamilyDecisionByGroup[groupKey]
		hasPending := len(pendingRefs) > 0
		entry := map[string]any{
			"task_family":                         group["task_family"],
			"task_family_source":                  group["task_family_source"],
			"direct_user_outcome_status":          group["direct_user_outcome_status"],
			"last_disposition_state":              "pending",
			"latest_review_decision_ref":          nil,
			"pending_candidate_refs":              pendingRefs,
			"appended_user_outcome_feedback_refs": mergeDistinctStrings(anyStringSlice(group["appended_user_outcome_feedback_refs"]), nil),
			"reviewable_candidate_dispositions":   candidateDispositions,
		}
		if !hasPending && latestDecision != nil {
			entry["last_disposition_state"] = latestDecision["decision"]
			entry["latest_review_decision_ref"] = latestDecision["ref"]
			if latestDecision["timestamp"] != "" {
				entry["latest_review_decision_timestamp"] = latestDecision["timestamp"]
			}
		}
		pendingCandidateGroups = append(pendingCandidateGroups, entry)
	}
	reviewQueue := map[string]any{
		"pending_candidate_groups": pendingCandidateGroups,
		"queue_generated_at": deterministicGeneratedAt(concatStringLists(
			mapValues(qualityCandidateMarkers),
			mapValues(learningReviewDecisionTimes),
			mapValues(userOutcomeFeedbackTimes),
		)),
		"queue_sequence": len(activeQualityCandidates),
	}

	hardeningQueueEntries := []map[string]any{}
	activeHardeningRefs := make([]string, 0, len(activeHardeningCandidates))
	for ref := range activeHardeningCandidates {
		activeHardeningRefs = append(activeHardeningRefs, ref)
	}
	sort.Strings(activeHardeningRefs)
	for _, ref := range activeHardeningRefs {
		candidate := activeHardeningCandidates[ref]
		decisionEntry, ok := hardeningDecisionsByCandidate[ref]
		entry := map[string]any{
			"hardening_candidate_ref":    ref,
			"policy_affecting_reason":    stringValue(candidate["policy_affecting_observation"]),
			"review_state":               "pending",
			"latest_review_decision_ref": nil,
			"task_family":                fallbackString(stringValue(candidate["task_family"]), "unknown"),
			"task_family_source":         fallbackString(stringValue(candidate["task_family_source"]), "task_type"),
		}
		if ok {
			entry["review_state"] = fallbackString(stringValue(decisionEntry.value["reviewer_decision"]), "pending")
			entry["latest_review_decision_ref"] = decisionEntry.key
		}
		hardeningQueueEntries = append(hardeningQueueEntries, entry)
	}
	hardeningQueue := map[string]any{
		"pending_hardening_entries": hardeningQueueEntries,
		"queue_generated_at": deterministicGeneratedAt(concatStringLists(
			mapValues(hardeningCandidateMarkers),
			mapValues(hardeningReviewDecisionTimes),
		)),
		"queue_sequence": len(activeHardeningCandidates),
	}

	latestApprovedMemoryRefsByFamily := map[string]any{}
	for ref, approvedMemory := range approvedMemories {
		family := stringValue(approvedMemory["task_family"])
		if family == "" {
			continue
		}
		familySource := fallbackString(stringValue(approvedMemory["task_family_source"]), "task_type")
		familyKey := familySourceKey(family, familySource)
		freshnessMarker := mapValue(approvedMemory["freshness_marker"])
		recordedAt := approvedMemoryOrderingMarker(approvedMemory)
		existing := mapValue(latestApprovedMemoryRefsByFamily[familyKey])
		if len(existing) == 0 || recordedAt >= stringValue(existing["recorded_at"]) {
			entry := map[string]any{
				"ref":                ref,
				"recorded_at":        recordedAt,
				"lookup_key":         familyKey,
				"task_family":        family,
				"task_family_source": familySource,
			}
			if seq, ok := freshnessMarker["freshness_sequence"]; ok {
				entry["sequence_marker"] = intValue(seq)
			}
			latestApprovedMemoryRefsByFamily[familyKey] = entry
		}
	}

	latestConfirmedSuccessRefsByFamily := map[string]any{}
	latestFailureRefsByFamily := map[string]any{}
	for ref, feedback := range userOutcomeFeedbacks {
		family := stringValue(feedback["task_family"])
		if family == "" {
			continue
		}
		familySource := fallbackString(stringValue(feedback["task_family_source"]), "task_type")
		familyKey := familySourceKey(family, familySource)
		recordedAt := feedbackOrderingMarker(feedback)
		classification := fallbackString(stringValue(feedback["feedback_classification"]), "unresolved")
		switch classification {
		case "accepted":
			existing := mapValue(latestConfirmedSuccessRefsByFamily[familyKey])
			if len(existing) == 0 || recordedAt >= stringValue(existing["recorded_at"]) {
				latestConfirmedSuccessRefsByFamily[familyKey] = evidenceRef(ref, recordedAt, familyKey, family, familySource)
			}
		case "corrected":
			existing := mapValue(latestFailureRefsByFamily[familyKey])
			if len(existing) == 0 || recordedAt >= stringValue(existing["recorded_at"]) {
				latestFailureRefsByFamily[familyKey] = evidenceRef(ref, recordedAt, familyKey, family, familySource)
			}
		}
	}
	for ref, candidate := range activeQualityCandidates {
		family := stringValue(candidate["task_family"])
		if family == "" {
			continue
		}
		familySource := fallbackString(stringValue(candidate["task_family_source"]), "task_type")
		familyKey := familySourceKey(family, familySource)
		userOutcomeSignal := mapValue(candidate["user_outcome_signal"])
		userOutcome := fallbackString(stringValue(userOutcomeSignal["status"]), "unavailable")
		evaluatorSupport := mapValue(candidate["evaluator_support_signal"])
		terminalOutcome := fallbackString(stringValue(evaluatorSupport["terminal_outcome_class"]), "unknown")
		recordedAt := qualityCandidateMarkers[ref]
		if userOutcome == "confirmed" && terminalOutcome == "passed" {
			existing := mapValue(latestConfirmedSuccessRefsByFamily[familyKey])
			if len(existing) == 0 || recordedAt >= stringValue(existing["recorded_at"]) {
				latestConfirmedSuccessRefsByFamily[familyKey] = evidenceRef(ref, recordedAt, familyKey, family, familySource)
			}
		}
	}

	latestReviewDecisionRefsByFamily := map[string]any{}
	latestProvisionalCandidateDispositionsByFamily := map[string]any{}
	for candidateRef, decisionEntry := range reviewDecisionsByCandidate {
		candidate := qualityCandidates[candidateRef]
		family := stringValue(candidate["task_family"])
		if family == "" {
			continue
		}
		familySource := fallbackString(stringValue(candidate["task_family_source"]), "task_type")
		familyKey := familySourceKey(family, familySource)
		recordedAt := stringValue(decisionEntry.value["decision_timestamp"])
		existing := mapValue(latestReviewDecisionRefsByFamily[familyKey])
		if len(existing) == 0 || recordedAt >= stringValue(existing["recorded_at"]) {
			latestReviewDecisionRefsByFamily[familyKey] = evidenceRef(decisionEntry.key, recordedAt, familyKey, family, familySource)
		}
		candidateStatusAtReview := fallbackString(stringValue(decisionEntry.value["candidate_user_outcome_status_at_review"]), "unavailable")
		if candidateStatusAtReview != "confirmed" {
			existing = mapValue(latestProvisionalCandidateDispositionsByFamily[familyKey])
			if len(existing) == 0 || recordedAt >= stringValue(existing["recorded_at"]) {
				latestProvisionalCandidateDispositionsByFamily[familyKey] = map[string]any{
					"ref":                decisionEntry.key,
					"candidate_ref":      candidateRef,
					"recorded_at":        recordedAt,
					"disposition":        fallbackString(stringValue(decisionEntry.value["reviewer_decision"]), "pending"),
					"lookup_key":         familyKey,
					"task_family":        family,
					"task_family_source": familySource,
				}
			}
		}
	}

	familyEvidenceIndex := map[string]any{
		"latest_approved_memory_refs_by_family":               latestApprovedMemoryRefsByFamily,
		"latest_confirmed_success_refs_by_family":             latestConfirmedSuccessRefsByFamily,
		"latest_failure_refs_by_family":                       latestFailureRefsByFamily,
		"latest_review_decision_refs_by_family":               latestReviewDecisionRefsByFamily,
		"latest_provisional_candidate_dispositions_by_family": latestProvisionalCandidateDispositionsByFamily,
		"index_generated_at": deterministicGeneratedAt(concatStringLists(
			mapValues(qualityCandidateMarkers),
			mapValues(learningReviewDecisionTimes),
			mapValues(userOutcomeFeedbackTimes),
			mapValues(approvedMemoryTimes),
		)),
		"index_sequence": len(activeQualityCandidates) + len(approvedMemories),
	}

	return derivedLearningViews{
		reviewQueue:         reviewQueue,
		hardeningQueue:      hardeningQueue,
		familyEvidenceIndex: familyEvidenceIndex,
	}, nil
}

type mapEntry struct {
	key   string
	value map[string]any
}

func loadNamedMaps(projectRoot, directory string, marker func(map[string]any) string) (map[string]map[string]any, map[string]string, error) {
	maps := map[string]map[string]any{}
	markers := map[string]string{}
	files, err := sortedYAMLFilesStrict(directory)
	if err != nil {
		return nil, nil, err
	}
	for _, file := range files {
		ref := filepath.ToSlash(mustRel(projectRoot, file))
		value, err := contracts.ReadYAMLFile(file)
		if err != nil {
			return nil, nil, err
		}
		mapValue, err := contracts.AsMap(value, file)
		if err != nil {
			return nil, nil, err
		}
		maps[ref] = mapValue
		markers[ref] = marker(mapValue)
	}
	return maps, markers, nil
}

func selectActiveCandidates(candidates map[string]map[string]any, markers map[string]string) map[string]map[string]any {
	activeRefs := map[string]string{}
	for ref, candidate := range candidates {
		identity := mapValue(candidate["originating_run_artifact_identity"])
		groupingKey := fmt.Sprintf("%s::%s", stringValue(identity["run_ref"]), candidateLineageKey(candidate))
		recordedAt := markers[ref]
		existingRef := activeRefs[groupingKey]
		if existingRef == "" || recordedAt >= markers[existingRef] {
			activeRefs[groupingKey] = ref
		}
	}

	active := map[string]map[string]any{}
	for _, ref := range activeRefs {
		active[ref] = candidates[ref]
	}
	return active
}

func candidateOrderingMarker(candidate map[string]any) string {
	identity := mapValue(candidate["originating_run_artifact_identity"])
	return strings.Join([]string{
		"quality",
		stringValue(identity["run_ref"]),
		stringValue(identity["artifact_ref"]),
		stringValue(candidate["candidate_identifier"]),
		stableDigest(canonicalJSON(candidate)),
	}, "|")
}

func hardeningCandidateOrderingMarker(candidate map[string]any) string {
	identity := mapValue(candidate["originating_run_artifact_identity"])
	return strings.Join([]string{
		"hardening",
		stringValue(identity["run_ref"]),
		stringValue(identity["artifact_ref"]),
		stringValue(candidate["candidate_identifier"]),
		stableDigest(canonicalJSON(candidate)),
	}, "|")
}

func learningDecisionOrderingMarker(decision map[string]any) string {
	if ts := stringValue(decision["decision_timestamp"]); ts != "" {
		return ts
	}
	return "decision:" + stableDigest(canonicalJSON(decision))
}

func feedbackOrderingMarker(feedback map[string]any) string {
	if ts := stringValue(feedback["submitted_at"]); ts != "" {
		return ts
	}
	return "feedback:" + stableDigest(canonicalJSON(feedback))
}

func approvedMemoryOrderingMarker(approvedMemory map[string]any) string {
	freshnessMarker := mapValue(approvedMemory["freshness_marker"])
	if ts := stringValue(freshnessMarker["refreshed_at"]); ts != "" {
		return ts
	}
	return "approved:" + stableDigest(canonicalJSON(approvedMemory))
}

func canonicalJSON(value any) string {
	normalized := normalizeForJSON(value)
	data, err := json.Marshal(normalized)
	if err != nil {
		return ""
	}
	return string(data)
}

func normalizeForJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		normalized := make(map[string]any, len(typed))
		for _, key := range keys {
			normalized[key] = normalizeForJSON(typed[key])
		}
		return normalized
	case []any:
		result := make([]any, len(typed))
		for i, entry := range typed {
			result[i] = normalizeForJSON(entry)
		}
		return result
	case []string:
		result := make([]any, len(typed))
		for i, entry := range typed {
			result[i] = entry
		}
		return result
	default:
		return typed
	}
}

func stableDigest(input string) string {
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(input))
	return fmt.Sprintf("%08x", hasher.Sum32())
}

func deterministicGeneratedAt(values []string) string {
	filtered := []string{}
	for _, value := range values {
		if value != "" {
			filtered = append(filtered, value)
		}
	}
	sort.Strings(filtered)
	if len(filtered) == 0 {
		return "derived:empty"
	}
	return "derived:" + stableDigest(strings.Join(filtered, "|"))
}

func candidateLineageKey(candidate map[string]any) string {
	identifier := fallbackString(stringValue(candidate["candidate_identifier"]), "candidate")
	atIndex := strings.LastIndex(identifier, "@")
	if atIndex == -1 {
		return identifier
	}
	return identifier[:atIndex]
}

func familySourceKey(taskFamily, taskFamilySource string) string {
	return taskFamily + "::" + taskFamilySource
}

func ensureFamilyGroup(families map[string]map[string]any, groupKey, family, familySource string) map[string]any {
	group, ok := families[groupKey]
	if ok {
		return group
	}
	group = map[string]any{
		"task_family":                         family,
		"task_family_source":                  familySource,
		"direct_user_outcome_status":          "unknown",
		"last_disposition_state":              "pending",
		"pending_candidate_refs":              []string{},
		"appended_user_outcome_feedback_refs": []string{},
		"reviewable_candidate_dispositions":   []map[string]any{},
		"_latest_candidate_at":                "",
	}
	families[groupKey] = group
	return group
}

func mergeDistinctStrings(left, right []string) []string {
	merged := map[string]struct{}{}
	for _, value := range append(append([]string{}, left...), right...) {
		if value != "" {
			merged[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(merged))
	for value := range merged {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func mapValues(source map[string]string) []string {
	values := make([]string, 0, len(source))
	for _, value := range source {
		values = append(values, value)
	}
	return values
}

func concatStringLists(lists ...[]string) []string {
	var result []string
	for _, list := range lists {
		result = append(result, list...)
	}
	return result
}

func evidenceRef(ref, recordedAt, lookupKey, taskFamily, taskFamilySource string) map[string]any {
	return map[string]any{
		"ref":                ref,
		"recorded_at":        recordedAt,
		"lookup_key":         lookupKey,
		"task_family":        taskFamily,
		"task_family_source": taskFamilySource,
	}
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func mapValue(value any) map[string]any {
	if value == nil {
		return map[string]any{}
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
	}
	if typed, ok := value.(map[any]any); ok {
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[fmt.Sprint(key)] = nested
		}
		return normalized
	}
	return map[string]any{}
}

func stringList(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string{}, typed...)
	case []any:
		result := make([]string, 0, len(typed))
		for _, entry := range typed {
			if text, ok := entry.(string); ok {
				result = append(result, text)
			}
		}
		return result
	default:
		return []string{}
	}
}

func anyStringSlice(value any) []string {
	return stringList(value)
}

func anyMapSlice(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return append([]map[string]any{}, typed...)
	case []any:
		result := make([]map[string]any, 0, len(typed))
		for _, entry := range typed {
			if mapEntry, ok := entry.(map[string]any); ok {
				result = append(result, mapEntry)
			}
		}
		return result
	default:
		return []map[string]any{}
	}
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}
