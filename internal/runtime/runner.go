package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rail/internal/request"

	"gopkg.in/yaml.v3"
)

type Runner struct {
	projectRoot  string
	bootstrapper *Bootstrapper
	router       *Router
	commands     CommandRunner
}

func NewRunner(projectRoot string) (*Runner, error) {
	bootstrapper, err := NewBootstrapper(projectRoot)
	if err != nil {
		return nil, err
	}
	router, err := NewRouter(projectRoot)
	if err != nil {
		return nil, err
	}
	return &Runner{
		projectRoot:  bootstrapper.projectRoot,
		bootstrapper: bootstrapper,
		router:       router,
		commands:     subprocessRunner{},
	}, nil
}

func (r *Runner) Run(requestPath, taskID string) (string, error) {
	effectiveTaskID := strings.TrimSpace(taskID)
	if effectiveTaskID == "" {
		effectiveTaskID = defaultTaskID(requestPath)
	}

	artifactDirectory, err := r.resolveArtifactDirectory(effectiveTaskID)
	if err != nil {
		return "", err
	}
	if err := ensureArtifactDirectoryAvailable(artifactDirectory); err != nil {
		return "", err
	}

	return r.bootstrapper.Bootstrap(requestPath, effectiveTaskID)
}

func (r *Runner) Execute(artifactPath string) (string, error) {
	artifactDirectory, err := r.router.resolveArtifactDirectory(artifactPath)
	if err != nil {
		return "", err
	}

	workflow, err := readResolvedWorkflow(filepath.Join(artifactDirectory, "resolved_workflow.json"))
	if err != nil {
		return "", err
	}
	executionPlan, err := readExecutionPlanFile(filepath.Join(artifactDirectory, "execution_plan.json"))
	if err != nil {
		return "", err
	}
	statePath := filepath.Join(artifactDirectory, "state.json")
	currentState, err := readState(statePath)
	if err != nil {
		return "", err
	}
	if currentState.CurrentActor == nil {
		return fmt.Sprintf("Harness execution already completed for %s", artifactDirectory), nil
	}

	requestValue, err := readArtifactRequest(filepath.Join(artifactDirectory, "request.yaml"))
	if err != nil {
		return "", err
	}

	runsDirectory := filepath.Join(artifactDirectory, "runs")
	if err := os.MkdirAll(runsDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create runs directory: %w", err)
	}

	lastSummary := ""
	for step := 0; step < maxExecutionSteps(workflow); step++ {
		if currentState.CurrentActor == nil {
			break
		}

		actorName := *currentState.CurrentActor
		actorIndex := workflowActorIndex(workflow, actorName)
		if actorIndex == -1 {
			return "", fmt.Errorf("unknown actor in state: %s", actorName)
		}

		outputName := canonicalOutputForActor(actorName)
		outputPath := filepath.Join(artifactDirectory, artifactFileName(outputName))
		logPath := filepath.Join(
			runsDirectory,
			fmt.Sprintf("%02d_%s-last-message.txt", actorIndex+1, actorName),
		)

		response, err := r.runActor(actorName, artifactDirectory, requestValue, executionPlan)
		if err != nil {
			return "", err
		}
		if err := writeActorLog(logPath, response); err != nil {
			return "", err
		}
		if response != nil {
			if err := writeYAML(outputPath, response); err != nil {
				return "", err
			}
		}

		if actorName == "evaluator" {
			lastSummary, err = r.router.RouteEvaluation(artifactDirectory)
			if err != nil {
				return "", err
			}
			currentState, err = readState(statePath)
			if err != nil {
				return "", err
			}
			if shouldTerminate(currentState) {
				break
			}
			continue
		}

		currentState = advanceAfterActor(currentState, workflow, actorName)
		if err := writeJSON(statePath, currentState); err != nil {
			return "", err
		}
	}

	if currentState.CurrentActor != nil {
		return "", fmt.Errorf("execution loop exceeded step budget for %s", artifactDirectory)
	}

	if lastSummary != "" {
		return lastSummary, nil
	}
	return formatExecutionSummary(artifactDirectory, currentState, "Harness execution completed"), nil
}

