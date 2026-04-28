package runtime

import (
	"context"
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
	actorBackend ActorBackend
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
		actorBackend: CodexCLIBackend{},
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

	workflow, err := readWorkflow(filepath.Join(artifactDirectory, "workflow.json"))
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
		if r.needsPersistedOutputRefresh(artifactDirectory, currentState) {
			return r.router.RouteEvaluation(artifactDirectory)
		}
		if err := writeRunStatus(artifactDirectory, runStatusAfterEvaluation(artifactDirectory, currentState)); err != nil {
			return "", err
		}
		return fmt.Sprintf("Harness execution already completed for %s", artifactDirectory), nil
	}

	requestValue, err := readArtifactRequest(filepath.Join(artifactDirectory, "request.yaml"))
	if err != nil {
		return "", err
	}
	effectiveProjectRoot := workflow.ProjectRoot
	if strings.TrimSpace(effectiveProjectRoot) == "" {
		effectiveProjectRoot = r.projectRoot
	}
	workingDirectory, err := filepath.Abs(effectiveProjectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve workflow project root: %w", err)
	}
	var backend ActorBackendConfig
	var actorProfiles ActorProfiles
	standardActorRuntimeLoaded := false
	ensureStandardActorRuntime := func() error {
		if standardActorRuntimeLoaded {
			return nil
		}

		backendPolicy, err := loadActorBackendPolicy(workingDirectory)
		if err != nil {
			return fmt.Errorf("load actor backend policy: %w", err)
		}
		backend, err = backendPolicy.DefaultBackend()
		if err != nil {
			return err
		}
		if len(currentState.ActorProfilesUsed) == 0 {
			actorProfiles, err = loadActorProfiles(workingDirectory, profiledActorsForWorkflow(workflow))
			if err != nil {
				return fmt.Errorf("load actor profiles: %w", err)
			}
			currentState.ActorProfilesUsed = snapshotActorProfilesUsed(workflow, actorProfiles)
			if err := writeJSON(statePath, currentState); err != nil {
				return err
			}
		} else {
			if err := validateActorProfilesSnapshot(workflow, currentState.ActorProfilesUsed); err != nil {
				return err
			}
			actorProfiles = actorProfilesFromSnapshot(currentState.ActorProfilesUsed)
		}
		standardActorRuntimeLoaded = true
		return nil
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
			return r.recordInterruption(
				artifactDirectory,
				currentState,
				"actor_resolution",
				actorName,
				fmt.Errorf("unknown actor in state: %s", actorName),
			)
		}
		if err := r.validateActorExecutionPrerequisites(workflow, currentState, artifactDirectory, actorName); err != nil {
			return r.recordInterruption(artifactDirectory, currentState, "prerequisite_validation", actorName, err)
		}

		outputName := canonicalOutputForActor(actorName)
		outputPath := filepath.Join(artifactDirectory, artifactFileName(outputName))
		logPath := filepath.Join(runsDirectory, actorLogFileName(actorIndex, actorName, currentState.CompletedActors))
		eventsPath := filepath.Join(runsDirectory, actorEventFileName(actorIndex, actorName, currentState.CompletedActors))
		briefPath := filepath.Join(artifactDirectory, "actor_briefs", fmt.Sprintf("%02d_%s.md", actorIndex+1, actorName))

		if actorRequiresStandardRuntime(requestValue, actorName) {
			if err := ensureStandardActorRuntime(); err != nil {
				return r.recordInterruption(artifactDirectory, currentState, "runtime_setup", actorName, err)
			}
		}
		response, err := r.runActor(actorName, actorIndex, artifactDirectory, briefPath, logPath, eventsPath, requestValue, executionPlan, actorProfiles, backend, workingDirectory)
		if err != nil {
			if isBackendPolicyViolation(err) {
				return r.blockForBackendPolicyViolation(artifactDirectory, statePath, currentState, actorName, err)
			}
			return r.recordInterruption(artifactDirectory, currentState, "actor_execution", actorName, err)
		}
		if err := writeActorLog(logPath, response); err != nil {
			return "", err
		}
		if response != nil {
			response = normalizeActorResponse(outputName, response)
			if err := writeYAML(outputPath, response); err != nil {
				return r.recordInterruption(artifactDirectory, currentState, "artifact_write", actorName, err)
			}
			if _, err := r.bootstrapper.validator.ValidateArtifactFile(outputPath, outputName); err != nil {
				return r.recordInterruption(
					artifactDirectory,
					currentState,
					"artifact_validation",
					actorName,
					fmt.Errorf("validate %s output: %w", actorName, err),
				)
			}
		}

		if actorName == "evaluator" {
			lastSummary, err = r.router.RouteEvaluation(artifactDirectory)
			if err != nil {
				return r.recordInterruption(artifactDirectory, currentState, "evaluation_routing", actorName, err)
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
		if err := updateContinuityAfterActor(artifactDirectory, actorName, currentState); err != nil {
			return "", err
		}
		if err := writeJSON(statePath, currentState); err != nil {
			return "", err
		}
	}

	if currentState.CurrentActor != nil {
		return r.recordInterruption(
			artifactDirectory,
			currentState,
			"execution_loop",
			actorLabel(currentState.CurrentActor),
			fmt.Errorf("execution loop exceeded step budget for %s", artifactDirectory),
		)
	}

	if lastSummary != "" {
		return lastSummary, nil
	}
	return formatExecutionSummary(artifactDirectory, currentState, "Harness execution completed"), nil
}

func isBackendPolicyViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "backend_policy_violation")
}

func actorRequiresStandardRuntime(requestValue request.CanonicalRequest, actorName string) bool {
	if requestValue.ValidationProfile == "smoke" {
		return false
	}
	switch actorName {
	case "planner", "context_builder", "critic", "generator", "evaluator":
		return true
	default:
		return false
	}
}

func (r *Runner) recordInterruption(
	artifactDirectory string,
	state State,
	phase string,
	actorName string,
	executionErr error,
) (string, error) {
	status := interruptedRunStatus(artifactDirectory, phase, actorName, state, executionErr)
	if err := writeRunStatus(artifactDirectory, status); err != nil {
		return "", fmt.Errorf("%w; additionally failed to write %s: %v", executionErr, runStatusFileName, err)
	}
	if err := appendWorkLedgerEntry(
		filepath.Join(artifactDirectory, workLedgerFileName),
		"Execution interrupted",
		[]string{
			"phase: " + phase,
			"actor: " + fallbackString(actorName, "unknown"),
			"interruption: " + status.InterruptionKind,
			"message: " + executionErr.Error(),
		},
	); err != nil {
		return "", fmt.Errorf("%w; additionally failed to append %s: %v", executionErr, workLedgerFileName, err)
	}
	return "", executionErr
}

func (r *Runner) blockForBackendPolicyViolation(
	artifactDirectory string,
	statePath string,
	state State,
	actorName string,
	violation error,
) (string, error) {
	nextState := state
	nextState.Status = "blocked_environment"
	nextState.CurrentActor = nil
	nextState.LastReasonCodes = []string{"backend_policy_violation"}
	nextState.ActionHistory = append(nextState.ActionHistory, "block_environment")
	nextState.LastDecision = stringPtr("reject")

	if err := writePolicyViolationExecutionReport(artifactDirectory, violation); err != nil {
		return "", err
	}
	if err := writePolicyViolationCriticReportIfMissing(artifactDirectory); err != nil {
		return "", err
	}
	if err := updateContinuityAfterEvaluation(artifactDirectory, nextState); err != nil {
		return "", err
	}
	if err := writeJSON(statePath, nextState); err != nil {
		return "", err
	}
	if err := appendSupervisorDecisionTrace(artifactDirectory, nextState); err != nil {
		return "", err
	}
	if err := r.router.writeTerminalSummary(artifactDirectory, nextState); err != nil {
		return "", err
	}
	if err := writeRunStatus(artifactDirectory, RunStatus{
		Status:              "blocked_environment",
		Phase:               "backend_policy",
		CurrentActor:        actorName,
		LastSuccessfulActor: lastSuccessfulActor(state),
		InterruptionKind:    "backend_policy_violation",
		Message:             violation.Error(),
		Evidence: []string{
			"execution_report.yaml",
			"critic_report.yaml",
			"terminal_summary.md",
			"state.json",
			workLedgerFileName,
			nextActionFileName,
		},
		NextStep: "Read terminal_summary.md and fix backend policy isolation before continuing.",
	}); err != nil {
		return "", err
	}
	return formatExecutionSummary(artifactDirectory, nextState, "Harness execution blocked by backend policy"), nil
}

