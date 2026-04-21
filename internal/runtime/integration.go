package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rail/internal/contracts"
)

func (r *Runner) Integrate(artifactPath string, projectRootOverride string) (string, error) {
	artifactDirectory, err := r.router.resolveArtifactDirectory(artifactPath)
	if err != nil {
		return "", err
	}

	workflow, err := readWorkflow(filepath.Join(artifactDirectory, "workflow.json"))
	if err != nil {
		return "", err
	}
	state, err := readState(filepath.Join(artifactDirectory, "state.json"))
	if err != nil {
		return "", err
	}

	validator, err := contracts.NewValidator(r.projectRoot)
	if err != nil {
		return "", err
	}
	evaluationMap, err := validator.ValidateArtifactFile(filepath.Join(artifactDirectory, "evaluation_result.yaml"), "evaluation_result")
	if err != nil {
		return "", err
	}
	executionMap, err := validator.ValidateArtifactFile(filepath.Join(artifactDirectory, "execution_report.yaml"), "execution_report")
	if err != nil {
		return "", err
	}

	decision, err := readString(evaluationMap, "decision")
	if err != nil {
		return "", err
	}
	if decision != "pass" {
		return "", fmt.Errorf("integrator requires evaluator decision `pass`, but found `%s`", decision)
	}
	if state.Status != "passed" && state.Status != "awaiting_integrator" {
		return "", fmt.Errorf("integrator requires a terminal passed state, but found `%s`", state.Status)
	}

	effectiveProjectRoot := workflow.ProjectRoot
	if strings.TrimSpace(projectRootOverride) != "" {
		effectiveProjectRoot = projectRootOverride
	}
	workingDirectory, err := filepath.Abs(effectiveProjectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve integration project root: %w", err)
	}
	actorProfiles, err := loadActorProfiles(workingDirectory, []string{"integrator"})
	if err != nil {
		return "", fmt.Errorf("load actor profiles: %w", err)
	}
	integratorProfile, err := actorProfiles.ProfileFor("integrator")
	if err != nil {
		return "", err
	}

	executionPlan, err := readExecutionPlanFile(filepath.Join(artifactDirectory, "execution_plan.json"))
	if err != nil {
		return "", err
	}
	contextContractMap, err := r.bootstrapper.loadMap(".harness/supervisor/context_contract.yaml")
	if err != nil {
		return "", err
	}
	contextContract, err := contextContractFromMap(contextContractMap)
	if err != nil {
		return "", err
	}
	actorContract, err := contextContract.contractFor("integrator")
	if err != nil {
		return "", err
	}
	actorInstructions, err := r.bootstrapper.loadTextAsset(".harness/actors/integrator.md")
	if err != nil {
		return "", err
	}

	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.MkdirAll(runsDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create runs directory: %w", err)
	}
	briefDirectory := filepath.Join(artifactDirectory, "actor_briefs")
	if err := os.MkdirAll(briefDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create actor brief directory: %w", err)
	}

	artifactLabel, err := filepath.Rel(r.projectRoot, artifactDirectory)
	if err != nil {
		return "", fmt.Errorf("relativize artifact directory: %w", err)
	}
	outputPath := filepath.Join(artifactDirectory, "integration_result.yaml")
	outputLabel, err := filepath.Rel(r.projectRoot, outputPath)
	if err != nil {
		return "", fmt.Errorf("relativize integration result path: %w", err)
	}

	actorIndex := len(workflow.Actors)
	briefPath := filepath.Join(briefDirectory, fmt.Sprintf("%02d_integrator.md", actorIndex+1))
	if err := os.WriteFile(briefPath, []byte(buildActorBrief(
		"integrator",
		actorInstructions,
		workflow,
		executionPlan,
		actorContract,
		artifactDirectory,
		materializedInputs{
			ArchitectureRulesPath: filepath.ToSlash(filepath.Join("inputs", "architecture_rules.md")),
			ProjectRulesPath:      filepath.ToSlash(filepath.Join("inputs", "project_rules.md")),
			ForbiddenChangesPath:  filepath.ToSlash(filepath.Join("inputs", "forbidden_changes.md")),
			ExecutionPolicyPath:   filepath.ToSlash(filepath.Join("inputs", "execution_policy.yaml")),
			RubricPath:            filepath.ToSlash(filepath.Join("inputs", "rubric.yaml")),
			RequestPath:           "request.yaml",
		},
	)), 0o644); err != nil {
		return "", fmt.Errorf("write integrator brief: %w", err)
	}

	logPath := filepath.Join(runsDirectory, fmt.Sprintf("%02d_integrator-last-message.txt", actorIndex+1))
	schemaPath, err := materializeOutputSchema(runsDirectory, actorIndex)
	if err != nil {
		return "", err
	}
	responseObject, err := runIntegratorActor(workingDirectory, integratorProfile, briefPath, artifactDirectory, outputPath, logPath, schemaPath)
	if err != nil {
		return "", err
	}

	integrationResult := normalizeIntegrationResult(responseObject, evaluationMap, executionMap)
	if err := writeYAML(outputPath, integrationResult); err != nil {
		return "", err
	}
	if _, err := validator.ValidateArtifactFile(outputPath, "integration_result"); err != nil {
		return "", err
	}

	return fmt.Sprintf(
		"Harness integration completed at %s (decision=pass preserved, output=%s)",
		filepath.ToSlash(artifactLabel),
		filepath.ToSlash(outputLabel),
	), nil
}