func (r *Runner) runActor(
	actorName string,
	artifactDirectory string,
	requestValue request.CanonicalRequest,
	executionPlan ExecutionPlan,
) (map[string]any, error) {
	if requestValue.ValidationProfile != "smoke" {
		return nil, fmt.Errorf("actor execution is only implemented for smoke validation profiles")
	}

	switch actorName {
	case "planner":
		return buildSmokePlan(artifactDirectory, r.projectRoot, requestValue), nil
	case "context_builder":
		return buildSmokeContextPack(artifactDirectory)
	case "generator":
		return buildSmokeImplementationResult(artifactDirectory)
	case "executor":
		return r.buildSmokeExecutionReport(executionPlan)
	case "evaluator":
		return buildSmokeEvaluationResult(artifactDirectory)
	default:
		return nil, fmt.Errorf("actor execution is not implemented for %s", actorName)
	}
}

func defaultTaskID(requestPath string) string {
	base := strings.TrimSuffix(filepath.Base(requestPath), filepath.Ext(requestPath))
	base = strings.TrimSpace(base)
	if base == "" {
		return "rail-task"
	}
	base = strings.ReplaceAll(base, " ", "-")
	return base
}

func (r *Runner) resolveArtifactDirectory(taskID string) (string, error) {
	executionPolicyMap, err := r.bootstrapper.loadMap(".harness/supervisor/execution_policy.yaml")
	if err != nil {
		return "", err
	}
	executionPolicy, err := executionPolicyFromMap(executionPolicyMap)
	if err != nil {
		return "", err
	}
	return filepath.Join(r.projectRoot, filepath.FromSlash(executionPolicy.ArtifactRoot), taskID), nil
}

func ensureArtifactDirectoryAvailable(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat artifact directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("artifact path already exists and is not a directory: %s", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return fmt.Errorf("read artifact directory: %w", err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("artifact directory already exists and is not empty: %s", path)
	}
	return nil
}

func readExecutionPlanFile(path string) (ExecutionPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ExecutionPlan{}, fmt.Errorf("read execution plan: %w", err)
	}
	var executionPlan ExecutionPlan
	if err := json.Unmarshal(data, &executionPlan); err != nil {
		return ExecutionPlan{}, fmt.Errorf("decode execution plan: %w", err)
	}
	return executionPlan, nil
}

func readArtifactRequest(path string) (request.CanonicalRequest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return request.CanonicalRequest{}, fmt.Errorf("read request snapshot: %w", err)
	}
	var requestValue request.CanonicalRequest
	if err := yaml.Unmarshal(data, &requestValue); err != nil {
		return request.CanonicalRequest{}, fmt.Errorf("decode request snapshot: %w", err)
	}
	return requestValue, nil
}

func writeActorLog(path string, value map[string]any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal actor log %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write actor log %s: %w", path, err)
	}
	return nil
}

func advanceAfterActor(state State, workflow ResolvedWorkflow, actorName string) State {
	nextState := state
	nextState.CompletedActors = append(append([]string{}, state.CompletedActors...), actorName)

	actorIndex := workflowActorIndex(workflow, actorName)
	var nextActor *string
	if actorIndex != -1 && actorIndex+1 < len(workflow.Actors) {
		nextActor = stringPtr(workflow.Actors[actorIndex+1])
	}
	nextState.CurrentActor = nextActor

	switch {
	case nextActor == nil:
		nextState.Status = "completed"
	case state.Status == "revising" && actorName == "generator":
		nextState.Status = "revising"
	case state.Status == "rebuilding_context" && actorName == "context_builder":
		nextState.Status = "rebuilding_context"
	case state.Status == "tightening_validation" && actorName == "executor":
		nextState.Status = "tightening_validation"
	default:
		nextState.Status = "in_progress"
	}

	return nextState
}

