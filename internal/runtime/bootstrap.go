package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
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

	requestFile, err := contracts.ResolvePathWithinRoot(b.projectRoot, requestPath)
	if err != nil {
		return "", err
	}
	requestValue, err := b.validator.ValidateRequestFile(requestPath)
	if err != nil {
		return "", err
	}

	registry, err := b.loadMap(".harness/supervisor/registry.yaml")
	if err != nil {
		return "", err
	}
	taskRouter, err := b.loadMap(".harness/supervisor/task_router.yaml")
	if err != nil {
		return "", err
	}
	policy, err := b.loadMap(".harness/supervisor/policy.yaml")
	if err != nil {
		return "", err
	}
	executionPolicyMap, err := b.loadMap(".harness/supervisor/execution_policy.yaml")
	if err != nil {
		return "", err
	}
	testRulesMap, err := b.loadMap(".harness/supervisor/test_target_rules.yaml")
	if err != nil {
		return "", err
	}
	contextContractMap, err := b.loadMap(".harness/supervisor/context_contract.yaml")
	if err != nil {
		return "", err
	}

	executionPolicy, err := executionPolicyFromMap(executionPolicyMap)
	if err != nil {
		return "", err
	}
	testRules, err := testTargetRulesFromMap(testRulesMap)
	if err != nil {
		return "", err
	}
	contextContract, err := contextContractFromMap(contextContractMap)
	if err != nil {
		return "", err
	}

	fileHints, err := normalizeFileHints(b.projectRoot, requestValue)
	if err != nil {
		return "", err
	}
	executionPlan, err := buildExecutionPlan(
		requestValue,
		b.projectRoot,
		executionPolicy,
		testRules,
		fileHints,
	)
	if err != nil {
		return "", err
	}

	workflow, err := buildWorkflow(
		b.projectRoot,
		requestFile,
		taskID,
		requestValue,
		registry,
		taskRouter,
		policy,
		contextContract,
		fileHints,
		executionPlan.TestTargets(),
	)
	if err != nil {
		return "", err
	}

	artifactDirectory := filepath.Join(b.projectRoot, filepath.FromSlash(executionPolicy.ArtifactRoot), taskID)
	if err := os.MkdirAll(artifactDirectory, 0o755); err != nil {
		return "", fmt.Errorf("create artifact directory: %w", err)
	}

	if err := os.MkdirAll(filepath.Join(artifactDirectory, "inputs"), 0o755); err != nil {
		return "", fmt.Errorf("create inputs directory: %w", err)
	}
	if executionPolicy.CreateActorBriefs {
		if err := os.MkdirAll(filepath.Join(artifactDirectory, "actor_briefs"), 0o755); err != nil {
			return "", fmt.Errorf("create actor_briefs directory: %w", err)
		}
	}

	requestBytes, err := os.ReadFile(requestFile)
	if err != nil {
		return "", fmt.Errorf("read request file: %w", err)
	}
	if err := os.WriteFile(filepath.Join(artifactDirectory, "request.yaml"), requestBytes, 0o644); err != nil {
		return "", fmt.Errorf("write request snapshot: %w", err)
	}

	materializedInputs, err := b.materializeStaticInputs(artifactDirectory, workflow.RubricPath)
	if err != nil {
		return "", err
	}

	if executionPolicy.PersistJSONSnapshots {
		if err := writeJSON(filepath.Join(artifactDirectory, "workflow.json"), workflow); err != nil {
			return "", err
		}
		if err := writeJSON(filepath.Join(artifactDirectory, "execution_plan.json"), executionPlan); err != nil {
			return "", err
		}
		if err := writeJSON(filepath.Join(artifactDirectory, "state.json"), initialState(workflow)); err != nil {
			return "", err
		}
	}

	if executionPolicy.CreatePlaceholders {
		for _, output := range workflow.RequiredOutputs {
			if err := writeYAML(filepath.Join(artifactDirectory, artifactFileName(output)), placeholderContent(output)); err != nil {
				return "", err
			}
		}
	}

	if err := os.WriteFile(filepath.Join(artifactDirectory, workLedgerFileName), []byte(initialWorkLedger(workflow)), 0o644); err != nil {
		return "", fmt.Errorf("write %s: %w", workLedgerFileName, err)
	}
	if err := writeYAML(filepath.Join(artifactDirectory, nextActionFileName), initialNextAction(workflow)); err != nil {
		return "", err
	}
	if err := writeYAML(filepath.Join(artifactDirectory, evidenceFileName), initialEvidence()); err != nil {
		return "", err
	}
	if err := writeYAML(filepath.Join(artifactDirectory, finalAnswerContractFileName), initialFinalAnswerContract()); err != nil {
		return "", err
	}

	if err := os.WriteFile(
		filepath.Join(artifactDirectory, "workflow_steps.md"),
		[]byte(buildWorkflowSteps(workflow, executionPlan)),
		0o644,
	); err != nil {
		return "", fmt.Errorf("write workflow steps: %w", err)
	}

	if executionPolicy.CreateActorBriefs {
		for index, actorName := range workflow.Actors {
			actorInstructions, err := b.loadTextAsset(filepath.ToSlash(filepath.Join(".harness", "actors", actorName+".md")))
			if err != nil {
				return "", err
			}
			actorContract, err := contextContract.contractFor(actorName)
			if err != nil {
				return "", err
			}
			briefPath := filepath.Join(
				artifactDirectory,
				"actor_briefs",
				fmt.Sprintf("%02d_%s.md", index+1, actorName),
			)
			if err := os.WriteFile(
				briefPath,
				[]byte(buildActorBrief(actorName, actorInstructions, workflow, executionPlan, actorContract, artifactDirectory, materializedInputs)),
				0o644,
			); err != nil {
				return "", fmt.Errorf("write actor brief %s: %w", actorName, err)
			}
		}
	}

	return artifactDirectory, nil
}