func materializeOutputSchema(runsDirectory string, actorIndex int) (string, error) {
	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"summary",
			"files_changed",
			"validation",
			"risks",
			"follow_up",
			"evidence_quality",
			"release_readiness",
			"blocking_issues",
		},
		"properties": map[string]any{
			"summary": map[string]any{"type": "string"},
			"files_changed": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
			"validation": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"check_name", "status", "evidence", "command", "details"},
					"properties": map[string]any{
						"check_name": map[string]any{"type": "string"},
						"status": map[string]any{
							"type": "string",
							"enum": []string{"pass", "fail", "warning", "blocked", "not_run"},
						},
						"evidence": map[string]any{"type": "string"},
						"command": map[string]any{
							"type": []string{"string", "null"},
						},
						"details": map[string]any{
							"type": []string{"string", "null"},
						},
					},
				},
			},
			"risks": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"description", "severity", "mitigation"},
					"properties": map[string]any{
						"description": map[string]any{"type": "string"},
						"severity": map[string]any{
							"type": "string",
							"enum": []string{"low", "medium", "high", "critical"},
						},
						"mitigation": map[string]any{
							"type": []string{"string", "null"},
						},
					},
				},
			},
			"follow_up": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required":             []string{"action", "owner", "due", "notes"},
					"properties": map[string]any{
						"action": map[string]any{"type": "string"},
						"owner":  map[string]any{"type": "string"},
						"due": map[string]any{
							"type": []string{"string", "null"},
						},
						"notes": map[string]any{
							"type": []string{"string", "null"},
						},
					},
				},
			},
			"evidence_quality": map[string]any{
				"type": "string",
				"enum": []string{"draft", "adequate", "high_confidence"},
			},
			"release_readiness": map[string]any{
				"type": "string",
				"enum": []string{"ready", "conditional", "blocked"},
			},
			"blocking_issues": map[string]any{
				"type":  "array",
				"items": map[string]any{"type": "string"},
			},
		},
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal output schema: %w", err)
	}
	targetPath := filepath.Join(runsDirectory, fmt.Sprintf("%02d_integrator-output-schema.json", actorIndex+1))
	if err := os.WriteFile(targetPath, data, 0o644); err != nil {
		return "", fmt.Errorf("write output schema: %w", err)
	}
	return targetPath, nil
}

func runIntegratorActor(
	workingDirectory string,
	profile ActorProfile,
	briefPath string,
	artifactDirectory string,
	outputPath string,
	logPath string,
	schemaPath string,
) (map[string]any, error) {
	prompt := strings.Join([]string{
		"Read the Rail integrator brief and produce only the schema-valid post-pass handoff output.",
		"Actor name: integrator",
		"Actor brief: " + briefPath,
		"Artifact directory: " + artifactDirectory,
		"Project root: " + workingDirectory,
		"Write no files yourself except the schema-valid response via Codex output handling.",
	}, "\n")
	return runCommand("integrator", profile, workingDirectory, prompt, logPath, schemaPath)
}

func normalizeIntegrationResult(
	responseObject map[string]any,
	evaluationMap map[string]any,
	executionMap map[string]any,
) map[string]any {
	validationEntries := normalizeIntegrationValidationEntries(responseObject["validation"], executionMap)
	riskEntries := normalizeIntegrationRiskEntries(responseObject["risks"])
	followUpEntries := normalizeIntegrationFollowUpEntries(responseObject["follow_up"])
	blockingIssues := normalizeIntegrationBlockingIssues(
		responseObject["release_readiness"],
		responseObject["blocking_issues"],
		validationEntries,
		riskEntries,
	)
	return map[string]any{
		"summary":           strings.TrimSpace(stringValue(responseObject["summary"])),
		"files_changed":     normalizeLooseStringList(responseObject["files_changed"]),
		"validation":        validationEntries,
		"risks":             riskEntries,
		"follow_up":         followUpEntries,
		"evidence_quality":  normalizeIntegrationEvidenceQuality(responseObject["evidence_quality"], evaluationMap, validationEntries),
		"release_readiness": normalizeIntegrationReleaseReadiness(responseObject["release_readiness"], validationEntries, riskEntries, blockingIssues),
		"blocking_issues":   blockingIssues,
	}
}

