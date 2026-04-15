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

var allowedRiskTolerance = map[string]struct{}{
	"low":    {},
	"medium": {},
	"high":   {},
}

func NormalizeDraft(draft Draft) (MaterializedRequest, error) {
	requestVersion := normalizedRequestVersion(draft.RequestVersion)
	if requestVersion != defaultRequestVersion {
		return MaterializedRequest{}, fmt.Errorf("unsupported request_version: %s", requestVersion)
	}

	taskType := strings.ToLower(strings.TrimSpace(draft.TaskType))
	if taskType == "" {
		return MaterializedRequest{}, fmt.Errorf("task_type is required")
	}

	defaultRiskTolerance, ok := defaultRiskToleranceByTaskType[taskType]
	if !ok {
		return MaterializedRequest{}, fmt.Errorf("unsupported task_type: %s", taskType)
	}

	goal := strings.TrimSpace(draft.Goal)
	if goal == "" {
		return MaterializedRequest{}, fmt.Errorf("goal is required")
	}

	projectRoot := strings.TrimSpace(draft.ProjectRoot)
	if projectRoot == "" {
		return MaterializedRequest{}, fmt.Errorf("project_root is required")
	}
	if !filepath.IsAbs(projectRoot) {
		return MaterializedRequest{}, fmt.Errorf("project_root must be an absolute path")
	}

	riskTolerance := strings.ToLower(strings.TrimSpace(draft.RiskTolerance))
	if riskTolerance == "" {
		riskTolerance = defaultRiskTolerance
	}
	if _, ok := allowedRiskTolerance[riskTolerance]; !ok {
		return MaterializedRequest{}, fmt.Errorf("unsupported risk_tolerance: %s", riskTolerance)
	}

	return MaterializedRequest{
		ProjectRoot: projectRoot,
		Request: CanonicalRequest{
			TaskType: taskType,
			Goal:     goal,
			Context: RequestContext{
				Feature:           strings.TrimSpace(draft.Context.Feature),
				SuspectedFiles:    normalizeStrings(draft.Context.SuspectedFiles),
				RelatedFiles:      normalizeStrings(draft.Context.RelatedFiles),
				ValidationRoots:   normalizeStrings(draft.Context.ValidationRoots),
				ValidationTargets: normalizeStrings(draft.Context.ValidationTargets),
			},
			Constraints:       normalizeStrings(draft.Constraints),
			DefinitionOfDone:  normalizeStrings(draft.DefinitionOfDone),
			Priority:          "medium",
			RiskTolerance:     riskTolerance,
			ValidationProfile: "standard",
		},
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