type Workflow struct {
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
	TaskID                             string             `json:"taskId"`
	TaskFamily                         string             `json:"taskFamily"`
	TaskFamilySource                   string             `json:"taskFamilySource"`
	Status                             string             `json:"status"`
	CurrentActor                       *string            `json:"currentActor"`
	CompletedActors                    []string           `json:"completedActors"`
	GeneratorRetriesRemaining          int                `json:"generatorRetriesRemaining"`
	ContextRebuildsRemaining           int                `json:"contextRebuildsRemaining"`
	ValidationTighteningsRemaining     int                `json:"validationTighteningsRemaining"`
	LastDecision                       *string            `json:"lastDecision"`
	LastReasonCodes                    []string           `json:"lastReasonCodes"`
	ActionHistory                      []string           `json:"actionHistory"`
	GeneratorRevisionsUsed             int                `json:"generatorRevisionsUsed"`
	ContextRefreshCount                int                `json:"contextRefreshCount"`
	LastContextRefreshTrigger          *string            `json:"lastContextRefreshTrigger"`
	LastContextRefreshReasonFamily     *string            `json:"lastContextRefreshReasonFamily"`
	LastInterventionTriggerReasonCodes []string           `json:"lastInterventionTriggerReasonCodes"`
	LastInterventionTriggerCategory    *string            `json:"lastInterventionTriggerCategory"`
	PendingContextRefreshTrigger       *string            `json:"pendingContextRefreshTrigger"`
	PendingContextRefreshReasonFamily  *string            `json:"pendingContextRefreshReasonFamily"`
	ValidationTighteningsUsed          int                `json:"validationTighteningsUsed"`
	QualityTrajectory                  []QualityIteration `json:"qualityTrajectory"`
	ActorProfilesUsed                  []ActorProfileUsed `json:"actorProfilesUsed"`
}

type ActorProfileUsed struct {
	Actor     string `json:"actor"`
	Model     string `json:"model"`
	Reasoning string `json:"reasoning"`
}

type QualityIteration struct {
	Iteration             int      `json:"iteration"`
	Actor                 string   `json:"actor"`
	Decision              string   `json:"decision"`
	Action                string   `json:"action"`
	QualityConfidence     string   `json:"qualityConfidence"`
	ReasonCodes           []string `json:"reasonCodes"`
	TriggerCategory       string   `json:"triggerCategory"`
	Status                string   `json:"status"`
	ExecutorInterventions int      `json:"executorInterventions"`
	ContextRebuilds       int      `json:"contextRebuilds"`
	GeneratorRevisions    int      `json:"generatorRevisions"`
	ValidationTightenings int      `json:"validationTightenings"`
	TotalInterventions    int      `json:"totalInterventions"`
}

type ExecutionPlan struct {
	FormatCommand   *string  `json:"formatCommand"`
	AnalyzeCommands []string `json:"analyzeCommands"`
	TestCommands    []string `json:"testCommands"`
	testTargets     []string
}

func (p ExecutionPlan) TestTargets() []string {
	return append([]string{}, p.testTargets...)
}

type executionPolicy struct {
	ArtifactRoot         string
	FormatCommand        string
	PackageAnalyze       string
	WorkspaceAnalyze     string
	SmokeAnalyze         string
	PackageTest          string
	WorkspaceTest        string
	SmokeTest            string
	CreatePlaceholders   bool
	CreateActorBriefs    bool
	PersistJSONSnapshots bool
}

type testTargetRules struct {
	SourceSuffix    string
	TestSuffix      string
	FeatureTestRoot string
	PackageTestRoot string
	PathRules       []testPathRule
}

type testPathRule struct {
	SourceRoot    string
	SourceSegment string
	TestSegment   string
}

type contextContract struct {
	ActorContracts        map[string]actorContract
	TerminationConditions []string
}

type actorContract struct {
	Inputs  []string
	Outputs []string
}

type materializedInputs struct {
	ArchitectureRulesPath string
	ProjectRulesPath      string
	ForbiddenChangesPath  string
	ExecutionPolicyPath   string
	RubricPath            string
	RequestPath           string
}