func workflowActorIndex(workflow ResolvedWorkflow, actorName string) int {
	for index, candidate := range workflow.Actors {
		if candidate == actorName {
			return index
		}
	}
	return -1
}

func maxExecutionSteps(workflow ResolvedWorkflow) int {
	return len(workflow.Actors)*4 + workflow.GeneratorRetryBudget + workflow.ContextRebuildBudget + workflow.ValidationTightenBudget + 8
}

func buildSmokePlan(
	artifactDirectory string,
	projectRoot string,
	requestValue request.CanonicalRequest,
) map[string]any {
	requestFile := filepath.Join(artifactDirectory, "request.yaml")
	return map[string]any{
		"summary": "Smoke-profile plan for `" + requestValue.Goal + "` focused on validating the separated rail control-plane actor chain without broad repository changes.",
		"likely_files": []string{
			filepath.Join(projectRoot, "bin", "rail.dart"),
			filepath.Join(projectRoot, ".harness", "supervisor", "execution_policy.yaml"),
			requestFile,
		},
		"assumptions": []string{
			"Smoke validation should stay inside the rail control-plane repo unless the execution plan explicitly calls the external target repo.",
			"Schema-valid actor outputs are sufficient for this smoke profile.",
		},
		"substeps": []string{
			"Produce minimal schema-valid plan/context/implementation artifacts.",
			"Run smoke validation commands from the execution plan.",
			"Decide pass or revise from the smoke execution report.",
		},
		"risks": []string{
			"Smoke profile verifies control-plane flow, not full target-repo correctness.",
		},
		"acceptance_criteria_refined": append([]string{}, requestValue.DefinitionOfDone...),
	}
}

func buildSmokeContextPack(artifactDirectory string) (map[string]any, error) {
	planMap, err := readYAMLMap(filepath.Join(artifactDirectory, "plan.yaml"))
	if err != nil {
		return nil, err
	}
	likelyFiles, err := readOptionalStringList(planMap, "likely_files")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"relevant_files": mapRelevantFiles(likelyFiles),
		"repo_patterns": []string{
			"Smoke validation uses deterministic actor outputs to verify control-plane orchestration quickly.",
			"Executor commands still come from the generated execution plan, even when actor outputs are synthesized.",
		},
		"test_patterns": []string{
			"Smoke requests favor reachability and schema validation over full lint/test coverage.",
		},
		"forbidden_changes": readBulletList(filepath.Join(artifactDirectory, "inputs", "forbidden_changes.md")),
		"implementation_hints": []string{
			"Keep smoke artifacts deterministic and scoped to the rail repo.",
		},
	}, nil
}

func mapRelevantFiles(paths []string) []map[string]any {
	relevant := make([]map[string]any, 0, len(paths))
	for _, path := range paths {
		relevant = append(relevant, map[string]any{
			"path": path,
			"why":  "Smoke-profile actor chain depends on this control-plane file.",
		})
	}
	return relevant
}

func buildSmokeImplementationResult(artifactDirectory string) (map[string]any, error) {
	planMap, err := readYAMLMap(filepath.Join(artifactDirectory, "plan.yaml"))
	if err != nil {
		return nil, err
	}
	likelyFiles, err := readOptionalStringList(planMap, "likely_files")
	if err != nil {
		return nil, err
	}

	result := map[string]any{
		"changed_files":          []string{},
		"patch_summary":          []string{"Smoke profile skips repository edits and validates orchestration using synthesized actor outputs."},
		"tests_added_or_updated": []string{},
		"known_limits":           []string{},
	}
	if len(likelyFiles) > 0 {
		result["known_limits"] = []string{
			"Likely implementation scope for non-smoke execution: " + strings.Join(likelyFiles, ", "),
		}
	}
	return result, nil
}