func normalizeIntegrationValidationEntries(rawValue any, executionMap map[string]any) []map[string]any {
	entries := []map[string]any{}
	if rawList, ok := rawValue.([]any); ok {
		for index, item := range rawList {
			switch typed := item.(type) {
			case string:
				evidence := strings.TrimSpace(typed)
				if evidence == "" {
					continue
				}
				entries = append(entries, map[string]any{
					"check_name": fmt.Sprintf("reported_validation_%d", index+1),
					"status":     "pass",
					"evidence":   evidence,
					"command":    nil,
					"details":    nil,
				})
			case map[string]any:
				checkName := strings.TrimSpace(stringValue(typed["check_name"]))
				if checkName == "" {
					checkName = fmt.Sprintf("validation_check_%d", index+1)
				}
				evidence := strings.TrimSpace(stringValue(typed["evidence"]))
				if evidence == "" {
					evidence = strings.TrimSpace(stringValue(typed["details"]))
				}
				if evidence == "" {
					evidence = "No evidence provided."
				}
				entry := map[string]any{
					"check_name": checkName,
					"status":     normalizeValidationStatus(stringValue(typed["status"])),
					"evidence":   evidence,
					"command":    nil,
					"details":    nil,
				}
				if command := strings.TrimSpace(stringValue(typed["command"])); command != "" {
					entry["command"] = command
				}
				if details := strings.TrimSpace(stringValue(typed["details"])); details != "" {
					entry["details"] = details
				}
				entries = append(entries, entry)
			}
		}
	}

	existingNames := map[string]struct{}{}
	for _, entry := range entries {
		existingNames[stringValue(entry["check_name"])] = struct{}{}
	}
	addDerivedEntry := func(checkName, status, evidence string) {
		if _, exists := existingNames[checkName]; exists {
			return
		}
		entries = append(entries, map[string]any{
			"check_name": checkName,
			"status":     status,
			"evidence":   evidence,
			"command":    nil,
			"details":    nil,
		})
		existingNames[checkName] = struct{}{}
	}

	if formatResult := strings.TrimSpace(stringValue(executionMap["format"])); formatResult != "" {
		addDerivedEntry("format", normalizeValidationStatus(formatResult), fmt.Sprintf("Formatting checks reported `%s` in execution_report.", formatResult))
	}
	if analyzeResult := strings.TrimSpace(stringValue(executionMap["analyze"])); analyzeResult != "" {
		addDerivedEntry("analyze", normalizeValidationStatus(analyzeResult), fmt.Sprintf("Static analysis reported `%s` in execution_report.", analyzeResult))
	}
	testsMap := mapValue(executionMap["tests"])
	if len(testsMap) > 0 {
		total := intValue(testsMap["total"])
		passed := intValue(testsMap["passed"])
		failed := intValue(testsMap["failed"])
		status := "pass"
		evidence := fmt.Sprintf("Automated tests passed (%d/%d).", passed, total)
		if total == 0 {
			status = "not_run"
			evidence = "No automated tests were executed according to execution_report."
		} else if failed > 0 {
			status = "fail"
			evidence = fmt.Sprintf("Automated tests failed (%d/%d).", failed, total)
		}
		addDerivedEntry("tests", status, evidence)
	}

	return entries
}