func buildWorkflow(
	projectRoot string,
	requestPath string,
	taskID string,
	requestValue request.CanonicalRequest,
	registry map[string]any,
	taskRouter map[string]any,
	policy map[string]any,
	contextContract contextContract,
	fileHints []string,
	testTargets []string,
) (Workflow, error) {
	taskRegistry, err := readMap(registry, "task_registry")
	if err != nil {
		return Workflow{}, err
	}
	taskConfig, err := readMap(taskRegistry, requestValue.TaskType)
	if err != nil {
		return Workflow{}, err
	}
	requiredOutputs, err := readStringList(taskConfig, "required_output")
	if err != nil {
		return Workflow{}, err
	}
	rubricPath, err := readString(taskConfig, "rubric")
	if err != nil {
		return Workflow{}, err
	}
	taskRetryBudget, err := readIntFromMap(taskConfig, "retry", "generator_max_retry")
	if err != nil {
		return Workflow{}, err
	}

	routes, err := readMap(taskRouter, "routes")
	if err != nil {
		return Workflow{}, err
	}
	routeConfig, err := readMap(routes, requestValue.TaskType)
	if err != nil {
		return Workflow{}, err
	}
	actors, err := readStringList(routeConfig, "actors")
	if err != nil {
		return Workflow{}, err
	}
	defaults, err := readMap(taskRouter, "defaults")
	if err != nil {
		return Workflow{}, err
	}
	routeRiskBudgets, err := readMap(defaults, "risk_tolerance")
	if err != nil {
		return Workflow{}, err
	}
	routeRiskRule, err := readMap(routeRiskBudgets, requestValue.RiskTolerance)
	if err != nil {
		return Workflow{}, err
	}
	routeRetryBudget, err := readInt(routeRiskRule, "retry_budget")
	if err != nil {
		return Workflow{}, err
	}

	retryRules, err := readMap(policy, "retry_rules")
	if err != nil {
		return Workflow{}, err
	}
	policyRiskRule, err := readMap(retryRules, requestValue.RiskTolerance)
	if err != nil {
		return Workflow{}, err
	}
	policyRetryBudget, err := readInt(policyRiskRule, "max_generator_retry")
	if err != nil {
		return Workflow{}, err
	}
	supervisorLoop, err := readMap(policy, "supervisor_loop")
	if err != nil {
		return Workflow{}, err
	}
	contextBudget, err := readInt(supervisorLoop, "max_context_rebuild")
	if err != nil {
		return Workflow{}, err
	}
	validationBudget, err := readInt(supervisorLoop, "max_validation_tighten")
	if err != nil {
		return Workflow{}, err
	}

	passIf, err := readStringList(policy, "pass_if")
	if err != nil {
		return Workflow{}, err
	}
	reviseIf, err := readStringList(policy, "revise_if")
	if err != nil {
		return Workflow{}, err
	}
	rejectIf, err := readStringList(policy, "reject_if")
	if err != nil {
		return Workflow{}, err
	}

	requestRelative, err := projectRelativePath(projectRoot, requestPath)
	if err != nil {
		return Workflow{}, fmt.Errorf("relativize request path: %w", err)
	}

	return Workflow{
		TaskID:                  taskID,
		TaskType:                requestValue.TaskType,
		TaskFamily:              requestValue.TaskType,
		TaskFamilySource:        "task_type",
		ProjectRoot:             projectRoot,
		Actors:                  actors,
		RubricPath:              rubricPath,
		GeneratorRetryBudget:    resolveRetryBudget(taskRetryBudget, routeRetryBudget, policyRetryBudget),
		ContextRebuildBudget:    contextBudget,
		ValidationTightenBudget: validationBudget,
		ChangedFileHints:        append([]string{}, fileHints...),
		InferredTestTargets:     append([]string{}, testTargets...),
		RequiredOutputs:         requiredOutputs,
		RequestPath:             requestRelative,
		TerminationConditions:   append([]string{}, contextContract.TerminationConditions...),
		PassIf:                  passIf,
		ReviseIf:                reviseIf,
		RejectIf:                rejectIf,
	}, nil
}

func initialState(workflow Workflow) State {
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
		QualityTrajectory:                  []QualityIteration{},
		ActorProfilesUsed:                  []ActorProfileUsed{},
	}
}

func buildExecutionPlan(
	requestValue request.CanonicalRequest,
	projectRoot string,
	policy executionPolicy,
	rules testTargetRules,
	fileHints []string,
) (ExecutionPlan, error) {
	analyzeRoots, err := normalizeProjectRelativeDirectories(projectRoot, requestValue.Context.ValidationRoots)
	if err != nil {
		return ExecutionPlan{}, fmt.Errorf("normalize context.validation_roots: %w", err)
	}
	if len(analyzeRoots) == 0 {
		analyzeRoots = inferPackageRoots(fileHints)
	}

	testTargets, err := normalizeProjectRelativeFiles(projectRoot, requestValue.Context.ValidationTargets)
	if err != nil {
		return ExecutionPlan{}, fmt.Errorf("normalize context.validation_targets: %w", err)
	}
	if len(testTargets) == 0 {
		testTargets = rules.inferTargets(projectRoot, fileHints, requestValue.Context.Feature)
	}

	var formatCommand *string
	if len(fileHints) > 0 {
		command := strings.ReplaceAll(policy.FormatCommand, "{files}", joinQuoted(fileHints))
		formatCommand = &command
	}

	var analyzeCommands []string
	switch {
	case requestValue.ValidationProfile == "smoke":
		analyzeCommands = []string{
			fmt.Sprintf("cd %s && %s", shellQuote(projectRoot), policy.SmokeAnalyze),
		}
	case len(analyzeRoots) == 0:
		analyzeCommands = []string{
			fmt.Sprintf("cd %s && %s", shellQuote(projectRoot), policy.WorkspaceAnalyze),
		}
	default:
		analyzeCommands = make([]string, 0, len(analyzeRoots))
		for _, packageRoot := range analyzeRoots {
			analyzeCommands = append(
				analyzeCommands,
				fmt.Sprintf("cd %s && %s", shellQuote(filepath.Join(projectRoot, packageRoot)), policy.PackageAnalyze),
			)
		}
	}

	var testCommands []string
	switch {
	case requestValue.ValidationProfile == "smoke":
		testCommands = []string{
			fmt.Sprintf("cd %s && %s", shellQuote(projectRoot), policy.SmokeTest),
		}
	case len(testTargets) == 0:
		testCommands = []string{
			fmt.Sprintf("cd %s && %s", shellQuote(projectRoot), policy.WorkspaceTest),
		}
	default:
		groupedTargets := groupTargetsByPackage(testTargets)
		packageRoots := make([]string, 0, len(groupedTargets))
		for packageRoot := range groupedTargets {
			packageRoots = append(packageRoots, packageRoot)
		}
		sort.Strings(packageRoots)
		for _, packageRoot := range packageRoots {
			command := strings.ReplaceAll(policy.PackageTest, "{targets}", joinQuoted(groupedTargets[packageRoot]))
			testCommands = append(
				testCommands,
				fmt.Sprintf("cd %s && %s", shellQuote(filepath.Join(projectRoot, packageRoot)), command),
			)
		}
	}

	return ExecutionPlan{
		FormatCommand:   formatCommand,
		AnalyzeCommands: analyzeCommands,
		TestCommands:    testCommands,
		testTargets:     testTargets,
	}, nil
}

