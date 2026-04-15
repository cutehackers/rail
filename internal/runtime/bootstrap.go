package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rail/internal/assets"
	"rail/internal/contracts"
	"rail/internal/request"

	"gopkg.in/yaml.v3"
)

type Bootstrapper struct {
	projectRoot string
	validator   *contracts.Validator
}

func NewBootstrapper(projectRoot string) (*Bootstrapper, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}

	validator, err := contracts.NewValidator(root)
	if err != nil {
		return nil, err
	}
	return &Bootstrapper{projectRoot: root, validator: validator}, nil
}

func (b *Bootstrapper) Bootstrap(requestPath, taskID string) (string, error) {
	if strings.TrimSpace(taskID) == "" {
		return "", errors.New("task id is required")
	}

	requestValue, err := b.validator.ValidateRequestFile(requestPath)
	if err != nil {
		return "", err
	}

	registry, err := b.loadMap(".harness/supervisor/registry.yaml")
	if err != nil {
		return "", err
	}
	policy, err := b.loadMap(".harness/supervisor/policy.yaml")
	if err != nil {
		return "", err
	}
	executionPolicy, err := b.loadMap(".harness/supervisor/execution_policy.yaml")
	if err != nil {
		return "", err
	}

	resolvedWorkflow, err := buildResolvedWorkflow(
		b.projectRoot,
		requestPath,
		taskID,
		requestValue,
		registry,
		policy,
	)
	if err != nil {
		return "", err
	}

	artifactRoot, err := readStringFromMap(executionPolicy, "artifacts", "root")
	if err != nil {
		return "", err
	}
	artifactDirectory := filepath.Join(b.projectRoot, filepath.FromSlash(artifactRoot), taskID)
	if err := os.MkdirAll(artifactDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create artifact directory: %w", err)
	}

	requestBytes, err := os.ReadFile(requestPath)
	if err != nil {
		return "", fmt.Errorf("read request file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDirectory, "request.yaml"), requestBytes, 0o644); err != nil {
		return "", fmt.Errorf("write request snapshot: %w", err)
	}

	if err := writeJSON(filepath.Join(artifactDirectory, "resolved_workflow.json"), resolvedWorkflow); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(artifactDirectory, "state.json"), initialState(resolvedWorkflow)); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(artifactDirectory, "execution_plan.json"), map[string]any{
		"formatCommand":   nil,
		"analyzeCommands": []string{},
		"testCommands":    []string{},
	}); err != nil {
		return "", err
	}

	for _, output := range resolvedWorkflow.RequiredOutputs {
		if err := writeYAML(filepath.Join(artifactDirectory, artifactFileName(output)), placeholderContent(output)); err != nil {
			return "", err
		}
	}

	return artifactDirectory, nil
}

type ResolvedWorkflow struct {
	TaskID                  string   `json:"taskId"`
	TaskType                string   `json:"taskType"`
	TaskFamily              string   `json:"taskFamily"`
	TaskFamilySource        string   `json:"taskFamilySource"`
	ProjectRoot             string   `json:"projectRoot"`
	Actors                  []string `json:"actors"`
	RubricPath              string   `json:"rubricPath"`
	GeneratorRetryBudget    int      `json:"generatorRetryBudget"`
	ContextRebuildBudget    int      `json:"contextRebuildBudget"`
	ValidationTightenBudget int      `json:"validationTightenBudget"`
	ChangedFileHints        []string `json:"changedFileHints"`
	InferredTestTargets     []string `json:"inferredTestTargets"`
	RequiredOutputs         []string `json:"requiredOutputs"`
	RequestPath             string   `json:"requestPath"`
	TerminationConditions   []string `json:"terminationConditions"`
	PassIf                  []string `json:"passIf"`
	ReviseIf                []string `json:"reviseIf"`
	RejectIf                []string `json:"rejectIf"`
}