func (r *Runner) buildSmokeExecutionReport(executionPlan ExecutionPlan) (map[string]any, error) {
	logs := []string{}
	failureDetails := []string{}
	formatPass := executionPlan.FormatCommand == nil
	analyzePass := true
	passedTests := 0
	failedTests := 0

	if executionPlan.FormatCommand != nil {
		result, err := r.commands.RunShell(*executionPlan.FormatCommand, r.projectRoot, time.Minute)
		if err != nil {
			return nil, err
		}
		formatPass = result.ExitCode == 0
		logs = append(logs, commandSummary(*executionPlan.FormatCommand, result.ExitCode))
		if !formatPass {
			failureDetails = append(failureDetails, "Format command failed: "+*executionPlan.FormatCommand)
		}
	}

	for _, command := range executionPlan.AnalyzeCommands {
		result, err := r.commands.RunShell(command, r.projectRoot, time.Minute)
		if err != nil {
			return nil, err
		}
		logs = append(logs, commandSummary(command, result.ExitCode))
		if result.ExitCode != 0 {
			analyzePass = false
			failureDetails = append(failureDetails, "Analyze command failed: "+command)
		}
	}

	for _, command := range executionPlan.TestCommands {
		result, err := r.commands.RunShell(command, r.projectRoot, time.Minute)
		if err != nil {
			return nil, err
		}
		logs = append(logs, commandSummary(command, result.ExitCode))
		if result.ExitCode == 0 {
			passedTests++
		} else {
			failedTests++
			failureDetails = append(failureDetails, "Test command failed: "+command)
		}
	}

	return map[string]any{
		"format":  ternaryStatus(formatPass),
		"analyze": ternaryStatus(analyzePass),
		"tests": map[string]any{
			"total":  len(executionPlan.TestCommands),
			"passed": passedTests,
			"failed": failedTests,
		},
		"failure_details": failureDetails,
		"logs":            logs,
	}, nil
}

func buildSmokeEvaluationResult(artifactDirectory string) (map[string]any, error) {
	executionReport, err := readYAMLMap(filepath.Join(artifactDirectory, "execution_report.yaml"))
	if err != nil {
		return nil, err
	}
	analyzePass := readOptionalStringValue(executionReport, "analyze", "fail") == "pass"
	tests, err := readMap(executionReport, "tests")
	if err != nil {
		return nil, err
	}
	testFailures, err := readInt(tests, "failed")
	if err != nil {
		return nil, err
	}
	pass := analyzePass && testFailures == 0

	response := map[string]any{
		"decision": passFailDecision(pass),
		"scores": map[string]any{
			"requirements":    conditionalScore(pass, 1.0, 0.5),
			"architecture":    1.0,
			"regression_risk": conditionalScore(pass, 1.0, 0.5),
		},
		"quality_confidence": conditionalText(pass, "high", "low"),
		"findings": []string{
			conditionalText(
				pass,
				"Smoke-profile actor chain completed with schema-valid artifacts and passing smoke validation commands.",
				"Smoke-profile validation reported at least one failed command.",
			),
		},
		"reason_codes": []string{},
	}
	if !pass {
		response["reason_codes"] = []string{"smoke_validation_failed"}
		response["next_action"] = "tighten_validation"
	}
	return response, nil
}

func readBulletList(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return []string{}
	}
	lines := strings.Split(string(data), "\n")
	values := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "- ") {
			continue
		}
		value := strings.TrimSpace(strings.TrimPrefix(trimmed, "- "))
		if value != "" {
			values = append(values, value)
		}
	}
	return values
}

func commandSummary(command string, exitCode int) string {
	return fmt.Sprintf("%s (exit=%d)", command, exitCode)
}

func ternaryStatus(pass bool) string {
	if pass {
		return "pass"
	}
	return "fail"
}

func passFailDecision(pass bool) string {
	if pass {
		return "pass"
	}
	return "revise"
}

func conditionalScore(pass bool, yes, no float64) float64 {
	if pass {
		return yes
	}
	return no
}

func conditionalText(pass bool, yes, no string) string {
	if pass {
		return yes
	}
	return no
}