func (b *Bootstrapper) materializeStaticInputs(artifactDirectory, rubricPath string) (materializedInputs, error) {
	type inputFile struct {
		Source string
		Target string
	}
	inputs := []inputFile{
		{Source: ".harness/rules/architecture_rules.md", Target: filepath.Join("inputs", "architecture_rules.md")},
		{Source: ".harness/rules/project_rules.md", Target: filepath.Join("inputs", "project_rules.md")},
		{Source: ".harness/rules/forbidden_changes.md", Target: filepath.Join("inputs", "forbidden_changes.md")},
		{Source: ".harness/supervisor/execution_policy.yaml", Target: filepath.Join("inputs", "execution_policy.yaml")},
		{Source: rubricPath, Target: filepath.Join("inputs", "rubric.yaml")},
	}
	for _, input := range inputs {
		data, err := b.loadTextAsset(input.Source)
		if err != nil {
			return materializedInputs{}, err
		}
		target := filepath.Join(artifactDirectory, input.Target)
		if err := os.WriteFile(target, []byte(data), 0o644); err != nil {
			return materializedInputs{}, fmt.Errorf("write %s: %w", input.Target, err)
		}
	}
	return materializedInputs{
		ArchitectureRulesPath: filepath.ToSlash(filepath.Join("inputs", "architecture_rules.md")),
		ProjectRulesPath:      filepath.ToSlash(filepath.Join("inputs", "project_rules.md")),
		ForbiddenChangesPath:  filepath.ToSlash(filepath.Join("inputs", "forbidden_changes.md")),
		ExecutionPolicyPath:   filepath.ToSlash(filepath.Join("inputs", "execution_policy.yaml")),
		RubricPath:            filepath.ToSlash(filepath.Join("inputs", "rubric.yaml")),
		RequestPath:           "request.yaml",
	}, nil
}