type State struct {
	TaskID                             string   `json:"taskId"`
	TaskFamily                         string   `json:"taskFamily"`
	TaskFamilySource                   string   `json:"taskFamilySource"`
	Status                             string   `json:"status"`
	CurrentActor                       *string  `json:"currentActor"`
	CompletedActors                    []string `json:"completedActors"`
	GeneratorRetriesRemaining          int      `json:"generatorRetriesRemaining"`
	ContextRebuildsRemaining           int      `json:"contextRebuildsRemaining"`
	ValidationTighteningsRemaining     int      `json:"validationTighteningsRemaining"`
	LastDecision                       *string  `json:"lastDecision"`
	LastReasonCodes                    []string `json:"lastReasonCodes"`
	ActionHistory                      []string `json:"actionHistory"`
	GeneratorRevisionsUsed             int      `json:"generatorRevisionsUsed"`
	ContextRefreshCount                int      `json:"contextRefreshCount"`
	LastContextRefreshTrigger          *string  `json:"lastContextRefreshTrigger"`
	LastContextRefreshReasonFamily     *string  `json:"lastContextRefreshReasonFamily"`
	LastInterventionTriggerReasonCodes []string `json:"lastInterventionTriggerReasonCodes"`
	LastInterventionTriggerCategory    *string  `json:"lastInterventionTriggerCategory"`
	PendingContextRefreshTrigger       *string  `json:"pendingContextRefreshTrigger"`
	PendingContextRefreshReasonFamily  *string  `json:"pendingContextRefreshReasonFamily"`
	ValidationTighteningsUsed          int      `json:"validationTighteningsUsed"`
}

func buildResolvedWorkflow(
	projectRoot string,
	requestPath string,
	taskID string,
	requestValue request.CanonicalRequest,
	registry map[string]any,
	policy map[string]any,
) (ResolvedWorkflow, error) {
	taskRegistry, err := readMap(registry, "task_registry")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	taskConfig, err := readMap(taskRegistry, requestValue.TaskType)
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	actors, err := readStringList(taskConfig, "actors")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	requiredOutputs, err := readStringList(taskConfig, "required_output")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	rubricPath, err := readString(taskConfig, "rubric")
	if err != nil {
		return ResolvedWorkflow{}, err
	}

	retryRules, err := readMap(policy, "retry_rules")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	riskRule, err := readMap(retryRules, requestValue.RiskTolerance)
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	generatorRetryBudget, err := readInt(riskRule, "max_generator_retry")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	supervisorLoop, err := readMap(policy, "supervisor_loop")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	contextBudget, err := readInt(supervisorLoop, "max_context_rebuild")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	validationBudget, err := readInt(supervisorLoop, "max_validation_tighten")
	if err != nil {
		return ResolvedWorkflow{}, err
	}

	passIf, err := readStringList(policy, "pass_if")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	reviseIf, err := readStringList(policy, "revise_if")
	if err != nil {
		return ResolvedWorkflow{}, err
	}
	rejectIf, err := readStringList(policy, "reject_if")
	if err != nil {
		return ResolvedWorkflow{}, err
	}

	return ResolvedWorkflow{
		TaskID:                  taskID,
		TaskType:                requestValue.TaskType,
		TaskFamily:              requestValue.TaskType,
		TaskFamilySource:        "task_type",
		ProjectRoot:             projectRoot,
		Actors:                  actors,
		RubricPath:              rubricPath,
		GeneratorRetryBudget:    generatorRetryBudget,
		ContextRebuildBudget:    contextBudget,
		ValidationTightenBudget: validationBudget,
		ChangedFileHints:        []string{},
		InferredTestTargets:     []string{},
		RequiredOutputs:         requiredOutputs,
		RequestPath:             filepath.ToSlash(requestPath),
		TerminationConditions: []string{
			"evaluator_decision == reject",
			"retries_exhausted == true",
			"evaluator_decision == pass",
		},
		PassIf:   passIf,
		ReviseIf: reviseIf,
		RejectIf: rejectIf,
	}, nil
}

func initialState(workflow ResolvedWorkflow) State {
	var currentActor *string
	if len(workflow.Actors) > 0 {
		currentActor = &workflow.Actors[0]
	}
	return State{
		TaskID:                             workflow.TaskID,
		TaskFamily:                         workflow.TaskFamily,
		TaskFamilySource:                   workflow.TaskFamilySource,
		Status:                             "initialized",
		CurrentActor:                       currentActor,
		CompletedActors:                    []string{},
		GeneratorRetriesRemaining:          workflow.GeneratorRetryBudget,
		ContextRebuildsRemaining:           workflow.ContextRebuildBudget,
		ValidationTighteningsRemaining:     workflow.ValidationTightenBudget,
		LastReasonCodes:                    []string{},
		ActionHistory:                      []string{},
		LastInterventionTriggerReasonCodes: []string{},
	}
}

