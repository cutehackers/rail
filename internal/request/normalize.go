package request

import (
	"fmt"
	"path/filepath"
	"strings"
)

const defaultRequestVersion = "1"

var defaultRiskToleranceByTaskType = map[string]string{
	"bug_fix":          "low",
	"feature_addition": "low",
	"test_repair":      "low",
	"safe_refactor":    "medium",
}

func NormalizeDraft(draft Draft, currentWorkingDir string) (Draft, error) {
	taskType := strings.ToLower(strings.TrimSpace(draft.TaskType))
	if taskType == "" {
		return Draft{}, fmt.Errorf("task_type is required")
	}

	defaultRiskTolerance, ok := defaultRiskToleranceByTaskType[taskType]
	if !ok {
		return Draft{}, fmt.Errorf("unsupported task_type: %s", taskType)
	}

	goal := strings.TrimSpace(draft.Goal)
	if goal == "" {
		return Draft{}, fmt.Errorf("goal is required")
	}

	projectRoot := strings.TrimSpace(draft.ProjectRoot)
	if projectRoot == "" {
		projectRoot = currentWorkingDir
	}
	if !filepath.IsAbs(projectRoot) {
		projectRoot = filepath.Join(currentWorkingDir, projectRoot)
	}

	absoluteProjectRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return Draft{}, fmt.Errorf("resolve project_root: %w", err)
	}

	riskTolerance := strings.ToLower(strings.TrimSpace(draft.RiskTolerance))
	if riskTolerance == "" {
		riskTolerance = defaultRiskTolerance
	}

	return Draft{
		RequestVersion:   normalizedRequestVersion(draft.RequestVersion),
		ProjectRoot:      filepath.Clean(absoluteProjectRoot),
		TaskType:         taskType,
		Goal:             goal,
		Context:          normalizeStrings(draft.Context),
		Constraints:      normalizeStrings(draft.Constraints),
		DefinitionOfDone: normalizeStrings(draft.DefinitionOfDone),
		RiskTolerance:    riskTolerance,
	}, nil
}

func RequestPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".harness", "requests", "request.yaml")
}

func normalizedRequestVersion(value string) string {
	version := strings.TrimSpace(value)
	if version == "" {
		return defaultRequestVersion
	}
	return version
}

func normalizeStrings(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized = append(normalized, trimmed)
	}
	return normalized
}