func buildWorkflowSteps(workflow Workflow, executionPlan ExecutionPlan) string {
	var builder strings.Builder
	builder.WriteString("# Workflow Steps\n\n")
	builder.WriteString(fmt.Sprintf("- Task ID: `%s`\n", workflow.TaskID))
	builder.WriteString(fmt.Sprintf("- Task type: `%s`\n", workflow.TaskType))
	builder.WriteString(fmt.Sprintf("- Target project root: `%s`\n", workflow.ProjectRoot))
	builder.WriteString(fmt.Sprintf("- Actors: `%s`\n", strings.Join(workflow.Actors, " -> ")))
	builder.WriteString(fmt.Sprintf("- Rubric: `%s`\n", workflow.RubricPath))
	builder.WriteString(fmt.Sprintf("- Generator revise budget: `%d`\n", workflow.GeneratorRetryBudget))
	builder.WriteString(fmt.Sprintf("- Context rebuild budget: `%d`\n", workflow.ContextRebuildBudget))
	builder.WriteString(fmt.Sprintf("- Validation tighten budget: `%d`\n\n", workflow.ValidationTightenBudget))

	if len(workflow.ChangedFileHints) > 0 {
		builder.WriteString("## File Hints\n")
		for _, fileHint := range workflow.ChangedFileHints {
			builder.WriteString(fmt.Sprintf("- `%s`\n", fileHint))
		}
		builder.WriteString("\n")
	}
	if len(workflow.InferredTestTargets) > 0 {
		builder.WriteString("## Test Targets\n")
		for _, target := range workflow.InferredTestTargets {
			builder.WriteString(fmt.Sprintf("- `%s`\n", target))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("## Policy\n")
	for _, condition := range workflow.PassIf {
		builder.WriteString(fmt.Sprintf("- pass_if: `%s`\n", condition))
	}
	for _, condition := range workflow.ReviseIf {
		builder.WriteString(fmt.Sprintf("- revise_if: `%s`\n", condition))
	}
	for _, condition := range workflow.RejectIf {
		builder.WriteString(fmt.Sprintf("- reject_if: `%s`\n", condition))
	}
	for _, condition := range workflow.TerminationConditions {
		builder.WriteString(fmt.Sprintf("- terminate_if: `%s`\n", condition))
	}
	builder.WriteString("\n")

	builder.WriteString("## Supervisor Actions\n")
	builder.WriteString("- `revise_generator`: request another implementation attempt with the current context.\n")
	builder.WriteString("- `rebuild_context`: refresh context artifacts before another implementation attempt.\n")
	builder.WriteString("- `tighten_validation`: reduce executor scope to the smallest credible validation set.\n")
	builder.WriteString("- `split_task`: stop orchestration and require a smaller follow-up task.\n")
	builder.WriteString("- `block_environment`: stop orchestration because tooling or environment setup prevents credible validation.\n\n")

	builder.WriteString("## Executor Commands\n")
	if executionPlan.FormatCommand != nil {
		builder.WriteString(fmt.Sprintf("- `%s`\n", *executionPlan.FormatCommand))
	}
	for _, command := range executionPlan.AnalyzeCommands {
		builder.WriteString(fmt.Sprintf("- `%s`\n", command))
	}
	for _, command := range executionPlan.TestCommands {
		builder.WriteString(fmt.Sprintf("- `%s`\n", command))
	}
	return builder.String()
}

func buildActorBrief(
	actorName string,
	actorInstructions string,
	workflow Workflow,
	executionPlan ExecutionPlan,
	contract actorContract,
	artifactDirectory string,
	materializedInputs materializedInputs,
) string {
	outputPath := filepath.Join(artifactDirectory, artifactFileName(canonicalOutputForActor(actorName)))
	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("# %s Brief\n\n", strings.ToUpper(actorName)))
	builder.WriteString(fmt.Sprintf("- Task ID: `%s`\n", workflow.TaskID))
	builder.WriteString(fmt.Sprintf("- Task type: `%s`\n", workflow.TaskType))
	builder.WriteString(fmt.Sprintf("- Target project root: `%s`\n", workflow.ProjectRoot))
	builder.WriteString(fmt.Sprintf("- Output file: `%s`\n", outputPath))
	builder.WriteString(fmt.Sprintf("- Rubric: `%s`\n\n", workflow.RubricPath))
	builder.WriteString("## Contract Inputs\n")
	for _, input := range contract.Inputs {
		builder.WriteString(fmt.Sprintf("- `%s`: `%s`\n", input, inputPathForToken(input, artifactDirectory, materializedInputs)))
	}
	builder.WriteString("\n## Continuity Inputs\n")
	builder.WriteString(fmt.Sprintf("- `work_ledger`: `%s`\n", filepath.Join(artifactDirectory, workLedgerFileName)))
	builder.WriteString(fmt.Sprintf("- `next_action`: `%s`\n", filepath.Join(artifactDirectory, nextActionFileName)))
	builder.WriteString(fmt.Sprintf("- `evidence`: `%s`\n", filepath.Join(artifactDirectory, evidenceFileName)))
	builder.WriteString("\n## Contract Outputs\n")
	for _, output := range contract.Outputs {
		builder.WriteString(fmt.Sprintf("- `%s`\n", output))
	}
	builder.WriteString("\n## Actor Instructions\n")
	builder.WriteString(actorInstructions)
	if !strings.HasSuffix(actorInstructions, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("\n## Executor Preview\n")
	preview, _ := json.MarshalIndent(executionPlan, "", "  ")
	builder.Write(preview)
	builder.WriteString("\n")
	return builder.String()
}

func inputPathForToken(token, artifactDirectory string, inputs materializedInputs) string {
	switch token {
	case "user_request", "constraints":
		return filepath.Join(artifactDirectory, inputs.RequestPath)
	case "architecture_rules":
		return filepath.Join(artifactDirectory, filepath.FromSlash(inputs.ArchitectureRulesPath))
	case "project_rules":
		return filepath.Join(artifactDirectory, filepath.FromSlash(inputs.ProjectRulesPath))
	case "forbidden_changes":
		return filepath.Join(artifactDirectory, filepath.FromSlash(inputs.ForbiddenChangesPath))
	case "execution_policy":
		return filepath.Join(artifactDirectory, filepath.FromSlash(inputs.ExecutionPolicyPath))
	case "rubric":
		return filepath.Join(artifactDirectory, filepath.FromSlash(inputs.RubricPath))
	case "plan":
		return filepath.Join(artifactDirectory, "plan.yaml")
	case "context_pack":
		return filepath.Join(artifactDirectory, "context_pack.yaml")
	case "critic_report":
		return filepath.Join(artifactDirectory, "critic_report.yaml")
	case "implementation_result":
		return filepath.Join(artifactDirectory, "implementation_result.yaml")
	case "execution_report":
		return filepath.Join(artifactDirectory, "execution_report.yaml")
	case "evaluation_result":
		return filepath.Join(artifactDirectory, "evaluation_result.yaml")
	default:
		return filepath.Join(artifactDirectory, token)
	}
}

func canonicalOutputForActor(actorName string) string {
	switch actorName {
	case "planner":
		return "plan"
	case "context_builder":
		return "context_pack"
	case "critic":
		return "critic_report"
	case "generator":
		return "implementation_result"
	case "executor":
		return "execution_report"
	case "evaluator":
		return "evaluation_result"
	default:
		return actorName
	}
}

func normalizeFileHints(projectRoot string, requestValue request.CanonicalRequest) ([]string, error) {
	fileHints := append([]string{}, requestValue.Context.SuspectedFiles...)
	fileHints = append(fileHints, requestValue.Context.RelatedFiles...)
	return normalizeProjectRelativePaths(projectRoot, fileHints)
}

func normalizeProjectRelativePaths(projectRoot string, rawPaths []string) ([]string, error) {
	deduped := map[string]struct{}{}
	for _, rawPath := range rawPaths {
		relative, err := projectRelativePath(projectRoot, rawPath)
		if err != nil {
			return nil, fmt.Errorf("relativize %s: %w", rawPath, err)
		}
		deduped[relative] = struct{}{}
	}
	normalized := make([]string, 0, len(deduped))
	for rawPath := range deduped {
		normalized = append(normalized, rawPath)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeProjectRelativeDirectories(projectRoot string, rawPaths []string) ([]string, error) {
	deduped := map[string]struct{}{}
	for _, rawPath := range rawPaths {
		resolved, err := contracts.ResolvePathWithinRoot(projectRoot, rawPath)
		if err != nil {
			return nil, err
		}
		info, err := os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", rawPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s must resolve to a directory", rawPath)
		}
		relative, err := projectRelativePath(projectRoot, resolved)
		if err != nil {
			return nil, fmt.Errorf("relativize %s: %w", rawPath, err)
		}
		deduped[relative] = struct{}{}
	}
	normalized := make([]string, 0, len(deduped))
	for rawPath := range deduped {
		normalized = append(normalized, rawPath)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func normalizeProjectRelativeFiles(projectRoot string, rawPaths []string) ([]string, error) {
	deduped := map[string]struct{}{}
	for _, rawPath := range rawPaths {
		resolved, err := contracts.ResolvePathWithinRoot(projectRoot, rawPath)
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(resolved); err == nil {
			if info.IsDir() {
				return nil, fmt.Errorf("%s must resolve to a file", rawPath)
			}
		} else if !os.IsNotExist(err) {
			return nil, fmt.Errorf("stat %s: %w", rawPath, err)
		}

		relative, err := projectRelativePath(projectRoot, resolved)
		if err != nil {
			return nil, fmt.Errorf("relativize %s: %w", rawPath, err)
		}
		deduped[relative] = struct{}{}
	}
	normalized := make([]string, 0, len(deduped))
	for rawPath := range deduped {
		normalized = append(normalized, rawPath)
	}
	sort.Strings(normalized)
	return normalized, nil
}

func projectRelativePath(projectRoot, rawPath string) (string, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	rootCanonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		rootCanonical = root
	}

	resolved, err := contracts.ResolvePathWithinRoot(projectRoot, rawPath)
	if err != nil {
		return "", err
	}

	relative, err := filepath.Rel(rootCanonical, resolved)
	if err != nil {
		return "", err
	}
	relative = filepath.Clean(relative)
	if relative != "." && (relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
		return "", fmt.Errorf("path escapes project root %s: %s", projectRoot, rawPath)
	}
	return filepath.ToSlash(relative), nil
}

func resolveRetryBudget(taskRetryBudget, routeRetryBudget, policyRetryBudget int) int {
	return min(taskRetryBudget, min(routeRetryBudget, policyRetryBudget))
}

func executionPolicyFromMap(mapValue map[string]any) (executionPolicy, error) {
	artifactsMap, err := readMap(mapValue, "artifacts")
	if err != nil {
		return executionPolicy{}, err
	}
	formatMap, err := readMap(mapValue, "format")
	if err != nil {
		return executionPolicy{}, err
	}
	analyzeMap, err := readMap(mapValue, "analyze")
	if err != nil {
		return executionPolicy{}, err
	}
	testsMap, err := readMap(mapValue, "tests")
	if err != nil {
		return executionPolicy{}, err
	}
	runtimeMap, err := readMap(mapValue, "runtime")
	if err != nil {
		return executionPolicy{}, err
	}
	artifactRoot, err := readStringWithContext(artifactsMap, "root", "execution policy artifacts")
	if err != nil {
		return executionPolicy{}, err
	}
	formatCommand, err := readStringWithContext(formatMap, "command", "execution policy format")
	if err != nil {
		return executionPolicy{}, err
	}
	packageAnalyze, err := readStringWithContext(analyzeMap, "package_command", "execution policy analyze")
	if err != nil {
		return executionPolicy{}, err
	}
	workspaceAnalyze, err := readStringWithContext(analyzeMap, "workspace_fallback", "execution policy analyze")
	if err != nil {
		return executionPolicy{}, err
	}
	smokeAnalyze, err := readStringWithContext(analyzeMap, "smoke_command", "execution policy analyze")
	if err != nil {
		return executionPolicy{}, err
	}
	packageTest, err := readStringWithContext(testsMap, "package_command", "execution policy tests")
	if err != nil {
		return executionPolicy{}, err
	}
	workspaceTest, err := readStringWithContext(testsMap, "workspace_fallback", "execution policy tests")
	if err != nil {
		return executionPolicy{}, err
	}
	smokeTest, err := readStringWithContext(testsMap, "smoke_command", "execution policy tests")
	if err != nil {
		return executionPolicy{}, err
	}
	createPlaceholders, err := readBoolWithContext(runtimeMap, "create_placeholders", "execution policy runtime")
	if err != nil {
		return executionPolicy{}, err
	}
	createActorBriefs, err := readBoolWithContext(runtimeMap, "create_actor_briefs", "execution policy runtime")
	if err != nil {
		return executionPolicy{}, err
	}
	persistJSONSnapshots, err := readBoolWithContext(runtimeMap, "persist_json_snapshots", "execution policy runtime")
	if err != nil {
		return executionPolicy{}, err
	}
	return executionPolicy{
		ArtifactRoot:         artifactRoot,
		FormatCommand:        formatCommand,
		PackageAnalyze:       packageAnalyze,
		WorkspaceAnalyze:     workspaceAnalyze,
		SmokeAnalyze:         smokeAnalyze,
		PackageTest:          packageTest,
		WorkspaceTest:        workspaceTest,
		SmokeTest:            smokeTest,
		CreatePlaceholders:   createPlaceholders,
		CreateActorBriefs:    createActorBriefs,
		PersistJSONSnapshots: persistJSONSnapshots,
	}, nil
}

func testTargetRulesFromMap(mapValue map[string]any) (testTargetRules, error) {
	naming, err := readMap(mapValue, "naming")
	if err != nil {
		return testTargetRules{}, err
	}
	fallback, err := readMap(mapValue, "fallback")
	if err != nil {
		return testTargetRules{}, err
	}
	pathRuleEntries, err := readListOfMaps(mapValue, "path_rules")
	if err != nil {
		return testTargetRules{}, err
	}
	pathRules := make([]testPathRule, 0, len(pathRuleEntries))
	for index, pathRuleEntry := range pathRuleEntries {
		sourceRoot, err := readStringWithContext(pathRuleEntry, "source_root", fmt.Sprintf("test target rules path_rules[%d]", index))
		if err != nil {
			return testTargetRules{}, err
		}
		sourceSegment, err := readStringWithContext(pathRuleEntry, "source_segment", fmt.Sprintf("test target rules path_rules[%d]", index))
		if err != nil {
			return testTargetRules{}, err
		}
		testSegment, err := readStringWithContext(pathRuleEntry, "test_segment", fmt.Sprintf("test target rules path_rules[%d]", index))
		if err != nil {
			return testTargetRules{}, err
		}
		pathRules = append(pathRules, testPathRule{
			SourceRoot:    sourceRoot,
			SourceSegment: sourceSegment,
			TestSegment:   testSegment,
		})
	}
	sourceSuffix, err := readStringWithContext(naming, "source_suffix", "test target rules naming")
	if err != nil {
		return testTargetRules{}, err
	}
	testSuffix, err := readStringWithContext(naming, "test_suffix", "test target rules naming")
	if err != nil {
		return testTargetRules{}, err
	}
	featureTestRoot, err := readStringWithContext(fallback, "feature_test_root", "test target rules fallback")
	if err != nil {
		return testTargetRules{}, err
	}
	packageTestRoot, err := readStringWithContext(fallback, "package_test_root", "test target rules fallback")
	if err != nil {
		return testTargetRules{}, err
	}
	return testTargetRules{
		SourceSuffix:    sourceSuffix,
		TestSuffix:      testSuffix,
		FeatureTestRoot: featureTestRoot,
		PackageTestRoot: packageTestRoot,
		PathRules:       pathRules,
	}, nil
}

func (r testTargetRules) inferTargets(projectRoot string, fileHints []string, featureName string) []string {
	targets := map[string]struct{}{}
	packageRoots := map[string]struct{}{}
	for _, fileHint := range fileHints {
		normalized := filepath.ToSlash(filepath.Clean(fileHint))
		segments := strings.Split(normalized, "/")
		if slices.Contains(segments, "test") {
			targets[normalized] = struct{}{}
			if len(segments) > 0 && segments[0] == "test" {
				packageRoots["."] = struct{}{}
			}
			continue
		}
		if !strings.HasSuffix(normalized, r.SourceSuffix) {
			continue
		}
		if len(segments) > 0 && segments[0] == "lib" {
			packageRoots["."] = struct{}{}
			relativeInsideSource := strings.Join(segments[1:], "/")
			testPath := filepath.ToSlash(filepath.Join("test", strings.TrimSuffix(relativeInsideSource, r.SourceSuffix)+r.TestSuffix))
			if projectPathExists(projectRoot, testPath) {
				targets[testPath] = struct{}{}
			} else {
				targets[filepath.ToSlash(filepath.Dir(testPath))] = struct{}{}
			}
			continue
		}
		for _, pathRule := range r.PathRules {
			sourceIndex := slices.Index(segments, pathRule.SourceSegment)
			if len(segments) >= 2 && segments[0] == pathRule.SourceRoot && sourceIndex >= 0 {
				packageRoot := strings.Join(segments[:sourceIndex], "/")
				packageRoots[packageRoot] = struct{}{}
				relativeInsideSource := strings.Join(segments[sourceIndex+1:], "/")
				testPath := filepath.ToSlash(filepath.Join(packageRoot, pathRule.TestSegment, strings.TrimSuffix(relativeInsideSource, r.SourceSuffix)+r.TestSuffix))
				if projectPathExists(projectRoot, testPath) {
					targets[testPath] = struct{}{}
				} else {
					targets[filepath.ToSlash(filepath.Dir(testPath))] = struct{}{}
				}
				break
			}
		}
	}
	if len(targets) == 0 && strings.TrimSpace(featureName) != "" {
		rootCandidate := filepath.ToSlash(filepath.Join(r.PackageTestRoot, featureName))
		if projectPathExists(projectRoot, rootCandidate) {
			targets[rootCandidate] = struct{}{}
		}
		appCandidate := filepath.ToSlash(filepath.Join("apps", featureName, r.FeatureTestRoot))
		if projectPathExists(projectRoot, appCandidate) {
			targets[appCandidate] = struct{}{}
		}
		for packageRoot := range packageRoots {
			packageCandidate := filepath.ToSlash(filepath.Join(packageRoot, r.PackageTestRoot))
			if projectPathExists(projectRoot, packageCandidate) {
				targets[packageCandidate] = struct{}{}
			}
		}
	}
	results := make([]string, 0, len(targets))
	for target := range targets {
		results = append(results, target)
	}
	sort.Strings(results)
	return results
}

func contextContractFromMap(mapValue map[string]any) (contextContract, error) {
	actorFlowSchema, err := readMap(mapValue, "actor_flow_schema")
	if err != nil {
		return contextContract{}, err
	}
	contracts := map[string]actorContract{}
	for actorName, raw := range actorFlowSchema {
		actorMap, ok := raw.(map[string]any)
		if !ok {
			return contextContract{}, fmt.Errorf("expected actor contract for %s", actorName)
		}
		inputs, err := readStringList(actorMap, "in")
		if err != nil {
			return contextContract{}, err
		}
		outputs, err := readStringList(actorMap, "out")
		if err != nil {
			return contextContract{}, err
		}
		contracts[actorName] = actorContract{Inputs: inputs, Outputs: outputs}
	}
	termination, err := readMap(mapValue, "termination")
	if err != nil {
		return contextContract{}, err
	}
	conditions, err := readStringList(termination, "conditions")
	if err != nil {
		return contextContract{}, err
	}
	return contextContract{
		ActorContracts:        contracts,
		TerminationConditions: conditions,
	}, nil
}

func (c contextContract) contractFor(actorName string) (actorContract, error) {
	contract, ok := c.ActorContracts[actorName]
	if !ok {
		return actorContract{}, fmt.Errorf("missing actor contract for %s", actorName)
	}
	return contract, nil
}

func inferPackageRoots(fileHints []string) []string {
	packageRoots := map[string]struct{}{}
	for _, fileHint := range fileHints {
		segments := strings.Split(filepath.ToSlash(filepath.Clean(fileHint)), "/")
		if len(segments) >= 2 && (segments[0] == "apps" || segments[0] == "packages") {
			packageRoots[filepath.ToSlash(filepath.Join(segments[0], segments[1]))] = struct{}{}
			continue
		}
		if len(segments) >= 2 && (segments[0] == "cmd" || segments[0] == "internal" || segments[0] == "pkg") {
			packageRoots[filepath.ToSlash(filepath.Join(segments[0], segments[1]))] = struct{}{}
			continue
		}
		if len(segments) > 0 && (segments[0] == "lib" || segments[0] == "test") {
			packageRoots["."] = struct{}{}
		}
	}
	results := make([]string, 0, len(packageRoots))
	for packageRoot := range packageRoots {
		results = append(results, packageRoot)
	}
	sort.Strings(results)
	return results
}

func groupTargetsByPackage(targets []string) map[string][]string {
	grouped := map[string][]string{}
	for _, target := range targets {
		normalized := filepath.ToSlash(filepath.Clean(target))
		segments := strings.Split(normalized, "/")
		if len(segments) >= 2 && (segments[0] == "apps" || segments[0] == "packages") {
			packageRoot := filepath.ToSlash(filepath.Join(segments[0], segments[1]))
			localTarget := filepath.ToSlash(strings.TrimPrefix(normalized, packageRoot+"/"))
			grouped[packageRoot] = append(grouped[packageRoot], localTarget)
			continue
		}
		if len(segments) >= 2 && strings.HasSuffix(normalized, ".go") {
			packageRoot := filepath.ToSlash(filepath.Dir(normalized))
			grouped[packageRoot] = append(grouped[packageRoot], ".")
			continue
		}
		if len(segments) >= 1 && !strings.Contains(filepath.Base(normalized), ".") {
			grouped[normalized] = append(grouped[normalized], ".")
			continue
		}
		grouped["."] = append(grouped["."], normalized)
	}
	for packageRoot := range grouped {
		sort.Strings(grouped[packageRoot])
	}
	return grouped
}

func projectPathExists(projectRoot, relativePath string) bool {
	_, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(relativePath)))
	return err == nil
}

func joinQuoted(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, shellQuote(value))
	}
	return strings.Join(quoted, " ")
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