func writePolicyViolationExecutionReport(artifactDirectory string, violation error) error {
	executionPath := filepath.Join(artifactDirectory, "execution_report.yaml")
	executionMap, err := readYAMLMap(executionPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		executionMap = recoveredExecutionReportBase(State{Status: "blocked_environment"})
	}
	executionMap["format"] = "fail"
	executionMap["analyze"] = "fail"
	executionMap["tests"] = map[string]any{
		"total":  0,
		"passed": 0,
		"failed": 0,
	}
	failureDetails, _ := readOptionalStringList(executionMap, "failure_details")
	failureDetails = append(failureDetails, violation.Error())
	executionMap["failure_details"] = failureDetails
	logs, _ := readOptionalStringList(executionMap, "logs")
	logs = append(logs, violation.Error())
	executionMap["logs"] = logs
	return writeYAML(executionPath, executionMap)
}

func writePolicyViolationCriticReportIfMissing(artifactDirectory string) error {
	criticPath := filepath.Join(artifactDirectory, "critic_report.yaml")
	if _, err := os.Stat(criticPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	return writeYAML(criticPath, map[string]any{
		"priority_focus":          []string{"Backend policy violation blocked actor execution before normal evaluation could complete."},
		"missing_requirements":    []string{},
		"risk_hypotheses":         []string{"The actor runtime may have inherited a forbidden user surface."},
		"validation_expectations": []string{"Do not claim implementation success while backend policy violations are present."},
		"generator_guardrails":    []string{"Stop execution and surface the policy violation to the operator."},
		"blocked_assumptions":     []string{"Normal critic evidence may be unavailable because execution was blocked by policy."},
	})
}

func (r *Runner) runActor(
	actorName string,
	actorIndex int,
	artifactDirectory string,
	briefPath string,
	logPath string,
	eventsPath string,
	requestValue request.CanonicalRequest,
	executionPlan ExecutionPlan,
	actorProfiles ActorProfiles,
	backend ActorBackendConfig,
	workingDirectory string,
) (map[string]any, error) {
	if requestValue.ValidationProfile == "smoke" {
		switch actorName {
		case "planner":
			return buildSmokePlan(artifactDirectory, workingDirectory, requestValue), nil
		case "context_builder":
			return buildSmokeContextPack(artifactDirectory)
		case "critic":
			return buildSmokeCriticReport(), nil
		case "generator":
			return buildSmokeImplementationResult(artifactDirectory)
		case "executor":
			return r.buildExecutionReport(executionPlan, workingDirectory)
		case "evaluator":
			return buildSmokeEvaluationResult(artifactDirectory)
		default:
			return nil, fmt.Errorf("actor execution is not implemented for %s", actorName)
		}
	}

	switch actorName {
	case "executor":
		return r.buildExecutionReport(executionPlan, workingDirectory)
	case "planner", "context_builder", "critic", "generator", "evaluator":
		profile, err := actorProfiles.ProfileFor(actorName)
		if err != nil {
			return nil, err
		}
		schemaPath, err := materializeActorOutputSchema(filepath.Join(artifactDirectory, "runs"), actorIndex, actorName, canonicalOutputForActor(actorName))
		if err != nil {
			return nil, err
		}
		prompt := strings.Join([]string{
			"Read the Rail actor brief and produce only the schema-valid actor output.",
			"Actor name: " + actorName,
			"Actor brief: " + briefPath,
			"Artifact directory: " + artifactDirectory,
			"Project root: " + workingDirectory,
			"Follow the actor brief exactly. You may inspect or edit files under the project root only when the brief requires it. Do not write artifact files yourself; return only the schema-valid actor response.",
		}, "\n")
		actorBackend := r.actorBackend
		if actorBackend == nil {
			actorBackend = CodexCLIBackend{}
		}
		result, err := actorBackend.RunActor(context.Background(), ActorInvocation{
			ActorName:         actorName,
			ActorRunID:        actorRunIDFromLogPath(logPath),
			WorkingDirectory:  workingDirectory,
			Prompt:            prompt,
			ArtifactDirectory: artifactDirectory,
			OutputSchemaPath:  schemaPath,
			LastMessagePath:   logPath,
			EventsPath:        eventsPath,
			Profile:           profile,
			Policy:            backend,
		})
		if err != nil {
			return nil, err
		}
		return result.StructuredOutput, nil
	default:
		return nil, fmt.Errorf("actor execution is not implemented for %s", actorName)
	}
}

func (r *Runner) validateActorExecutionPrerequisites(workflow Workflow, state State, artifactDirectory string, actorName string) error {
	actorIndex := workflowActorIndex(workflow, actorName)
	if actorIndex <= 0 {
		return nil
	}

	completedActors := make(map[string]struct{}, len(state.CompletedActors))
	for _, completedActor := range state.CompletedActors {
		completedActors[completedActor] = struct{}{}
	}

	for _, prerequisiteActor := range workflow.Actors[:actorIndex] {
		if _, ok := completedActors[prerequisiteActor]; !ok {
			return fmt.Errorf("cannot run %s: prerequisite actor %s has not completed", actorName, prerequisiteActor)
		}

		outputName := canonicalOutputForActor(prerequisiteActor)
		outputPath := filepath.Join(artifactDirectory, artifactFileName(outputName))
		if _, err := r.bootstrapper.validator.ValidateArtifactFile(outputPath, outputName); err != nil {
			return fmt.Errorf("cannot run %s: prerequisite %s output invalid: %w", actorName, outputName, err)
		}
	}

	return nil
}

func buildSmokeCriticReport() map[string]any {
	return map[string]any{
		"priority_focus":          []string{"Keep the smoke-path execution bounded, deterministic, and reviewable."},
		"missing_requirements":    []string{},
		"risk_hypotheses":         []string{},
		"validation_expectations": []string{"Preserve passing format, analyze, and tests evidence for the smoke execution plan."},
		"generator_guardrails":    []string{"Do not edit files outside the scoped target."},
		"blocked_assumptions":     []string{},
	}
}

func snapshotActorProfilesUsed(workflow Workflow, actorProfiles ActorProfiles) []ActorProfileUsed {
	snapshot := make([]ActorProfileUsed, 0, len(workflow.Actors))
	for _, actorName := range workflow.Actors {
		if actorName == "executor" {
			continue
		}
		profile, err := actorProfiles.ProfileFor(actorName)
		if err != nil {
			continue
		}
		snapshot = append(snapshot, ActorProfileUsed{
			Actor:     actorName,
			Model:     profile.Model,
			Reasoning: profile.Reasoning,
		})
	}
	return snapshot
}

func actorProfilesFromSnapshot(snapshot []ActorProfileUsed) ActorProfiles {
	actors := make(map[string]ActorProfile, len(snapshot))
	for _, profile := range snapshot {
		actors[profile.Actor] = ActorProfile{
			Model:     profile.Model,
			Reasoning: profile.Reasoning,
		}
	}
	return ActorProfiles{
		Version: 1,
		Actors:  actors,
	}
}

func profiledActorsForWorkflow(workflow Workflow) []string {
	profiledActors := make([]string, 0, len(workflow.Actors))
	for _, actorName := range workflow.Actors {
		if actorName == "executor" {
			continue
		}
		profiledActors = append(profiledActors, actorName)
	}
	return profiledActors
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

func actorLogFileName(actorIndex int, actorName string, completedActors []string) string {
	visit := countActor(completedActors, actorName) + 1
	if visit == 1 {
		return fmt.Sprintf("%02d_%s-last-message.txt", actorIndex+1, actorName)
	}
	return fmt.Sprintf("%02d_%s-visit-%02d-last-message.txt", actorIndex+1, actorName, visit)
}

func actorEventFileName(actorIndex int, actorName string, completedActors []string) string {
	visit := countActor(completedActors, actorName) + 1
	if visit == 1 {
		return fmt.Sprintf("%02d_%s-events.jsonl", actorIndex+1, actorName)
	}
	return fmt.Sprintf("%02d_%s-visit-%02d-events.jsonl", actorIndex+1, actorName, visit)
}

func (r *Runner) needsPersistedOutputRefresh(artifactDirectory string, state State) bool {
	tracePath := filepath.Join(artifactDirectory, "supervisor_trace.md")
	if len(state.ActionHistory) > 0 && persistedOutputMissing(tracePath) {
		return true
	}
	if len(state.ActionHistory) > 0 {
		executionPath := filepath.Join(artifactDirectory, "execution_report.yaml")
		if _, err := r.bootstrapper.validator.ValidateArtifactFile(executionPath, "execution_report"); err != nil {
			return true
		}
	}

	if !shouldTerminate(state) {
		return false
	}

	summaryPath := filepath.Join(artifactDirectory, "terminal_summary.md")
	return persistedOutputMissing(summaryPath)
}

func persistedOutputMissing(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func advanceAfterActor(state State, workflow Workflow, actorName string) State {
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
	case state.Status == "revising":
		nextState.Status = "revising"
	case state.Status == "rebuilding_context":
		nextState.Status = "rebuilding_context"
	case state.Status == "tightening_validation":
		nextState.Status = "tightening_validation"
	default:
		nextState.Status = "in_progress"
	}

	return nextState
}

func workflowActorIndex(workflow Workflow, actorName string) int {
	for index, candidate := range workflow.Actors {
		if candidate == actorName {
			return index
		}
	}
	return -1
}

func maxExecutionSteps(workflow Workflow) int {
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
			filepath.Join(projectRoot, "cmd", "rail", "main.go"),
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

func (r *Runner) buildExecutionReport(executionPlan ExecutionPlan, workingDirectory string) (map[string]any, error) {
	logs := []string{}
	failureDetails := []string{}
	formatPass := executionPlan.FormatCommand == nil
	analyzePass := true
	passedTests := 0
	failedTests := 0

	if executionPlan.FormatCommand != nil {
		result, err := r.commands.RunShell(*executionPlan.FormatCommand, workingDirectory, time.Minute)
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
		result, err := r.commands.RunShell(command, workingDirectory, time.Minute)
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
		result, err := r.commands.RunShell(command, workingDirectory, time.Minute)
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
	formatPass := readOptionalStringValue(executionReport, "format", "fail") == "pass"
	analyzePass := readOptionalStringValue(executionReport, "analyze", "fail") == "pass"
	tests, err := readMap(executionReport, "tests")
	if err != nil {
		return nil, err
	}
	testFailures, err := readInt(tests, "failed")
	if err != nil {
		return nil, err
	}
	pass := formatPass && analyzePass && testFailures == 0

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

func normalizeActorResponse(outputName string, response map[string]any) map[string]any {
	if outputName != "evaluation_result" {
		return response
	}

	decision, _ := response["decision"].(string)
	nextAction, exists := response["next_action"]
	if !exists {
		return response
	}
	if decision != "revise" && nextAction == nil {
		delete(response, "next_action")
	}
	return response
}