func placeholderContent(outputName string) map[string]any {
	switch outputName {
	case "plan":
		return map[string]any{
			"summary":                     "",
			"likely_files":                []string{},
			"assumptions":                 []string{},
			"substeps":                    []string{},
			"risks":                       []string{},
			"acceptance_criteria_refined": []string{},
		}
	case "context_pack":
		return map[string]any{
			"relevant_files":        []map[string]string{},
			"repo_patterns":         []string{},
			"test_patterns":         []string{},
			"forbidden_changes":     []string{},
			"implementation_hints":  []string{},
			"approved_memory_hints": []map[string]any{},
		}
	case "implementation_result":
		return map[string]any{
			"changed_files":          []string{},
			"patch_summary":          []string{},
			"tests_added_or_updated": []string{},
			"known_limits":           []string{},
		}
	case "execution_report":
		return map[string]any{
			"format":          "fail",
			"analyze":         "fail",
			"tests":           map[string]any{"total": 0, "passed": 0, "failed": 0},
			"failure_details": []string{"bootstrap placeholder"},
			"logs":            []string{},
		}
	case "evaluation_result":
		return map[string]any{
			"decision":           "revise",
			"scores":             map[string]any{"requirements": 0, "architecture": 0, "regression_risk": 0},
			"quality_confidence": "low",
			"findings":           []string{"bootstrap placeholder"},
			"reason_codes":       []string{"bootstrap_placeholder"},
			"next_action":        "revise_generator",
		}
	default:
		return map[string]any{}
	}
}

func artifactFileName(outputName string) string {
	switch outputName {
	case "plan":
		return "plan.yaml"
	case "context_pack":
		return "context_pack.yaml"
	case "implementation_result":
		return "implementation_result.yaml"
	case "execution_report":
		return "execution_report.yaml"
	case "evaluation_result":
		return "evaluation_result.yaml"
	default:
		return outputName + ".yaml"
	}
}

func (b *Bootstrapper) loadMap(relPath string) (map[string]any, error) {
	data, _, err := assets.Resolve(b.projectRoot, relPath)
	if err != nil {
		return nil, fmt.Errorf("load %s: %w", relPath, err)
	}

	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("decode %s: %w", relPath, err)
	}
	value, err := runtimeAsMap(runtimeNormalizeYAMLValue(raw), relPath)
	if err != nil {
		return nil, err
	}
	return value, nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal json %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write json %s: %w", path, err)
	}
	return nil
}

func writeYAML(path string, value any) error {
	data, err := yaml.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal yaml %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write yaml %s: %w", path, err)
	}
	return nil
}

func readMap(source map[string]any, key string) (map[string]any, error) {
	value, ok := source[key]
	if !ok {
		return nil, fmt.Errorf("missing `%s`", key)
	}
	mapValue, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected `%s` to be a map", key)
	}
	return mapValue, nil
}

func readString(source map[string]any, key string) (string, error) {
	value, ok := source[key]
	if !ok {
		return "", fmt.Errorf("missing `%s`", key)
	}
	stringValue, ok := value.(string)
	if !ok || strings.TrimSpace(stringValue) == "" {
		return "", fmt.Errorf("expected `%s` to be a non-empty string", key)
	}
	return stringValue, nil
}

func readStringFromMap(source map[string]any, key, nested string) (string, error) {
	mapValue, err := readMap(source, key)
	if err != nil {
		return "", err
	}
	return readString(mapValue, nested)
}

func readStringList(source map[string]any, key string) ([]string, error) {
	value, ok := source[key]
	if !ok {
		return nil, fmt.Errorf("missing `%s`", key)
	}
	listValue, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected `%s` to be a list", key)
	}
	result := make([]string, 0, len(listValue))
	for _, entry := range listValue {
		text, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("expected `%s` entries to be strings", key)
		}
		result = append(result, text)
	}
	return result, nil
}

func readInt(source map[string]any, key string) (int, error) {
	value, ok := source[key]
	if !ok {
		return 0, fmt.Errorf("missing `%s`", key)
	}
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	default:
		return 0, fmt.Errorf("expected `%s` to be an integer", key)
	}
}

func runtimeNormalizeYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[key] = runtimeNormalizeYAMLValue(nested)
		}
		return normalized
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, nested := range typed {
			normalized[fmt.Sprint(key)] = runtimeNormalizeYAMLValue(nested)
		}
		return normalized
	case []any:
		normalized := make([]any, len(typed))
		for i, nested := range typed {
			normalized[i] = runtimeNormalizeYAMLValue(nested)
		}
		return normalized
	default:
		return value
	}
}

func runtimeAsMap(value any, context string) (map[string]any, error) {
	mapValue, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected object for %s", context)
	}
	return mapValue, nil
}