func normalizeIntegrationRiskEntries(rawValue any) []map[string]any {
	rawList, ok := rawValue.([]any)
	if !ok {
		return []map[string]any{}
	}
	entries := []map[string]any{}
	for _, item := range rawList {
		switch typed := item.(type) {
		case string:
			description := strings.TrimSpace(typed)
			if description == "" {
				continue
			}
			entries = append(entries, map[string]any{"description": description, "severity": "medium", "mitigation": nil})
		case map[string]any:
			description := strings.TrimSpace(stringValue(typed["description"]))
			if description == "" {
				continue
			}
			entry := map[string]any{
				"description": description,
				"severity":    normalizeRiskSeverity(stringValue(typed["severity"])),
				"mitigation":  nil,
			}
			if mitigation := strings.TrimSpace(stringValue(typed["mitigation"])); mitigation != "" {
				entry["mitigation"] = mitigation
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

func normalizeIntegrationFollowUpEntries(rawValue any) []map[string]any {
	rawList, ok := rawValue.([]any)
	if !ok {
		return []map[string]any{}
	}
	entries := []map[string]any{}
	for _, item := range rawList {
		switch typed := item.(type) {
		case string:
			action := strings.TrimSpace(typed)
			if action == "" {
				continue
			}
			entries = append(entries, map[string]any{"action": action, "owner": "operator", "due": nil, "notes": nil})
		case map[string]any:
			action := strings.TrimSpace(stringValue(typed["action"]))
			if action == "" {
				continue
			}
			entry := map[string]any{
				"action": action,
				"owner":  "operator",
				"due":    nil,
				"notes":  nil,
			}
			if owner := strings.TrimSpace(stringValue(typed["owner"])); owner != "" {
				entry["owner"] = owner
			}
			if due := strings.TrimSpace(stringValue(typed["due"])); due != "" {
				entry["due"] = due
			}
			if notes := strings.TrimSpace(stringValue(typed["notes"])); notes != "" {
				entry["notes"] = notes
			}
			entries = append(entries, entry)
		}
	}
	return entries
}

func normalizeIntegrationBlockingIssues(
	requestedReleaseReadiness any,
	requested any,
	validationEntries []map[string]any,
	riskEntries []map[string]any,
) []string {
	derived := normalizeLooseStringList(requested)
	for _, entry := range validationEntries {
		status := stringValue(entry["status"])
		checkName := fallbackString(stringValue(entry["check_name"]), "unknown")
		if status == "fail" || status == "blocked" {
			derived = append(derived, fmt.Sprintf("Validation check `%s` reported `%s`.", checkName, status))
		}
	}
	for _, entry := range riskEntries {
		if stringValue(entry["severity"]) == "critical" {
			if description := stringValue(entry["description"]); description != "" {
				derived = append(derived, description)
			}
		}
	}
	if strings.TrimSpace(stringValue(requestedReleaseReadiness)) != "blocked" {
		// only explicit blocked or derived blocking evidence should retain issues
	}
	return mergeDistinctStrings(derived, nil)
}

func normalizeIntegrationEvidenceQuality(requested any, evaluationMap map[string]any, validationEntries []map[string]any) string {
	requestedValue := strings.TrimSpace(stringValue(requested))
	switch requestedValue {
	case "draft", "adequate", "high_confidence":
		return requestedValue
	}
	for _, entry := range validationEntries {
		status := stringValue(entry["status"])
		if status == "fail" || status == "blocked" {
			return "draft"
		}
	}
	switch strings.TrimSpace(stringValue(evaluationMap["quality_confidence"])) {
	case "high":
		return "high_confidence"
	case "medium":
		return "adequate"
	case "low":
		return "draft"
	default:
		if len(validationEntries) == 0 {
			return "draft"
		}
		return "adequate"
	}
}

func normalizeIntegrationReleaseReadiness(
	requested any,
	validationEntries []map[string]any,
	riskEntries []map[string]any,
	blockingIssues []string,
) string {
	derived := deriveIntegrationReleaseReadiness(validationEntries, riskEntries, blockingIssues)
	requestedValue := strings.TrimSpace(stringValue(requested))
	switch requestedValue {
	case "ready", "conditional", "blocked":
		if derived == "blocked" {
			return "blocked"
		}
		if derived == "conditional" && requestedValue == "ready" {
			return "conditional"
		}
		return requestedValue
	default:
		return derived
	}
}

func deriveIntegrationReleaseReadiness(validationEntries []map[string]any, riskEntries []map[string]any, blockingIssues []string) string {
	if len(blockingIssues) > 0 {
		return "blocked"
	}
	hasWarningValidation := false
	for _, entry := range validationEntries {
		status := stringValue(entry["status"])
		if status == "warning" || status == "not_run" {
			hasWarningValidation = true
			break
		}
	}
	hasHighRisk := false
	for _, entry := range riskEntries {
		if stringValue(entry["severity"]) == "high" {
			hasHighRisk = true
			break
		}
	}
	if hasWarningValidation || hasHighRisk {
		return "conditional"
	}
	return "ready"
}

func normalizeValidationStatus(rawValue string) string {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case "pass", "passed":
		return "pass"
	case "fail", "failed":
		return "fail"
	case "blocked":
		return "blocked"
	case "not_run", "not run", "skipped":
		return "not_run"
	case "warning", "warn":
		return "warning"
	default:
		return "warning"
	}
}

func normalizeRiskSeverity(rawValue string) string {
	switch strings.ToLower(strings.TrimSpace(rawValue)) {
	case "low":
		return "low"
	case "high":
		return "high"
	case "critical":
		return "critical"
	default:
		return "medium"
	}
}

func normalizeLooseStringList(rawValue any) []string {
	switch typed := rawValue.(type) {
	case []string:
		return mergeDistinctStrings(typed, nil)
	case []any:
		values := []string{}
		for _, item := range typed {
			if text, ok := item.(string); ok {
				text = strings.TrimSpace(text)
				if text != "" {
					values = append(values, text)
				}
			}
		}
		return mergeDistinctStrings(values, nil)
	default:
		return []string{}
	}
}