func (b *Bootstrapper) loadTextAsset(relPath string) (string, error) {
	data, _, err := assets.Resolve(b.projectRoot, relPath)
	if err != nil {
		return "", fmt.Errorf("load %s: %w", relPath, err)
	}
	return string(data), nil
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
	case "critic_report":
		return map[string]any{
			"priority_focus":          []string{},
			"missing_requirements":    []string{},
			"risk_hypotheses":         []string{},
			"validation_expectations": []string{},
			"generator_guardrails":    []string{},
			"blocked_assumptions":     []string{},
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
			"approved_memory_consideration": map[string]any{
				"considered_ref":                     "",
				"lookup_key":                         "",
				"task_family_source":                 "",
				"disposition":                        "drop",
				"reasons":                            []string{},
				"originating_candidate_refs":         []string{},
				"current_state_refresh_ref":          "",
				"current_state_refresh_generated_at": nil,
			},
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
	case "critic_report":
		return "critic_report.yaml"
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

func readListOfMaps(source map[string]any, key string) ([]map[string]any, error) {
	value, ok := source[key]
	if !ok {
		return nil, fmt.Errorf("missing `%s`", key)
	}
	listValue, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected `%s` to be a list", key)
	}
	result := make([]map[string]any, 0, len(listValue))
	for _, entry := range listValue {
		mapEntry, ok := entry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("expected `%s` entries to be maps", key)
		}
		result = append(result, mapEntry)
	}
	return result, nil
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

func readStringWithContext(source map[string]any, key, context string) (string, error) {
	value, err := readString(source, key)
	if err != nil {
		return "", fmt.Errorf("%s.%s: %w", context, key, err)
	}
	return value, nil
}

func mustReadString(source map[string]any, key string) string {
	text, err := readString(source, key)
	if err != nil {
		panic(err)
	}
	return text
}

func readIntFromMap(source map[string]any, key, nested string) (int, error) {
	mapValue, err := readMap(source, key)
	if err != nil {
		return 0, err
	}
	return readInt(mapValue, nested)
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

func readBool(source map[string]any, key string) (bool, error) {
	value, ok := source[key]
	if !ok {
		return false, fmt.Errorf("missing `%s`", key)
	}
	boolValue, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("expected `%s` to be a bool", key)
	}
	return boolValue, nil
}

func readBoolWithContext(source map[string]any, key, context string) (bool, error) {
	value, err := readBool(source, key)
	if err != nil {
		return false, fmt.Errorf("%s.%s: %w", context, key, err)
	}
	return value, nil
}

func mustReadBool(source map[string]any, key string) bool {
	value, err := readBool(source, key)
	if err != nil {
		panic(err)
	}
	return value
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
