import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:yaml/yaml.dart';

void main(List<String> args) async {
  final command = args.isEmpty ? 'help' : args.first;
  final runner = HarnessRunner(_resolveScriptRoot());

  switch (command) {
    case 'init-request':
      final outputPath =
          _readOption(args.skip(1).toList(), '--output') ??
          '.harness/request.template.yaml';
      await runner.initRequestTemplate(outputPath);
      return;
    case 'compose-request':
      final goal = _readRequiredOption(
        args.skip(1).toList(),
        '--goal',
        usageSink: stderr,
      );
      final outputPath = _readOption(args.skip(1).toList(), '--output');
      final taskType = _readRequiredOption(
        args.skip(1).toList(),
        '--task-type',
        usageSink: stderr,
      );
      final feature = _readOption(args.skip(1).toList(), '--feature');
      final riskTolerance =
          _readOption(args.skip(1).toList(), '--risk-tolerance') ?? 'low';
      final priority = _readOption(args.skip(1).toList(), '--priority') ?? 'medium';
      final validationProfile =
          _readOption(args.skip(1).toList(), '--validation-profile') ??
          'standard';
      final constraints = _readMultiOption(args.skip(1).toList(), '--constraint');
      final definitionOfDone = _readMultiOption(args.skip(1).toList(), '--dod');
      final suspectedFiles = _readMultiOption(
        args.skip(1).toList(),
        '--suspected-file',
      );
      final relatedFiles = _readMultiOption(
        args.skip(1).toList(),
        '--related-file',
      );
      final validationRoots = _readMultiOption(
        args.skip(1).toList(),
        '--validation-root',
      );
      final validationTargets = _readMultiOption(
        args.skip(1).toList(),
        '--validation-target',
      );
      final composedRequest = await runner.composeRequest(
        goal: goal,
        outputPath: outputPath,
        taskType: taskType,
        feature: feature,
        riskTolerance: riskTolerance,
        priority: priority,
        validationProfile: validationProfile,
        constraints: constraints,
        definitionOfDone: definitionOfDone,
        suspectedFiles: suspectedFiles,
        relatedFiles: relatedFiles,
        validationRoots: validationRoots,
        validationTargets: validationTargets,
      );
      stdout.writeln(
        'Request composed at ${p.relative(composedRequest.file.path, from: runner.root.path)}',
      );
      stdout.writeln(
        'Inferred task_type=${composedRequest.request['task_type']} risk_tolerance=${composedRequest.request['risk_tolerance']} priority=${composedRequest.request['priority']}',
      );
      return;
    case 'validate-request':
      final requestPath = _readRequiredOption(
        args.skip(1).toList(),
        '--request',
        usageSink: stderr,
      );
      await runner.validateRequest(requestPath);
      return;
    case 'validate-artifact':
      final filePath = _readRequiredOption(
        args.skip(1).toList(),
        '--file',
        usageSink: stderr,
      );
      final schemaName = _readRequiredOption(
        args.skip(1).toList(),
        '--schema',
        usageSink: stderr,
      );
      await runner.validateArtifact(filePath: filePath, schemaName: schemaName);
      return;
    case 'run':
      final requestPath = _readRequiredOption(
        args.skip(1).toList(),
        '--request',
        usageSink: stderr,
      );
      final projectRoot = _readRequiredOption(
        args.skip(1).toList(),
        '--project-root',
        usageSink: stderr,
      );
      final taskId = _readOption(args.skip(1).toList(), '--task-id');
      final force = args.skip(1).contains('--force');
      final artifactPath = await runner.run(
        requestPath: requestPath,
        projectRoot: projectRoot,
        taskId: taskId,
        force: force,
      );
      stdout.writeln(
        'Harness artifacts created at ${p.relative(artifactPath, from: runner.root.path)}',
      );
      return;
    case 'execute':
      final artifactPath = _readRequiredOption(
        args.skip(1).toList(),
        '--artifact',
        usageSink: stderr,
      );
      final projectRoot = _readOption(args.skip(1).toList(), '--project-root');
      final throughActor = _readOption(args.skip(1).toList(), '--through');
      final result = await runner.execute(
        artifactPath: artifactPath,
        projectRoot: projectRoot,
        throughActor: throughActor,
      );
      stdout.writeln(result);
      return;
    case 'route-evaluation':
      final artifactPath = _readRequiredOption(
        args.skip(1).toList(),
        '--artifact',
        usageSink: stderr,
      );
      final result = await runner.routeEvaluation(artifactPath: artifactPath);
      stdout.writeln(result);
      return;
    default:
      _printUsage(stdout);
  }
}

void _printUsage(IOSink sink) {
  sink.writeln('Usage:');
  sink.writeln(
    '  dart run bin/rail.dart init-request [--output <path>]',
  );
  sink.writeln(
    '  dart run bin/rail.dart compose-request --goal <text> --task-type <bug_fix|feature_addition|safe_refactor|test_repair> [--feature <name>] [--suspected-file <path>] [--related-file <path>] [--validation-root <path>] [--validation-target <path>] [--constraint <text>] [--dod <text>] [--risk-tolerance <low|medium|high>] [--priority <low|medium|high>] [--validation-profile <standard|smoke>] [--output <path>]',
  );
  sink.writeln(
    '  dart run bin/rail.dart validate-request --request <path>',
  );
  sink.writeln(
    '  dart run bin/rail.dart validate-artifact --file <path> --schema <request|plan|context_pack|implementation_result|execution_report|evaluation_result|integration_result>',
  );
  sink.writeln(
    '  dart run bin/rail.dart run --request <path> --project-root <path> [--task-id <id>] [--force]',
  );
  sink.writeln(
    '  dart run bin/rail.dart execute --artifact <path> [--project-root <path>] [--through <actor>]',
  );
  sink.writeln(
    '  dart run bin/rail.dart route-evaluation --artifact <path>',
  );
}

Directory _resolveScriptRoot() {
  final scriptFile = File.fromUri(Platform.script);
  return scriptFile.parent.parent;
}

String? _readOption(List<String> args, String name) {
  for (var index = 0; index < args.length - 1; index++) {
    if (args[index] == name) {
      return args[index + 1];
    }
  }
  return null;
}

List<String> _readMultiOption(List<String> args, String name) {
  final values = <String>[];
  for (var index = 0; index < args.length - 1; index++) {
    if (args[index] == name) {
      values.add(args[index + 1]);
    }
  }
  return values;
}

String _readRequiredOption(
  List<String> args,
  String name, {
  required IOSink usageSink,
}) {
  final value = _readOption(args, name);
  if (value == null || value.isEmpty) {
    _printUsage(usageSink);
    throw ArgumentError('Missing required option: $name');
  }
  return value;
}

bool _pathExists(String path) {
  return FileSystemEntity.typeSync(path, followLinks: false) !=
      FileSystemEntityType.notFound;
}

bool _projectPathExists(String projectRoot, String relativePath) {
  final candidate = p.normalize(p.join(projectRoot, relativePath));
  return _pathExists(candidate);
}

class HarnessRunner {
  HarnessRunner(this.root);

  final Directory root;
  late final String _rootCanonicalPath = Directory(
    root.path,
  ).resolveSymbolicLinksSync();

  Future<void> initRequestTemplate(String outputPath) async {
    final templateFile = File(
      p.join(root.path, '.harness', 'templates', 'request.template.yaml'),
    );
    if (!templateFile.existsSync()) {
      throw StateError('Missing request template at ${templateFile.path}');
    }

    final outputFile = _resolveFileWithinRoot(outputPath);
    await outputFile.parent.create(recursive: true);
    await outputFile.writeAsString(await templateFile.readAsString());
    stdout.writeln(
      'Request template written to ${p.relative(outputFile.path, from: root.path)}',
    );
  }

  Future<ComposedRequest> composeRequest({
    required String goal,
    required String? outputPath,
    required String taskType,
    required String? feature,
    required String riskTolerance,
    required String priority,
    required String validationProfile,
    required List<String> constraints,
    required List<String> definitionOfDone,
    required List<String> suspectedFiles,
    required List<String> relatedFiles,
    required List<String> validationRoots,
    required List<String> validationTargets,
  }) async {
    final effectiveOutputPath =
        outputPath ?? '.harness/requests/${_requestFileName(goal)}';
    final outputFile = _resolveFileWithinRoot(effectiveOutputPath);

    final normalizedConstraints = constraints;
    final normalizedDefinitionOfDone = definitionOfDone.isEmpty
        ? <String>[
            '요청한 동작이 재현 가능하게 충족된다',
            '관련 테스트 또는 영향 범위 검토가 가능하다',
            'analyze 기준을 충족한다',
          ]
        : definitionOfDone;

    final requestMap = <String, Object?>{
      'task_type': taskType,
      'goal': goal,
      'context': <String, Object?>{
        if (feature != null && feature.isNotEmpty) 'feature': feature,
        if (suspectedFiles.isNotEmpty) 'suspected_files': suspectedFiles,
        if (relatedFiles.isNotEmpty) 'related_files': relatedFiles,
        if (validationRoots.isNotEmpty) 'validation_roots': validationRoots,
        if (validationTargets.isNotEmpty)
          'validation_targets': validationTargets,
      },
      'constraints': normalizedConstraints,
      'definition_of_done': normalizedDefinitionOfDone,
      'priority': priority,
      'risk_tolerance': riskTolerance,
      'validation_profile': validationProfile,
    };

    final requestSchema = _loadSchema('request');
    requestSchema.validate(requestMap, fileLabel: effectiveOutputPath);
    UserRequest.fromMap(
      requestMap.cast<String, dynamic>(),
      requestPath: effectiveOutputPath,
    );

    await outputFile.parent.create(recursive: true);
    await outputFile.writeAsString(_toYaml(requestMap));
    return ComposedRequest(file: outputFile, request: requestMap);
  }

  Future<void> validateRequest(String requestPath) async {
    final requestFile = _resolveFileWithinRoot(requestPath);
    if (!requestFile.existsSync()) {
      throw ArgumentError('Request file not found: $requestPath');
    }

    final requestSchema = _loadSchema('request');
    final requestValue = _loadYamlValue(requestFile);
    requestSchema.validate(requestValue, fileLabel: requestPath);
    UserRequest.fromMap(
      _asMap(requestValue, context: 'request'),
      requestPath: p.relative(requestFile.path, from: root.path),
    );
    stdout.writeln('Request is valid: $requestPath');
  }

  Future<void> validateArtifact({
    required String filePath,
    required String schemaName,
  }) async {
    final artifactFile = _resolveFileWithinRoot(filePath);
    if (!artifactFile.existsSync()) {
      throw ArgumentError('Artifact file not found: $filePath');
    }

    final schema = _loadSchema(schemaName);
    final value = _loadYamlValue(artifactFile);
    schema.validate(value, fileLabel: filePath);
    stdout.writeln('Artifact is valid for `$schemaName`: $filePath');
  }

  Future<String> run({
    required String requestPath,
    required String projectRoot,
    required bool force,
    String? taskId,
  }) async {
    final rawRequestFile = _resolveFileWithinRoot(requestPath);
    final projectDirectory = _resolveProjectDirectory(projectRoot);
    if (!rawRequestFile.existsSync()) {
      throw ArgumentError('Request file not found: $requestPath');
    }

    final requestSchema = _loadSchema('request');
    final requestValue = _loadYamlValue(rawRequestFile);
    requestSchema.validate(requestValue, fileLabel: requestPath);
    final userRequest = UserRequest.fromMap(
      _asMap(requestValue, context: 'request'),
      requestPath: p.relative(rawRequestFile.path, from: root.path),
    );

    final registry = Registry.fromMap(
      _loadYamlMap('.harness/supervisor/registry.yaml'),
    );
    final taskRouter = TaskRouter.fromMap(
      _loadYamlMap('.harness/supervisor/task_router.yaml'),
    );
    final policy = Policy.fromMap(
      _loadYamlMap('.harness/supervisor/policy.yaml'),
    );
    final executionPolicy = ExecutionPolicy.fromMap(
      _loadYamlMap('.harness/supervisor/execution_policy.yaml'),
    );
    final testRules = TestTargetRules.fromMap(
      _loadYamlMap('.harness/supervisor/test_target_rules.yaml'),
    );
    final contextContract = ContextContract.fromMap(
      _loadYamlMap('.harness/supervisor/context_contract.yaml'),
    );

    final taskConfig = registry.taskFor(userRequest.taskType);
    final route = taskRouter.routeFor(userRequest.taskType);
    _validateRouterConsistency(
      taskType: userRequest.taskType,
      taskConfig: taskConfig,
      route: route,
    );
    _validateContextContractConsistency(
      taskType: userRequest.taskType,
      taskConfig: taskConfig,
      contextContract: contextContract,
    );

    final effectiveTaskId = _sanitizeTaskId(taskId ?? _defaultTaskId(userRequest));
    final artifactDirectory = _resolveArtifactDirectory(
      artifactRoot: executionPolicy.artifactRoot,
      taskId: effectiveTaskId,
    );
    if (artifactDirectory.existsSync() &&
        artifactDirectory.listSync().isNotEmpty &&
        !force) {
      throw StateError(
        'Artifact directory already exists and is not empty: ${p.relative(artifactDirectory.path, from: root.path)}. Use --force to overwrite.',
      );
    }

    if (artifactDirectory.existsSync() && force) {
      await artifactDirectory.delete(recursive: true);
    }

    await artifactDirectory.create(recursive: true);
    final inputsDirectory = Directory(p.join(artifactDirectory.path, 'inputs'));
    await inputsDirectory.create(recursive: true);
    if (executionPolicy.createActorBriefs) {
      await Directory(
        p.join(artifactDirectory.path, 'actor_briefs'),
      ).create(recursive: true);
    }

    final fileHints = [
      ...userRequest.context.suspectedFiles,
      ...userRequest.context.relatedFiles,
    ].map(_normalizeRepoRelativePath).toSet().toList()
      ..sort();

    final testTargets = userRequest.context.validationTargets.isNotEmpty
        ? userRequest.context.validationTargets
        : testRules.inferTargets(
            projectRoot: projectDirectory.path,
            fileHints: fileHints,
            featureName: userRequest.context.feature,
          );
    final executionPlan = _buildExecutionPlan(
      userRequest: userRequest,
      projectRoot: projectDirectory.path,
      executionPolicy: executionPolicy,
      testRules: testRules,
      fileHints: fileHints,
    );

    final resolvedWorkflow = ResolvedWorkflow(
      taskId: effectiveTaskId,
      taskType: userRequest.taskType,
      projectRoot: projectDirectory.path,
      actors: taskConfig.actors,
      rubricPath: taskConfig.rubric,
      generatorRetryBudget: _resolveRetryBudget(
        taskRetryBudget: taskConfig.generatorMaxRetry,
        routeRetryBudget: route.retryBudgetFor(userRequest.riskTolerance),
        policyRetryBudget: policy.retryBudgetFor(userRequest.riskTolerance),
      ),
      contextRebuildBudget: policy.contextRebuildBudget,
      validationTightenBudget: policy.validationTightenBudget,
      changedFileHints: fileHints,
      inferredTestTargets: testTargets,
      requiredOutputs: taskConfig.requiredOutputs,
      requestPath: userRequest.requestPath,
      terminationConditions: contextContract.terminationConditions,
      passIf: policy.passIf,
      reviseIf: policy.reviseIf,
      rejectIf: policy.rejectIf,
    );

    await File(
      p.join(artifactDirectory.path, 'request.yaml'),
    ).writeAsString(await rawRequestFile.readAsString());
    final materializedInputs = await _materializeStaticInputs(
      artifactDirectory: artifactDirectory,
      inputsDirectory: inputsDirectory,
      taskConfig: taskConfig,
    );

    if (executionPolicy.persistJsonSnapshots) {
      await File(
        p.join(artifactDirectory.path, 'resolved_workflow.json'),
      ).writeAsString(
        const JsonEncoder.withIndent('  ').convert(resolvedWorkflow.toJson()),
      );
      await File(
        p.join(artifactDirectory.path, 'execution_plan.json'),
      ).writeAsString(
        const JsonEncoder.withIndent('  ').convert(executionPlan.toJson()),
      );
      await File(p.join(artifactDirectory.path, 'state.json')).writeAsString(
        const JsonEncoder.withIndent('  ').convert(
          {
            'taskId': effectiveTaskId,
            'status': 'initialized',
            'currentActor': taskConfig.actors.firstOrNull,
            'completedActors': <String>[],
            'generatorRetriesRemaining': resolvedWorkflow.generatorRetryBudget,
            'contextRebuildsRemaining': resolvedWorkflow.contextRebuildBudget,
            'validationTighteningsRemaining':
                resolvedWorkflow.validationTightenBudget,
            'lastDecision': null,
            'lastReasonCodes': <String>[],
            'actionHistory': <String>[],
          },
        ),
      );
    }

    if (executionPolicy.createPlaceholders) {
      for (final outputName in taskConfig.requiredOutputs) {
        final outputPath = _artifactFilePath(artifactDirectory.path, outputName);
        await File(outputPath).writeAsString(_placeholderContent(outputName));
      }
    }

    await File(
      p.join(artifactDirectory.path, 'workflow_steps.md'),
    ).writeAsString(
      _buildWorkflowSteps(
        workflow: resolvedWorkflow,
        executionPlan: executionPlan,
      ),
    );

    if (executionPolicy.createActorBriefs) {
      for (var index = 0; index < taskConfig.actors.length; index++) {
        final actorName = taskConfig.actors[index];
        final actorDoc = File(
          p.join(root.path, '.harness', 'actors', '$actorName.md'),
        );
        final actorInstructions = actorDoc.existsSync()
            ? await actorDoc.readAsString()
            : 'Actor instructions not found.';
        final actorBriefPath = p.join(
          artifactDirectory.path,
          'actor_briefs',
          '${(index + 1).toString().padLeft(2, '0')}_$actorName.md',
        );
        await File(actorBriefPath).writeAsString(
          _buildActorBrief(
            actorName: actorName,
            actorInstructions: actorInstructions,
            workflow: resolvedWorkflow,
            executionPlan: executionPlan,
            actorContract: contextContract.contractFor(actorName),
            artifactDirectory: artifactDirectory,
            materializedInputs: materializedInputs,
          ),
        );
      }
    }

    return artifactDirectory.path;
  }

  Future<String> execute({
    required String artifactPath,
    String? projectRoot,
    String? throughActor,
  }) async {
    final artifactDirectory = Directory(
      p.isAbsolute(artifactPath)
          ? p.normalize(artifactPath)
          : p.normalize(p.join(root.path, artifactPath)),
    );
    _assertPathWithinRoot(artifactDirectory.path, isDirectory: true);
    if (!artifactDirectory.existsSync()) {
      throw ArgumentError('Artifact directory not found: $artifactPath');
    }

    final workflow = ResolvedWorkflow.fromJson(
      _readJsonFile(p.join(artifactDirectory.path, 'resolved_workflow.json')),
    );
    final userRequest = UserRequest.fromMap(
      _asMap(
        _loadYamlValue(File(p.join(artifactDirectory.path, 'request.yaml'))),
        context: p.join(artifactDirectory.path, 'request.yaml'),
      ),
      requestPath: workflow.requestPath,
    );
    final projectDirectory = _resolveProjectDirectory(
      projectRoot ?? workflow.projectRoot,
    );
    final executionPolicy = ExecutionPolicy.fromMap(
      _loadYamlMap('.harness/supervisor/execution_policy.yaml'),
    );
    final testRules = TestTargetRules.fromMap(
      _loadYamlMap('.harness/supervisor/test_target_rules.yaml'),
    );
    final contextContract = ContextContract.fromMap(
      _loadYamlMap('.harness/supervisor/context_contract.yaml'),
    );
    var executionPlan = ExecutionPlan.fromJson(
      _readJsonFile(p.join(artifactDirectory.path, 'execution_plan.json')),
    );
    final stateFile = File(p.join(artifactDirectory.path, 'state.json'));
    final state = HarnessState.fromJson(_readJsonFile(stateFile.path));

    if (state.currentActor == null) {
      return 'Harness execution already completed for ${p.relative(artifactDirectory.path, from: root.path)}';
    }

    final stopActor = throughActor;
    if (stopActor != null && !workflow.actors.contains(stopActor)) {
      throw ArgumentError('Unknown actor `$stopActor` for task `${workflow.taskType}`.');
    }

    final runsDirectory = Directory(p.join(artifactDirectory.path, 'runs'));
    await runsDirectory.create(recursive: true);

    var currentState = state;
    while (currentState.currentActor != null) {
      final actorName = currentState.currentActor!;
      final actorIndex = workflow.actors.indexOf(actorName);
      final actorBriefPath = p.join(
        artifactDirectory.path,
        'actor_briefs',
        '${(actorIndex + 1).toString().padLeft(2, '0')}_$actorName.md',
      );
      final outputName = _canonicalOutputForActor(actorName);
      final schemaName = _schemaNameForActor(actorName);
      final outputPath = outputName == null
          ? null
          : _artifactFilePath(artifactDirectory.path, outputName);
      final logPath = p.join(
        runsDirectory.path,
        '${(actorIndex + 1).toString().padLeft(2, '0')}_$actorName-last-message.txt',
      );
      final schemaPath = schemaName == null
          ? null
          : await _materializeOutputSchema(
              schemaName: schemaName,
              runsDirectory: runsDirectory,
              actorIndex: actorIndex,
              actorName: actorName,
            );

      executionPlan = await _refreshStandardExecutionPlanIfNeeded(
        actorName: actorName,
        artifactDirectory: artifactDirectory,
        workflow: workflow,
        userRequest: userRequest,
        executionPlan: executionPlan,
        executionPolicy: executionPolicy,
        testRules: testRules,
        contextContract: contextContract,
        actorIndex: actorIndex,
        projectRoot: projectDirectory.path,
      );
      final priorExecutionPlan = executionPlan;
      executionPlan = await _tightenExecutionPlanIfNeeded(
        actorName: actorName,
        artifactDirectory: artifactDirectory,
        workflow: workflow,
        userRequest: userRequest,
        executionPlan: executionPlan,
        executionPolicy: executionPolicy,
        testRules: testRules,
        contextContract: contextContract,
        actorIndex: actorIndex,
        projectRoot: projectDirectory.path,
        state: currentState,
      );
      if (actorName == 'executor' &&
          currentState.status == 'tightening_validation' &&
          _sameExecutionPlan(priorExecutionPlan, executionPlan)) {
        currentState = currentState.copyWith(
          status: 'evolution_exhausted',
          clearCurrentActor: true,
          lastReasonCodes: [
            ...currentState.lastReasonCodes,
            'tighten_validation_noop',
          ],
        );
        await stateFile.writeAsString(
          const JsonEncoder.withIndent('  ').convert(currentState.toJson()),
        );
        await _writeTerminalOutcomeReport(
          artifactDirectory: artifactDirectory,
          state: currentState,
        );
        break;
      }

      final fastPathResponse = await _runFastPathActor(
        actorName: actorName,
        artifactDirectory: artifactDirectory,
        workflow: workflow,
        executionPlan: executionPlan,
        userRequest: userRequest,
        outputPath: outputPath,
        logPath: logPath,
        projectRoot: projectDirectory.path,
      );
      if (fastPathResponse != null) {
        await _refreshActorBriefs(
          artifactDirectory: artifactDirectory,
          workflow: workflow,
          executionPlan: executionPlan,
          contextContract: contextContract,
          startIndex: actorIndex + 1,
        );
        if (schemaName != null && outputPath != null) {
          await validateArtifact(
            filePath: p.relative(outputPath, from: root.path),
            schemaName: schemaName,
          );
        }

        currentState = _advanceState(
          state: currentState,
          workflow: workflow,
          actorName: actorName,
          artifactDirectory: artifactDirectory,
        );
        await stateFile.writeAsString(
          const JsonEncoder.withIndent('  ').convert(currentState.toJson()),
        );

        if (stopActor != null && actorName == stopActor) {
          break;
        }
        if (_shouldTerminate(currentState)) {
          break;
        }
        continue;
      }

      if (userRequest.validationProfile == 'smoke') {
        final smokeResponse = await _runSmokeActor(
          actorName: actorName,
          artifactDirectory: artifactDirectory,
          workflow: workflow,
          executionPlan: executionPlan,
          userRequest: userRequest,
          outputPath: outputPath,
          logPath: logPath,
          projectRoot: projectDirectory.path,
        );
        if (smokeResponse != null) {
          if (schemaName != null && outputPath != null) {
            await File(outputPath).writeAsString(_toYaml(smokeResponse));
            await validateArtifact(
              filePath: p.relative(outputPath, from: root.path),
              schemaName: schemaName,
            );
          }

          currentState = _advanceState(
            state: currentState,
            workflow: workflow,
            actorName: actorName,
            artifactDirectory: artifactDirectory,
          );
          await stateFile.writeAsString(
            const JsonEncoder.withIndent('  ').convert(currentState.toJson()),
          );
          if (actorName == 'evaluator') {
            await _appendSupervisorDecisionTrace(
              artifactDirectory: artifactDirectory,
              state: currentState,
            );
          }

          if (stopActor != null && actorName == stopActor) {
            break;
          }
          if (_shouldTerminate(currentState)) {
            break;
          }
          continue;
        }
      }

      final actorWorkingDirectory = _actorWorkingDirectory(
        actorName: actorName,
        artifactDirectory: artifactDirectory,
        projectRoot: projectDirectory.path,
      );

      final prompt = _buildCodexExecutionPrompt(
        actorName: actorName,
        actorBriefPath: actorBriefPath,
        artifactDirectory: artifactDirectory.path,
        projectRoot: projectDirectory.path,
        actorWorkingDirectory: actorWorkingDirectory,
        outputPath: outputPath,
        returnsStructuredOutput: schemaPath != null,
      );

      final result = await _runCommand(
        'codex',
        [
          'exec',
          '--full-auto',
          '--ephemeral',
          '--color',
          'never',
          '--skip-git-repo-check',
          '-c',
          'reasoning_effort="low"',
          '-c',
          'sandbox_mode="danger-full-access"',
          '-c',
          'approval_policy="never"',
          if (schemaPath != null) ...[
            '--output-schema',
            schemaPath,
          ],
          '--output-last-message',
          logPath,
          prompt,
        ],
        workingDirectory: actorWorkingDirectory,
        timeout: const Duration(minutes: 5),
      );

      if (result.exitCode != 0) {
        final timeoutMessage = result.exitCode == -1
            ? 'Timed out while executing actor `$actorName`.'
            : 'Actor `$actorName` failed with exit code ${result.exitCode}.';
        throw StateError(
          '$timeoutMessage\n${result.stderr.isEmpty ? result.stdout : result.stderr}',
        );
      }

      if (schemaName != null && outputPath != null) {
        var responseObject = _decodeStructuredResponse(
          filePath: logPath,
          fallbackText: result.stdout,
        );
        if (schemaName == 'execution_report') {
          responseObject = _normalizeExecutionReport(
            report: responseObject,
            artifactDirectory: artifactDirectory,
            actorName: actorName,
          );
        }
        await File(outputPath).writeAsString(_toYaml(responseObject));
        await validateArtifact(
          filePath: p.relative(outputPath, from: root.path),
          schemaName: schemaName,
        );
      }

      currentState = _advanceState(
        state: currentState,
        workflow: workflow,
        actorName: actorName,
        artifactDirectory: artifactDirectory,
      );
      await stateFile.writeAsString(
        const JsonEncoder.withIndent('  ').convert(currentState.toJson()),
      );
      if (actorName == 'evaluator') {
        await _appendSupervisorDecisionTrace(
          artifactDirectory: artifactDirectory,
          state: currentState,
        );
      }

      if (stopActor != null && actorName == stopActor) {
        break;
      }
      if (_shouldTerminate(currentState)) {
        await _writeTerminalOutcomeReport(
          artifactDirectory: artifactDirectory,
          state: currentState,
        );
        break;
      }
    }

    return _formatExecutionSummary(
      artifactDirectory: artifactDirectory,
      state: currentState,
    );
  }

  Future<String> routeEvaluation({
    required String artifactPath,
  }) async {
    final artifactDirectory = Directory(
      p.isAbsolute(artifactPath)
          ? p.normalize(artifactPath)
          : p.normalize(p.join(root.path, artifactPath)),
    );
    _assertPathWithinRoot(artifactDirectory.path, isDirectory: true);
    if (!artifactDirectory.existsSync()) {
      throw ArgumentError('Artifact directory not found: $artifactPath');
    }

    final workflow = ResolvedWorkflow.fromJson(
      _readJsonFile(p.join(artifactDirectory.path, 'resolved_workflow.json')),
    );
    final stateFile = File(p.join(artifactDirectory.path, 'state.json'));
    final state = HarnessState.fromJson(_readJsonFile(stateFile.path));
    if (state.currentActor != 'evaluator' || _shouldTerminate(state)) {
      return 'Harness evaluation routing skipped for ${p.relative(artifactDirectory.path, from: root.path)} (currentActor=${state.currentActor ?? 'none'}, status=${state.status})';
    }
    final nextState = _advanceState(
      state: state,
      workflow: workflow,
      actorName: 'evaluator',
      artifactDirectory: artifactDirectory,
    );
    await stateFile.writeAsString(
      const JsonEncoder.withIndent('  ').convert(nextState.toJson()),
    );
    await _appendSupervisorDecisionTrace(
      artifactDirectory: artifactDirectory,
      state: nextState,
    );
    if (_shouldTerminate(nextState)) {
      await _writeTerminalOutcomeReport(
        artifactDirectory: artifactDirectory,
        state: nextState,
      );
    }
    return _formatExecutionSummary(
      artifactDirectory: artifactDirectory,
      state: nextState,
      prefix: 'Harness evaluation routed',
    );
  }

  String _formatExecutionSummary({
    required Directory artifactDirectory,
    required HarnessState state,
    String prefix = 'Harness execution updated',
  }) {
    final artifactLabel = p.relative(artifactDirectory.path, from: root.path);
    final action = state.actionHistory.isEmpty ? 'none' : state.actionHistory.last;
    final reasons = state.lastReasonCodes.isEmpty
        ? 'none'
        : state.lastReasonCodes.join(', ');
    final outcome = switch (state.status) {
      'passed' => 'passed cleanly',
      'blocked_environment' => 'blocked by environment',
      'split_required' => 'requires task split',
      'evolution_exhausted' => 'stopped after exhausted evolution budget',
      'revise_exhausted' => 'stopped after exhausted revision budget',
      'rejected' => 'rejected by evaluator',
      _ => 'updated',
    };
    return '$prefix at $artifactLabel ($outcome, status=${state.status}, currentActor=${state.currentActor ?? 'none'}, action=$action, reasons=$reasons)';
  }

  SchemaValidator _loadSchema(String schemaName) {
    final schemaPath = switch (schemaName) {
      'request' => '.harness/templates/user_request.schema.yaml',
      'plan' => '.harness/templates/plan.schema.yaml',
      'context_pack' => '.harness/templates/context_pack.schema.yaml',
      'implementation_result' =>
        '.harness/templates/implementation_result.schema.yaml',
      'execution_report' => '.harness/templates/execution_report.schema.yaml',
      'evaluation_result' => '.harness/templates/evaluation_result.schema.yaml',
      'integration_result' =>
        '.harness/templates/integration_result.schema.yaml',
      _ => throw ArgumentError('Unsupported schema: $schemaName'),
    };
    return SchemaValidator(_loadYamlMap(schemaPath), schemaName: schemaName);
  }

  Map<String, dynamic> _loadYamlMap(String relativePath) {
    final file = _resolveFileWithinRoot(relativePath);
    final value = _loadYamlValue(file);
    return _asMap(value, context: relativePath);
  }

  dynamic _loadYamlValue(File file) {
    if (!file.existsSync()) {
      throw ArgumentError(
        'Missing YAML file: ${p.relative(file.path, from: root.path)}',
      );
    }
    final parsed = loadYaml(file.readAsStringSync());
    return _toNativeValue(parsed);
  }

  File _resolveFileWithinRoot(String relativeOrAbsolutePath) {
    final candidate = p.isAbsolute(relativeOrAbsolutePath)
        ? p.normalize(relativeOrAbsolutePath)
        : p.normalize(p.join(root.path, relativeOrAbsolutePath));
    _assertPathWithinRoot(candidate, isDirectory: false);
    return File(candidate);
  }

  void _assertPathWithinRoot(String absolutePath, {required bool isDirectory}) {
    final normalized = p.normalize(absolutePath);
    if (_pathExists(normalized)) {
      final canonical = _resolveExistingEntity(normalized);
      _assertCanonicalWithinRoot(canonical);
      return;
    }

    var existingAncestor = isDirectory ? normalized : p.dirname(normalized);
    while (!_pathExists(existingAncestor)) {
      final parent = p.dirname(existingAncestor);
      if (parent == existingAncestor) {
        throw ArgumentError('Path escapes repository root: $absolutePath');
      }
      existingAncestor = parent;
    }
    final canonicalAncestor = _resolveExistingEntity(existingAncestor);
    _assertCanonicalWithinRoot(canonicalAncestor);
  }

  Directory _resolveArtifactDirectory({
    required String artifactRoot,
    required String taskId,
  }) {
    final basePath = p.normalize(p.join(root.path, artifactRoot));
    final candidate = p.normalize(p.join(basePath, taskId));
    _assertPathWithinRoot(basePath, isDirectory: true);
    _assertPathWithinRoot(candidate, isDirectory: true);
    final relative = p.relative(candidate, from: basePath);
    if (relative == '..' ||
        relative.startsWith('..${p.separator}') ||
        p.isAbsolute(relative)) {
      throw ArgumentError('Invalid task id produced an unsafe artifact path.');
    }
    return Directory(candidate);
  }

  Directory _resolveProjectDirectory(String relativeOrAbsolutePath) {
    final candidate = p.isAbsolute(relativeOrAbsolutePath)
        ? p.normalize(relativeOrAbsolutePath)
        : p.normalize(p.join(Directory.current.path, relativeOrAbsolutePath));
    final directory = Directory(candidate);
    if (!directory.existsSync()) {
      throw ArgumentError('Project root not found: $relativeOrAbsolutePath');
    }
    return Directory(directory.resolveSymbolicLinksSync());
  }

  List<String> _inferPackageRoots(List<String> fileHints) {
    final packageRoots = <String>{};
    for (final hint in fileHints) {
      final normalized = p.normalize(hint);
      final segments = p.split(normalized);
      if (segments.length >= 2 &&
          (segments.first == 'apps' || segments.first == 'packages')) {
        packageRoots.add(p.join(segments.first, segments[1]));
        continue;
      }

      if (segments.isNotEmpty &&
          (segments.first == 'lib' || segments.first == 'test')) {
        packageRoots.add('.');
      }
    }
    return packageRoots.toList()..sort();
  }

  Map<String, List<String>> _groupTargetsByPackage(List<String> targets) {
    final grouped = <String, List<String>>{};
    for (final target in targets) {
      final normalized = p.normalize(target);
      final segments = p.split(normalized);
      if (segments.length >= 2 &&
          (segments.first == 'apps' || segments.first == 'packages')) {
        final packageRoot = p.join(segments.first, segments[1]);
        final localTarget = p.relative(normalized, from: packageRoot);
        grouped.putIfAbsent(packageRoot, () => <String>[]).add(localTarget);
      } else {
        grouped.putIfAbsent('.', () => <String>[]).add(normalized);
      }
    }
    return grouped;
  }

  void _validateRouterConsistency({
    required String taskType,
    required TaskConfig taskConfig,
    required TaskRoute route,
  }) {
    if (!_listsEqual(taskConfig.actors, route.actors)) {
      throw StateError(
        'Registry/task_router mismatch for `$taskType`: ${taskConfig.actors} != ${route.actors}',
      );
    }
  }

  void _validateContextContractConsistency({
    required String taskType,
    required TaskConfig taskConfig,
    required ContextContract contextContract,
  }) {
    final availableInputs = <String>{
      'user_request',
      'constraints',
      'project_rules',
      'architecture_rules',
      'forbidden_changes',
      'execution_policy',
      'rubric',
    };

    for (final actor in taskConfig.actors) {
      if (!contextContract.hasActor(actor)) {
        throw StateError(
          'Missing actor contract for `$actor` in task `$taskType`.',
        );
      }

      final contract = contextContract.contractFor(actor);
      final missingInputs = contract.inputs
          .where((input) => !availableInputs.contains(input))
          .toList(growable: false);
      if (missingInputs.isNotEmpty) {
        throw StateError(
          'Unsatisfied inputs for actor `$actor` in task `$taskType`: ${missingInputs.join(', ')}',
        );
      }

      availableInputs.addAll(contract.outputs);
      final canonicalOutput = _canonicalOutputForActor(actor);
      if (canonicalOutput != null) {
        availableInputs.add(canonicalOutput);
      }
    }
  }

  int _resolveRetryBudget({
    required int taskRetryBudget,
    required int routeRetryBudget,
    required int policyRetryBudget,
  }) {
    final routeLimited = taskRetryBudget < routeRetryBudget
        ? taskRetryBudget
        : routeRetryBudget;
    return routeLimited < policyRetryBudget ? routeLimited : policyRetryBudget;
  }

  String _defaultTaskId(UserRequest request) {
    final timestamp = DateTime.now()
        .toIso8601String()
        .replaceAll(':', '')
        .replaceAll('.', '');
    final trimmedSlug = _slugifyGoal(
      request.goal,
      fallbackPrefix: request.taskType,
    );
    return '$timestamp-${request.taskType}-$trimmedSlug';
  }

  String _requestFileName(String goal) {
    final timestamp = DateTime.now()
        .toIso8601String()
        .replaceAll(':', '')
        .replaceAll('.', '');
    final trimmedSlug = _slugifyGoal(goal, fallbackPrefix: 'task');
    return '$timestamp-$trimmedSlug.yaml';
  }

  String _slugifyGoal(String goal, {required String fallbackPrefix}) {
    final slug = goal
        .toLowerCase()
        .replaceAll(RegExp(r'[^a-z0-9]+'), '-')
        .replaceAll(RegExp(r'-+'), '-')
        .replaceAll(RegExp(r'^-|-$'), '');
    if (slug.isNotEmpty) {
      return slug;
    }

    final bytes = utf8.encode(goal);
    var hash = 2166136261;
    for (final byte in bytes) {
      hash ^= byte;
      hash = (hash * 16777619) & 0xffffffff;
    }
    final suffix = hash.toRadixString(16).padLeft(8, '0');
    return '$fallbackPrefix-$suffix';
  }

  String _sanitizeTaskId(String taskId) {
    final sanitized = taskId
        .trim()
        .replaceAll(RegExp(r'[^a-zA-Z0-9._-]+'), '-')
        .replaceAll(RegExp(r'-+'), '-')
        .replaceAll(RegExp(r'^-|-$'), '');
    if (sanitized.isEmpty || sanitized == '.' || sanitized == '..') {
      throw ArgumentError('Invalid task id: $taskId');
    }
    return sanitized;
  }

  String _normalizeRepoRelativePath(String path) {
    final normalized = p.normalize(path);
    final absolute = p.isAbsolute(normalized)
        ? normalized
        : p.join(root.path, normalized);
    _assertPathWithinRoot(absolute, isDirectory: false);
    final relative = p.relative(absolute, from: root.path);
    if (relative == '..' ||
        relative.startsWith('..${p.separator}') ||
        p.isAbsolute(relative)) {
      throw ArgumentError('File hint escapes repository root: $path');
    }
    return relative;
  }

  String _artifactFilePath(String artifactRoot, String outputName) {
    switch (outputName) {
      case 'plan':
        return p.join(artifactRoot, 'plan.yaml');
      case 'context_pack':
        return p.join(artifactRoot, 'context_pack.yaml');
      case 'implementation_result':
        return p.join(artifactRoot, 'implementation_result.yaml');
      case 'execution_report':
        return p.join(artifactRoot, 'execution_report.yaml');
      case 'evaluation_result':
        return p.join(artifactRoot, 'evaluation_result.yaml');
      case 'integration_result':
        return p.join(artifactRoot, 'integration_result.yaml');
    }
    return p.join(artifactRoot, '$outputName.yaml');
  }

  String _placeholderContent(String outputName) {
    return _toYaml(_placeholderObject(outputName));
  }

  String? _schemaNameForActor(String actorName) {
    switch (actorName) {
      case 'planner':
        return 'plan';
      case 'context_builder':
        return 'context_pack';
      case 'generator':
        return 'implementation_result';
      case 'executor':
        return 'execution_report';
      case 'evaluator':
        return 'evaluation_result';
      case 'integrator':
        return 'integration_result';
    }
    return null;
  }

  String? _canonicalOutputForActor(String actorName) {
    switch (actorName) {
      case 'planner':
        return 'plan';
      case 'context_builder':
        return 'context_pack';
      case 'generator':
        return 'implementation_result';
      case 'executor':
        return 'execution_report';
      case 'evaluator':
        return 'evaluation_result';
      case 'integrator':
        return 'integration_result';
    }
    return null;
  }

  String _buildWorkflowSteps({
    required ResolvedWorkflow workflow,
    required ExecutionPlan executionPlan,
  }) {
    final buffer = StringBuffer()
      ..writeln('# Workflow Steps')
      ..writeln()
      ..writeln('- Task ID: `${workflow.taskId}`')
      ..writeln('- Task type: `${workflow.taskType}`')
      ..writeln('- Target project root: `${workflow.projectRoot}`')
      ..writeln('- Actors: `${workflow.actors.join(' -> ')}`')
      ..writeln('- Rubric: `${workflow.rubricPath}`')
      ..writeln('- Generator revise budget: `${workflow.generatorRetryBudget}`')
      ..writeln('- Context rebuild budget: `${workflow.contextRebuildBudget}`')
      ..writeln(
        '- Validation tighten budget: `${workflow.validationTightenBudget}`',
      )
      ..writeln();

    if (workflow.changedFileHints.isNotEmpty) {
      buffer.writeln('## File Hints');
      for (final file in workflow.changedFileHints) {
        buffer.writeln('- `$file`');
      }
      buffer.writeln();
    }

    if (workflow.inferredTestTargets.isNotEmpty) {
      buffer.writeln('## Test Targets');
      for (final target in workflow.inferredTestTargets) {
        buffer.writeln('- `$target`');
      }
      buffer.writeln();
    }

    buffer.writeln('## Policy');
    for (final condition in workflow.passIf) {
      buffer.writeln('- pass_if: `$condition`');
    }
    for (final condition in workflow.reviseIf) {
      buffer.writeln('- revise_if: `$condition`');
    }
    for (final condition in workflow.rejectIf) {
      buffer.writeln('- reject_if: `$condition`');
    }
    for (final condition in workflow.terminationConditions) {
      buffer.writeln('- terminate_if: `$condition`');
    }
    buffer.writeln();

    buffer.writeln('## Supervisor Actions');
    buffer.writeln(
      '- `revise_generator`: request another implementation attempt with the current context.',
    );
    buffer.writeln(
      '- `rebuild_context`: refresh context artifacts before another implementation attempt.',
    );
    buffer.writeln(
      '- `tighten_validation`: reduce executor scope to the smallest credible validation set.',
    );
    buffer.writeln(
      '- `split_task`: stop orchestration and require a smaller follow-up task.',
    );
    buffer.writeln(
      '- `block_environment`: stop orchestration because tooling or environment setup prevents credible validation.',
    );
    buffer.writeln();

    buffer.writeln('## Executor Commands');
    if (executionPlan.formatCommand != null) {
      buffer.writeln('- `${executionPlan.formatCommand}`');
    }
    for (final command in executionPlan.analyzeCommands) {
      buffer.writeln('- `$command`');
    }
    for (final command in executionPlan.testCommands) {
      buffer.writeln('- `$command`');
    }
    return buffer.toString();
  }

  String _buildActorBrief({
    required String actorName,
    required String actorInstructions,
    required ResolvedWorkflow workflow,
    required ExecutionPlan executionPlan,
    required ActorContract actorContract,
    required Directory artifactDirectory,
    required MaterializedInputs materializedInputs,
  }) {
    final outputPath = switch (actorName) {
      'planner' => 'plan.yaml',
      'context_builder' => 'context_pack.yaml',
      'generator' => 'implementation_result.yaml',
      'executor' => 'execution_report.yaml',
      'evaluator' => 'evaluation_result.yaml',
      'integrator' => 'integration_result.yaml',
      _ => '$actorName.yaml',
    };

    return '''
# ${actorName.toUpperCase()} Brief

- Task ID: `${workflow.taskId}`
- Task type: `${workflow.taskType}`
- Target project root: `${workflow.projectRoot}`
- Output file: `${p.join(artifactDirectory.path, outputPath)}`
- Rubric: `${workflow.rubricPath}`

## Contract Inputs
${actorContract.inputs.map((value) => '- `$value`: `${_inputPathForToken(value, artifactDirectory: artifactDirectory, materializedInputs: materializedInputs)}`').join('\n')}

## Contract Outputs
${actorContract.outputs.map((value) => '- `$value`').join('\n')}

## Actor Instructions
$actorInstructions

## Executor Preview
${const JsonEncoder.withIndent('  ').convert(executionPlan.toJson())}
''';
  }

  Future<MaterializedInputs> _materializeStaticInputs({
    required Directory artifactDirectory,
    required Directory inputsDirectory,
    required TaskConfig taskConfig,
  }) async {
    final architectureRulesSource = _resolveFileWithinRoot(
      '.harness/rules/architecture_rules.md',
    );
    final projectRulesSource = _resolveFileWithinRoot(
      '.harness/rules/project_rules.md',
    );
    final forbiddenChangesSource = _resolveFileWithinRoot(
      '.harness/rules/forbidden_changes.md',
    );
    final executionPolicySource = _resolveFileWithinRoot(
      '.harness/supervisor/execution_policy.yaml',
    );
    final rubricSource = _resolveFileWithinRoot(taskConfig.rubric);

    final architectureRulesTarget = File(
      p.join(inputsDirectory.path, 'architecture_rules.md'),
    );
    final projectRulesTarget = File(
      p.join(inputsDirectory.path, 'project_rules.md'),
    );
    final forbiddenChangesTarget = File(
      p.join(inputsDirectory.path, 'forbidden_changes.md'),
    );
    final executionPolicyTarget = File(
      p.join(inputsDirectory.path, 'execution_policy.yaml'),
    );
    final rubricTarget = File(p.join(inputsDirectory.path, 'rubric.yaml'));

    await architectureRulesTarget.writeAsString(
      await architectureRulesSource.readAsString(),
    );
    await projectRulesTarget.writeAsString(
      await projectRulesSource.readAsString(),
    );
    await forbiddenChangesTarget.writeAsString(
      await forbiddenChangesSource.readAsString(),
    );
    await executionPolicyTarget.writeAsString(
      await executionPolicySource.readAsString(),
    );
    await rubricTarget.writeAsString(await rubricSource.readAsString());

    return MaterializedInputs(
      architectureRulesPath: p.relative(
        architectureRulesTarget.path,
        from: artifactDirectory.path,
      ),
      projectRulesPath: p.relative(
        projectRulesTarget.path,
        from: artifactDirectory.path,
      ),
      forbiddenChangesPath: p.relative(
        forbiddenChangesTarget.path,
        from: artifactDirectory.path,
      ),
      executionPolicyPath: p.relative(
        executionPolicyTarget.path,
        from: artifactDirectory.path,
      ),
      rubricPath: p.relative(rubricTarget.path, from: artifactDirectory.path),
      requestPath: 'request.yaml',
    );
  }

  String _inputPathForToken(
    String token, {
    required Directory artifactDirectory,
    required MaterializedInputs materializedInputs,
  }) {
    switch (token) {
      case 'user_request':
      case 'constraints':
        return p.join(artifactDirectory.path, materializedInputs.requestPath);
      case 'architecture_rules':
        return p.join(
          artifactDirectory.path,
          materializedInputs.architectureRulesPath,
        );
      case 'project_rules':
        return p.join(artifactDirectory.path, materializedInputs.projectRulesPath);
      case 'forbidden_changes':
        return p.join(
          artifactDirectory.path,
          materializedInputs.forbiddenChangesPath,
        );
      case 'execution_policy':
        return p.join(
          artifactDirectory.path,
          materializedInputs.executionPolicyPath,
        );
      case 'rubric':
        return p.join(artifactDirectory.path, materializedInputs.rubricPath);
      case 'plan':
        return p.join(artifactDirectory.path, 'plan.yaml');
      case 'context_pack':
        return p.join(artifactDirectory.path, 'context_pack.yaml');
      case 'implementation_result':
        return p.join(artifactDirectory.path, 'implementation_result.yaml');
      case 'execution_report':
        return p.join(artifactDirectory.path, 'execution_report.yaml');
      case 'evaluation_result':
        return p.join(artifactDirectory.path, 'evaluation_result.yaml');
      case 'integration_result':
        return p.join(artifactDirectory.path, 'integration_result.yaml');
    }
    return token;
  }

  Map<String, Object?> _placeholderObject(String outputName) {
    switch (outputName) {
      case 'plan':
        return {
          'summary': '',
          'likely_files': <String>[],
          'assumptions': <String>[],
          'substeps': <String>[],
          'risks': <String>[],
          'acceptance_criteria_refined': <String>[],
        };
      case 'context_pack':
        return {
          'relevant_files': <Map<String, String>>[],
          'repo_patterns': <String>[],
          'test_patterns': <String>[],
          'forbidden_changes': <String>[],
          'implementation_hints': <String>[],
        };
      case 'implementation_result':
        return {
          'changed_files': <String>[],
          'patch_summary': <String>[],
          'tests_added_or_updated': <String>[],
          'known_limits': <String>[],
        };
      case 'execution_report':
        return {
          'format': 'fail',
          'analyze': 'fail',
          'tests': {
            'total': 0,
            'passed': 0,
            'failed': 0,
          },
          'failure_details': <String>['bootstrap placeholder'],
          'logs': <String>[],
        };
      case 'evaluation_result':
        return {
          'decision': 'revise',
          'scores': {
            'requirements': 0,
            'architecture': 0,
            'regression_risk': 0,
          },
          'findings': <String>['bootstrap placeholder'],
          'reason_codes': <String>['bootstrap_placeholder'],
          'next_action': 'revise_generator',
        };
      case 'integration_result':
        return {
          'summary': '',
          'files_changed': <String>[],
          'validation': <String>[],
          'risks': <String>[],
          'follow_up': <String>[],
        };
    }
    return <String, Object?>{};
  }

  String _resolveExistingEntity(String path) {
    final type = FileSystemEntity.typeSync(path);
    if (type == FileSystemEntityType.directory) {
      return Directory(path).resolveSymbolicLinksSync();
    }
    if (type == FileSystemEntityType.file) {
      return File(path).resolveSymbolicLinksSync();
    }
    if (type == FileSystemEntityType.link) {
      return Link(path).resolveSymbolicLinksSync();
    }
    throw ArgumentError('Missing or unsupported filesystem entity: $path');
  }

  void _assertCanonicalWithinRoot(String canonicalPath) {
    final relative = p.relative(canonicalPath, from: _rootCanonicalPath);
    if (relative == '..' ||
        relative.startsWith('..${p.separator}') ||
        p.isAbsolute(relative)) {
      throw ArgumentError('Path escapes repository root: $canonicalPath');
    }
  }

  Map<String, dynamic> _readJsonFile(String filePath) {
    final file = File(filePath);
    if (!file.existsSync()) {
      throw ArgumentError(
        'Missing JSON file: ${p.relative(file.path, from: root.path)}',
      );
    }
    final decoded = jsonDecode(file.readAsStringSync());
    return _asMap(decoded, context: filePath);
  }

  Future<String> _materializeOutputSchema({
    required String schemaName,
    required Directory runsDirectory,
    required int actorIndex,
    required String actorName,
  }) async {
    final schema = _strictJsonSchema(_loadSchema(schemaName).schema);
    final schemaFile = File(
      p.join(
        runsDirectory.path,
        '${(actorIndex + 1).toString().padLeft(2, '0')}_$actorName-output-schema.json',
      ),
    );
    await schemaFile.writeAsString(
      const JsonEncoder.withIndent('  ').convert(schema),
    );
    return schemaFile.path;
  }

  Map<String, dynamic> _decodeStructuredResponse({
    required String filePath,
    required String fallbackText,
  }) {
    final file = File(filePath);
    final text = file.existsSync() ? file.readAsStringSync() : fallbackText;
    final trimmed = text.trim();
    if (trimmed.isEmpty) {
      throw StateError('Structured actor response was empty: $filePath');
    }
    final decoded = jsonDecode(trimmed);
    return _asMap(decoded, context: filePath);
  }

  Map<String, dynamic> _strictJsonSchema(Map<String, dynamic> schema) {
    final strict = <String, dynamic>{};
    for (final entry in schema.entries) {
      final value = entry.value;
      if (value is Map<String, dynamic>) {
        strict[entry.key] = _strictJsonSchema(value);
      } else if (value is List) {
        strict[entry.key] = value.map((item) {
          if (item is Map<String, dynamic>) {
            return _strictJsonSchema(item);
          }
          if (item is Map) {
            return _strictJsonSchema(Map<String, dynamic>.from(item));
          }
          return item;
        }).toList(growable: false);
      } else if (value is Map) {
        strict[entry.key] = _strictJsonSchema(Map<String, dynamic>.from(value));
      } else {
        strict[entry.key] = value;
      }
    }

    if (strict['type'] == 'object') {
      strict['additionalProperties'] = false;
      final properties = strict['properties'];
      if (properties is Map<String, dynamic>) {
        strict['properties'] = {
          for (final entry in properties.entries)
            entry.key: entry.value is Map<String, dynamic>
                ? _strictJsonSchema(entry.value)
                : entry.value,
        };
        strict['required'] = properties.keys.toList(growable: false);
      } else if (properties is Map) {
        final normalizedProperties = {
          for (final entry in properties.entries)
            entry.key.toString(): entry.value is Map<String, dynamic>
                ? _strictJsonSchema(entry.value)
                : entry.value,
        };
        strict['properties'] = normalizedProperties;
        strict['required'] = normalizedProperties.keys.toList(growable: false);
      }
    }

    return strict;
  }

  Future<CommandResult> _runCommand(
    String executable,
    List<String> arguments, {
    required String workingDirectory,
    required Duration timeout,
  }) async {
    final process = await Process.start(
      executable,
      arguments,
      workingDirectory: workingDirectory,
    );
    await process.stdin.close();
    final stdoutFuture = process.stdout.transform(utf8.decoder).join();
    final stderrFuture = process.stderr.transform(utf8.decoder).join();
    final exitCode = await process.exitCode.timeout(
      timeout,
      onTimeout: () {
        process.kill();
        return -1;
      },
    );
    return CommandResult(
      exitCode: exitCode,
      stdout: await stdoutFuture,
      stderr: await stderrFuture,
    );
  }

  HarnessState _advanceState({
    required HarnessState state,
    required ResolvedWorkflow workflow,
    required String actorName,
    required Directory artifactDirectory,
  }) {
    final completedActors = [...state.completedActors, actorName];
    if (actorName == 'evaluator') {
      final evaluationPath = p.join(artifactDirectory.path, 'evaluation_result.yaml');
      final evaluationMap = _asMap(
        _loadYamlValue(File(evaluationPath)),
        context: evaluationPath,
      );
      final decision = _readString(evaluationMap, 'decision');
      final reasonCodes = _readOptionalStringList(evaluationMap, 'reason_codes');
      final nextAction = _readOptionalString(evaluationMap, 'next_action');
      if (decision == 'pass') {
        final integratorIndex = workflow.actors.indexOf('integrator');
        if (integratorIndex != -1 && !completedActors.contains('integrator')) {
          return state.copyWith(
            status: 'awaiting_integrator',
            currentActor: 'integrator',
            completedActors: completedActors,
            lastDecision: decision,
            lastReasonCodes: reasonCodes,
            actionHistory: [...state.actionHistory, 'pass'],
          );
        }
          return state.copyWith(
            status: 'passed',
            clearCurrentActor: true,
            completedActors: completedActors,
            lastDecision: decision,
            lastReasonCodes: reasonCodes,
            actionHistory: [...state.actionHistory, 'pass'],
          );
      }
      if (decision == 'reject') {
        return state.copyWith(
          status: 'rejected',
          clearCurrentActor: true,
          completedActors: completedActors,
          lastDecision: decision,
          lastReasonCodes: reasonCodes,
          actionHistory: [...state.actionHistory, 'reject'],
        );
      }
      final primaryAction = _resolveSupervisorAction(
        nextAction: nextAction,
        reasonCodes: reasonCodes,
      );
      if (primaryAction == null) {
        throw StateError(
          'evaluation_result.yaml revise decision requires either a supported `next_action` or routeable `reason_codes`.',
        );
      }
      switch (primaryAction) {
        case 'rebuild_context':
          final remaining = state.contextRebuildsRemaining - 1;
          if (remaining < 0 || !workflow.actors.contains('context_builder')) {
            return state.copyWith(
              status: 'evolution_exhausted',
              clearCurrentActor: true,
              completedActors: completedActors,
              contextRebuildsRemaining: remaining,
              lastDecision: decision,
              lastReasonCodes: reasonCodes,
              actionHistory: [...state.actionHistory, primaryAction],
            );
          }
          return state.copyWith(
            status: 'rebuilding_context',
            currentActor: 'context_builder',
            completedActors: completedActors,
            contextRebuildsRemaining: remaining,
            lastDecision: decision,
            lastReasonCodes: reasonCodes,
            actionHistory: [...state.actionHistory, primaryAction],
          );
        case 'tighten_validation':
          final remaining = state.validationTighteningsRemaining - 1;
          if (remaining < 0 || !workflow.actors.contains('executor')) {
            return state.copyWith(
              status: 'evolution_exhausted',
              clearCurrentActor: true,
              completedActors: completedActors,
              validationTighteningsRemaining: remaining,
              lastDecision: decision,
              lastReasonCodes: reasonCodes,
              actionHistory: [...state.actionHistory, primaryAction],
            );
          }
          return state.copyWith(
            status: 'tightening_validation',
            currentActor: 'executor',
            completedActors: completedActors,
            validationTighteningsRemaining: remaining,
            lastDecision: decision,
            lastReasonCodes: reasonCodes,
            actionHistory: [...state.actionHistory, primaryAction],
          );
        case 'split_task':
          return state.copyWith(
            status: 'split_required',
            clearCurrentActor: true,
            completedActors: completedActors,
            lastDecision: decision,
            lastReasonCodes: reasonCodes,
            actionHistory: [...state.actionHistory, primaryAction],
          );
        case 'block_environment':
          return state.copyWith(
            status: 'blocked_environment',
            clearCurrentActor: true,
            completedActors: completedActors,
            lastDecision: decision,
            lastReasonCodes: reasonCodes,
            actionHistory: [...state.actionHistory, primaryAction],
          );
        case 'revise_generator':
        default:
          final retriesRemaining = state.generatorRetriesRemaining - 1;
          if (retriesRemaining < 0 || !workflow.actors.contains('generator')) {
            return state.copyWith(
              status: 'revise_exhausted',
              clearCurrentActor: true,
              completedActors: completedActors,
              generatorRetriesRemaining: retriesRemaining,
              lastDecision: decision,
              lastReasonCodes: reasonCodes,
              actionHistory: [...state.actionHistory, 'revise_generator'],
            );
          }
          return state.copyWith(
            status: 'revising',
            currentActor: 'generator',
            completedActors: completedActors,
            generatorRetriesRemaining: retriesRemaining,
            lastDecision: decision,
            lastReasonCodes: reasonCodes,
            actionHistory: [...state.actionHistory, 'revise_generator'],
          );
      }
    }

    final actorIndex = workflow.actors.indexOf(actorName);
    final nextActor = actorIndex == -1 || actorIndex + 1 >= workflow.actors.length
        ? null
        : workflow.actors[actorIndex + 1];
    return state.copyWith(
      status: nextActor == null ? 'completed' : 'in_progress',
      currentActor: nextActor,
      completedActors: completedActors,
    );
  }

  bool _shouldTerminate(HarnessState state) {
    return state.currentActor == null ||
        state.status == 'passed' ||
        state.status == 'rejected' ||
        state.status == 'revise_exhausted' ||
        state.status == 'evolution_exhausted' ||
        state.status == 'blocked_environment' ||
        state.status == 'split_required';
  }

  String? _resolveSupervisorAction({
    required String? nextAction,
    required List<String> reasonCodes,
  }) {
    return _preferredSupervisorAction(reasonCodes) ?? nextAction;
  }

  String? _preferredSupervisorAction(List<String> reasonCodes) {
    if (_hasEnvironmentFailure(reasonCodes)) {
      return 'block_environment';
    }
    if (_hasScopeFailure(reasonCodes)) {
      return 'split_task';
    }
    if (_hasContextFailure(reasonCodes)) {
      return 'rebuild_context';
    }
    if (_hasValidationScopeFailure(reasonCodes)) {
      return 'tighten_validation';
    }
    if (_hasValidationEvidenceFailure(reasonCodes) ||
        _hasValidationRequirementFailure(reasonCodes) ||
        _hasRequirementsCoverageFailure(reasonCodes) ||
        _hasRequirementsBehaviorFailure(reasonCodes) ||
        _hasValidationFailure(reasonCodes) ||
        _hasRequirementsFailure(reasonCodes)) {
      return 'revise_generator';
    }
    if (_hasImplementationFailure(reasonCodes)) {
      return 'revise_generator';
    }
    if (_hasArchitectureFailure(reasonCodes)) {
      return 'revise_generator';
    }
    return null;
  }

  bool _hasEnvironmentFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) {
      return code.startsWith('environment_') ||
          code.contains('permission_error') ||
          code.contains('sandbox') ||
          code.contains('tooling_unavailable') ||
          code.contains('sdk_cache');
    });
  }

  bool _hasScopeFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('scope_'));
  }

  bool _hasContextFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('context_'));
  }

  bool _hasValidationScopeFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) {
      return code.startsWith('validation_scope_') ||
          code.startsWith('validation_target_') ||
          code.startsWith('validation_mismatch_');
    });
  }

  bool _hasValidationFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('validation_'));
  }

  bool _hasValidationEvidenceFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('validation_evidence_'));
  }

  bool _hasValidationRequirementFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('validation_requirement_'));
  }

  bool _hasRequirementsFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('requirements_'));
  }

  bool _hasRequirementsCoverageFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('requirements_coverage_'));
  }

  bool _hasRequirementsBehaviorFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('requirements_behavior_'));
  }

  bool _hasImplementationFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('implementation_'));
  }

  bool _hasArchitectureFailure(List<String> reasonCodes) {
    return reasonCodes.any((code) => code.startsWith('architecture_'));
  }

  Future<void> _appendSupervisorDecisionTrace({
    required Directory artifactDirectory,
    required HarnessState state,
  }) async {
    final traceFile = File(p.join(artifactDirectory.path, 'supervisor_trace.md'));
    final evaluationFile = File(
      p.join(artifactDirectory.path, 'evaluation_result.yaml'),
    );
    final evaluationMap = evaluationFile.existsSync()
        ? _asMap(_loadYamlValue(evaluationFile), context: evaluationFile.path)
        : const <String, dynamic>{};
    final decision = evaluationMap['decision']?.toString() ?? state.lastDecision ?? '';
    final action = state.actionHistory.isEmpty ? '' : state.actionHistory.last;
    final iteration = state.completedActors.where((actor) => actor == 'evaluator').length;
    final category = _primaryReasonCategory(state.lastReasonCodes);
    final buffer = StringBuffer();
    if (!traceFile.existsSync()) {
      buffer
        ..writeln('# Supervisor Decision Trace')
        ..writeln()
        ..writeln('Reason code taxonomy:')
        ..writeln('- `environment_*`: tooling, sandbox, SDK cache, permissions, or external setup failures')
        ..writeln('- `validation_scope_*` / `validation_target_*` / `validation_mismatch_*`: validation scope or target selection problems')
        ..writeln('- `validation_evidence_*`: validation evidence is missing, incomplete, or weak')
        ..writeln('- `validation_requirement_*`: validation exposed a concrete unmet requirement')
        ..writeln('- `requirements_coverage_*` / `requirements_behavior_*`: required coverage or behavior is still missing')
        ..writeln('- `context_*`: insufficient repository context or missing grounding')
        ..writeln('- `implementation_*`: code or patch quality gaps')
        ..writeln('- `scope_*`: blast radius or task-boundary findings')
        ..writeln('- `architecture_*`: design or layering violations')
        ..writeln()
        ..writeln('Routing rule:')
        ..writeln('- runtime treats `reason_codes` as authoritative; `next_action` is used only when the reason-code taxonomy does not resolve a supervisor action')
        ..writeln();
    }
    buffer
      ..writeln('## Iteration $iteration')
      ..writeln()
      ..writeln('- decision: `${decision.isEmpty ? 'unknown' : decision}`')
      ..writeln('- selected_action: `${action.isEmpty ? 'unknown' : action}`')
      ..writeln('- status_after_routing: `${state.status}`')
      ..writeln('- primary_reason_category: `${category}`')
      ..writeln('- reason_codes: `${state.lastReasonCodes.join(', ')}`')
      ..writeln(
        '- budgets_remaining: `generator=${state.generatorRetriesRemaining}, context=${state.contextRebuildsRemaining}, validation=${state.validationTighteningsRemaining}`',
      )
      ..writeln();
    await traceFile.writeAsString(
      buffer.toString(),
      mode: FileMode.append,
      flush: true,
    );
  }

  Future<void> _writeTerminalOutcomeReport({
    required Directory artifactDirectory,
    required HarnessState state,
  }) async {
    final summaryFile = File(p.join(artifactDirectory.path, 'terminal_summary.md'));
    final evaluationFile = File(
      p.join(artifactDirectory.path, 'evaluation_result.yaml'),
    );
    final executionFile = File(
      p.join(artifactDirectory.path, 'execution_report.yaml'),
    );
    final evaluationMap = evaluationFile.existsSync()
        ? _asMap(_loadYamlValue(evaluationFile), context: evaluationFile.path)
        : const <String, dynamic>{};
    final executionMap = executionFile.existsSync()
        ? _asMap(_loadYamlValue(executionFile), context: executionFile.path)
        : const <String, dynamic>{};
    final findings = _readOptionalStringList(evaluationMap, 'findings');
    final failureDetails = _readOptionalStringList(executionMap, 'failure_details');
    final logs = _readOptionalStringList(executionMap, 'logs');
    final action = state.actionHistory.isEmpty ? 'none' : state.actionHistory.last;
    final decision = state.lastDecision ?? _readOptionalString(evaluationMap, 'decision');
    final buffer = StringBuffer()
      ..writeln('# Terminal Outcome')
      ..writeln()
      ..writeln('- status: `${state.status}`')
      ..writeln('- action: `$action`')
      ..writeln('- decision: `${decision ?? 'unknown'}`')
      ..writeln('- reason_category: `${_primaryReasonCategory(state.lastReasonCodes)}`')
      ..writeln('- reason_codes: `${state.lastReasonCodes.isEmpty ? 'none' : state.lastReasonCodes.join(', ')}`')
      ..writeln();

    if (findings.isNotEmpty) {
      buffer.writeln('## Evaluator Findings');
      for (final finding in findings) {
        buffer.writeln('- $finding');
      }
      buffer.writeln();
    }

    if (failureDetails.isNotEmpty) {
      buffer.writeln('## Failure Details');
      for (final detail in failureDetails) {
        buffer.writeln('- $detail');
      }
      buffer.writeln();
    }

    if (logs.isNotEmpty) {
      buffer.writeln('## Command Logs');
      for (final log in logs.take(10)) {
        buffer.writeln('- $log');
      }
      if (logs.length > 10) {
        buffer.writeln('- ... (${logs.length - 10} more)');
      }
      buffer.writeln();
    }

    await summaryFile.writeAsString(buffer.toString(), flush: true);
  }

  Map<String, dynamic> _normalizeExecutionReport({
    required Map<String, dynamic> report,
    required Directory artifactDirectory,
    required String actorName,
  }) {
    if (actorName != 'executor') {
      return report;
    }
    final normalized = Map<String, dynamic>.from(report);
    final failureDetails = _readOptionalStringList(normalized, 'failure_details').toList();
    final logs = _readOptionalStringList(normalized, 'logs').toList();
    final format = normalized['format']?.toString() ?? 'fail';
    final analyze = normalized['analyze']?.toString() ?? 'fail';
    final tests = normalized['tests'] is Map<String, dynamic>
        ? Map<String, dynamic>.from(normalized['tests'] as Map<String, dynamic>)
        : normalized['tests'] is Map
            ? Map<String, dynamic>.from(normalized['tests'] as Map)
            : <String, dynamic>{};
    final totalTests = _readInt(tests, 'total');
    final failedTests = _readInt(tests, 'failed');
    final artifactLabel = p.relative(artifactDirectory.path, from: root.path);

    if (logs.isEmpty) {
      logs.add(
        'executor_report_missing_logs :: artifact=$artifactLabel :: runtime inserted fallback summary',
      );
    }
    if (failureDetails.isEmpty && (format != 'pass' || analyze != 'pass' || failedTests > 0)) {
      failureDetails.add(
        'Executor reported a failed validation status without concrete failure details; runtime preserved the failure and inserted this fallback note.',
      );
    }
    if (failureDetails.isEmpty && totalTests == 0 && format == 'pass' && analyze == 'pass') {
      failureDetails.add(
        'No test commands were executed. This can be acceptable for tightly scoped validation, but should be reviewed against the task rubric.',
      );
    }

    normalized['tests'] = tests;
    normalized['failure_details'] = failureDetails;
    normalized['logs'] = logs;
    return normalized;
  }

  String _primaryReasonCategory(List<String> reasonCodes) {
    for (final code in reasonCodes) {
      if (code.startsWith('environment_') ||
          code.contains('permission_error') ||
          code.contains('sandbox') ||
          code.contains('tooling_unavailable') ||
          code.contains('sdk_cache')) {
        return 'environment';
      }
      if (code.startsWith('validation_') ||
          code.startsWith('requirements_')) {
        return 'validation';
      }
      if (code.startsWith('context_')) {
        return 'context';
      }
      if (code.startsWith('implementation_')) {
        return 'implementation';
      }
      if (code.startsWith('scope_')) {
        return 'scope';
      }
      if (code.startsWith('architecture_')) {
        return 'architecture';
      }
    }
    return reasonCodes.isEmpty ? 'none' : 'mixed';
  }

  ExecutionPlan _buildExecutionPlan({
    required UserRequest userRequest,
    required String projectRoot,
    required ExecutionPolicy executionPolicy,
    required TestTargetRules testRules,
    required List<String> fileHints,
    List<String>? analyzePackagesOverride,
    List<String>? testTargetsOverride,
  }) {
    final analyzePackages = analyzePackagesOverride ??
        (userRequest.context.validationRoots.isNotEmpty
            ? userRequest.context.validationRoots
            : _inferPackageRoots(fileHints));
    final testTargets = testTargetsOverride ??
        (userRequest.context.validationTargets.isNotEmpty
            ? userRequest.context.validationTargets
            : testRules.inferTargets(
            projectRoot: projectRoot,
            fileHints: fileHints,
            featureName: userRequest.context.feature,
          ));

    return ExecutionPlan(
      formatCommand: fileHints.isEmpty
          ? null
          : executionPolicy.formatCommand.replaceAll(
              '{files}',
              fileHints.map(_shellQuote).join(' '),
            ),
      analyzeCommands: userRequest.validationProfile == 'smoke'
          ? [
              'cd ${_shellQuote(projectRoot)} && ${executionPolicy.smokeAnalyzeCommand}',
            ]
          : analyzePackages.isEmpty
          ? [
              'cd ${_shellQuote(projectRoot)} && ${executionPolicy.workspaceAnalyzeCommand}',
            ]
          : analyzePackages
              .map(
                (packageRoot) =>
                    'cd ${_shellQuote(p.join(projectRoot, packageRoot))} && ${executionPolicy.packageAnalyzeCommand}',
              )
              .toList(growable: false),
      testCommands: userRequest.validationProfile == 'smoke'
          ? [
              'cd ${_shellQuote(projectRoot)} && ${executionPolicy.smokeTestCommand}',
            ]
          : testTargets.isEmpty
          ? [
              'cd ${_shellQuote(projectRoot)} && ${executionPolicy.workspaceTestCommand}',
            ]
          : _groupTargetsByPackage(testTargets).entries
              .map(
                (entry) =>
                    'cd ${_shellQuote(p.join(projectRoot, entry.key))} && ${executionPolicy.packageTestCommand.replaceAll('{targets}', entry.value.map(_shellQuote).join(' '))}',
              )
              .toList(growable: false),
    );
  }

  Future<ExecutionPlan> _refreshStandardExecutionPlanIfNeeded({
    required String actorName,
    required Directory artifactDirectory,
    required ResolvedWorkflow workflow,
    required UserRequest userRequest,
    required ExecutionPlan executionPlan,
    required ExecutionPolicy executionPolicy,
    required TestTargetRules testRules,
    required ContextContract contextContract,
    required int actorIndex,
    required String projectRoot,
  }) async {
    if (actorName != 'executor' ||
        userRequest.validationProfile != 'standard' ||
        userRequest.context.validationRoots.isNotEmpty ||
        userRequest.context.validationTargets.isNotEmpty) {
      return executionPlan;
    }

    final planFileHints = _projectRelativePlanLikelyFiles(
      artifactDirectory: artifactDirectory,
      projectRoot: projectRoot,
    );
    if (planFileHints.isEmpty) {
      return executionPlan;
    }

    final refreshedExecutionPlan = _buildExecutionPlan(
      userRequest: userRequest,
      projectRoot: projectRoot,
      executionPolicy: executionPolicy,
      testRules: testRules,
      fileHints: planFileHints,
    );
    if (_sameExecutionPlan(executionPlan, refreshedExecutionPlan)) {
      return executionPlan;
    }

    await _persistExecutionPlanRefresh(
      actorName: actorName,
      artifactDirectory: artifactDirectory,
      workflow: workflow,
      executionPlan: refreshedExecutionPlan,
      contextContract: contextContract,
      actorIndex: actorIndex,
    );
    return refreshedExecutionPlan;
  }

  Future<ExecutionPlan> _tightenExecutionPlanIfNeeded({
    required String actorName,
    required Directory artifactDirectory,
    required ResolvedWorkflow workflow,
    required UserRequest userRequest,
    required ExecutionPlan executionPlan,
    required ExecutionPolicy executionPolicy,
    required TestTargetRules testRules,
    required ContextContract contextContract,
    required int actorIndex,
    required String projectRoot,
    required HarnessState state,
  }) async {
    if (actorName != 'executor' || state.status != 'tightening_validation') {
      return executionPlan;
    }

    if (userRequest.validationProfile == 'smoke') {
      final tightenedExecutionPlan = _tightenSmokeExecutionPlan(
        executionPlan: executionPlan,
        projectRoot: projectRoot,
      );
      if (_sameExecutionPlan(executionPlan, tightenedExecutionPlan)) {
        return executionPlan;
      }
      await _persistExecutionPlanRefresh(
        actorName: actorName,
        artifactDirectory: artifactDirectory,
        workflow: workflow,
        executionPlan: tightenedExecutionPlan,
        contextContract: contextContract,
        actorIndex: actorIndex,
      );
      return tightenedExecutionPlan;
    }

    if (userRequest.validationProfile != 'standard') {
      return executionPlan;
    }

    final explicitValidationTargets = _tightenValidationTargets(
      userRequest.context.validationTargets,
    );
    final explicitValidationRoots = _tightenValidationRoots(
      roots: userRequest.context.validationRoots,
      tightenedTargets: explicitValidationTargets,
    );
    final candidatePlans = <ExecutionPlan>[];
    if (explicitValidationRoots.isNotEmpty || explicitValidationTargets.isNotEmpty) {
      candidatePlans.add(
        _buildExecutionPlan(
          userRequest: userRequest,
          projectRoot: projectRoot,
          executionPolicy: executionPolicy,
          testRules: testRules,
          fileHints: const <String>[],
          analyzePackagesOverride: explicitValidationRoots,
          testTargetsOverride: explicitValidationTargets,
        ),
      );
    }

    final planFileHints = _projectRelativePlanLikelyFiles(
      artifactDirectory: artifactDirectory,
      projectRoot: projectRoot,
    );
    if (planFileHints.isNotEmpty) {
      candidatePlans.add(
        _buildExecutionPlan(
          userRequest: userRequest,
          projectRoot: projectRoot,
          executionPolicy: executionPolicy,
          testRules: testRules,
          fileHints: planFileHints,
        ),
      );
    }
    if (candidatePlans.isEmpty) {
      return executionPlan;
    }

    var tightenedExecutionPlan = candidatePlans.first;
    for (final candidate in candidatePlans.skip(1)) {
      tightenedExecutionPlan = _preferNarrowerExecutionPlan(
        tightenedExecutionPlan,
        candidate,
      );
    }
    if (_sameExecutionPlan(executionPlan, tightenedExecutionPlan)) {
      return executionPlan;
    }

    await _persistExecutionPlanRefresh(
      actorName: actorName,
      artifactDirectory: artifactDirectory,
      workflow: workflow,
      executionPlan: tightenedExecutionPlan,
      contextContract: contextContract,
      actorIndex: actorIndex,
    );
    return tightenedExecutionPlan;
  }

  ExecutionPlan _tightenSmokeExecutionPlan({
    required ExecutionPlan executionPlan,
    required String projectRoot,
  }) {
    final hasSmokeAnalyzeRoot =
        File(p.join(projectRoot, 'pubspec.yaml')).existsSync() ||
        File(p.join(projectRoot, 'melos.yaml')).existsSync();
    final hasSmokeTestRoot =
        Directory(p.join(projectRoot, 'test')).existsSync() ||
        Directory(p.join(projectRoot, 'integration_test')).existsSync() ||
        File(p.join(projectRoot, 'melos.yaml')).existsSync();
    return ExecutionPlan(
      formatCommand: executionPlan.formatCommand,
      analyzeCommands: hasSmokeAnalyzeRoot
          ? executionPlan.analyzeCommands
          : const [],
      testCommands: hasSmokeTestRoot ? executionPlan.testCommands : const [],
    );
  }

  ExecutionPlan _preferNarrowerExecutionPlan(
    ExecutionPlan current,
    ExecutionPlan candidate,
  ) {
    final currentScore = _executionPlanNarrownessScore(current);
    final candidateScore = _executionPlanNarrownessScore(candidate);
    if (candidateScore < currentScore) {
      return candidate;
    }
    return current;
  }

  int _executionPlanNarrownessScore(ExecutionPlan plan) {
    return plan.analyzeCommands.length * 1000 +
        plan.testCommands.length * 100 +
        (plan.formatCommand == null ? 0 : 10) +
        plan.analyzeCommands.join('').length +
        plan.testCommands.join('').length;
  }

  List<String> _tightenValidationTargets(List<String> targets) {
    final normalizedTargets = {
      for (final target in targets) p.normalize(target),
    }.toList()
      ..sort((left, right) {
        final depthCompare = p.split(right).length.compareTo(p.split(left).length);
        if (depthCompare != 0) {
          return depthCompare;
        }
        return right.length.compareTo(left.length);
      });
    if (normalizedTargets.length <= 1) {
      return normalizedTargets;
    }
    return <String>[normalizedTargets.first];
  }

  List<String> _tightenValidationRoots({
    required List<String> roots,
    required List<String> tightenedTargets,
  }) {
    if (tightenedTargets.isNotEmpty) {
      final inferredRoots = _groupTargetsByPackage(tightenedTargets).keys.toList()
        ..sort();
      if (inferredRoots.isNotEmpty) {
        return inferredRoots;
      }
    }

    final normalizedRoots = {
      for (final root in roots) p.normalize(root),
    }.toList()
      ..sort((left, right) {
        final depthCompare = p.split(right).length.compareTo(p.split(left).length);
        if (depthCompare != 0) {
          return depthCompare;
        }
        return right.length.compareTo(left.length);
      });
    if (normalizedRoots.length <= 1) {
      return normalizedRoots;
    }
    return <String>[normalizedRoots.first];
  }

  Future<void> _persistExecutionPlanRefresh({
    required String actorName,
    required Directory artifactDirectory,
    required ResolvedWorkflow workflow,
    required ExecutionPlan executionPlan,
    required ContextContract contextContract,
    required int actorIndex,
  }) async {
    await File(
      p.join(artifactDirectory.path, 'execution_plan.json'),
    ).writeAsString(
      const JsonEncoder.withIndent('  ').convert(executionPlan.toJson()),
    );
    await File(
      p.join(artifactDirectory.path, 'workflow_steps.md'),
    ).writeAsString(
      _buildWorkflowSteps(
        workflow: workflow,
        executionPlan: executionPlan,
      ),
    );

    final actorDoc = File(p.join(root.path, '.harness', 'actors', '$actorName.md'));
    final actorInstructions = actorDoc.existsSync()
        ? await actorDoc.readAsString()
        : 'Actor instructions not found.';
    final actorBriefPath = p.join(
      artifactDirectory.path,
      'actor_briefs',
      '${(actorIndex + 1).toString().padLeft(2, '0')}_$actorName.md',
    );
    await File(actorBriefPath).writeAsString(
      _buildActorBrief(
        actorName: actorName,
        actorInstructions: actorInstructions,
        workflow: workflow,
        executionPlan: executionPlan,
        actorContract: contextContract.contractFor(actorName),
        artifactDirectory: artifactDirectory,
        materializedInputs: _materializedInputsForArtifact(),
      ),
    );
  }

  Future<void> _refreshActorBriefs({
    required Directory artifactDirectory,
    required ResolvedWorkflow workflow,
    required ExecutionPlan executionPlan,
    required ContextContract contextContract,
    int startIndex = 0,
  }) async {
    final actorBriefDirectory = Directory(
      p.join(artifactDirectory.path, 'actor_briefs'),
    );
    if (!actorBriefDirectory.existsSync()) {
      return;
    }

    final materializedInputs = _materializedInputsForArtifact();
    for (var index = startIndex; index < workflow.actors.length; index++) {
      final actorName = workflow.actors[index];
      final actorDoc = File(p.join(root.path, '.harness', 'actors', '$actorName.md'));
      final actorInstructions = actorDoc.existsSync()
          ? await actorDoc.readAsString()
          : 'Actor instructions not found.';
      final actorBriefPath = p.join(
        artifactDirectory.path,
        'actor_briefs',
        '${(index + 1).toString().padLeft(2, '0')}_$actorName.md',
      );
      await File(actorBriefPath).writeAsString(
        _buildActorBrief(
          actorName: actorName,
          actorInstructions: actorInstructions,
          workflow: workflow,
          executionPlan: executionPlan,
          actorContract: contextContract.contractFor(actorName),
          artifactDirectory: artifactDirectory,
          materializedInputs: materializedInputs,
        ),
      );
    }
  }

  MaterializedInputs _materializedInputsForArtifact() {
    return MaterializedInputs(
      architectureRulesPath: p.join('inputs', 'architecture_rules.md'),
      projectRulesPath: p.join('inputs', 'project_rules.md'),
      forbiddenChangesPath: p.join('inputs', 'forbidden_changes.md'),
      executionPolicyPath: p.join('inputs', 'execution_policy.yaml'),
      rubricPath: p.join('inputs', 'rubric.yaml'),
      requestPath: 'request.yaml',
    );
  }

  List<String> _projectRelativePlanLikelyFiles({
    required Directory artifactDirectory,
    required String projectRoot,
  }) {
    final likelyFiles = _planLikelyFiles(artifactDirectory);
    final projectFileHints = <String>{};
    for (final path in likelyFiles) {
      final projectRelative = _projectRelativeHint(path, projectRoot);
      if (projectRelative != null) {
        projectFileHints.add(projectRelative);
      }
    }
    final sorted = projectFileHints.toList(growable: false)..sort();
    return sorted;
  }

  String? _projectRelativeHint(String path, String projectRoot) {
    final normalized = p.normalize(path);
    if (p.isAbsolute(normalized)) {
      if (_isPathWithinRoot(normalized, projectRoot)) {
        return p.relative(normalized, from: projectRoot);
      }
      return null;
    }
    return _projectPathExists(projectRoot, normalized) ? normalized : null;
  }

  bool _sameExecutionPlan(ExecutionPlan left, ExecutionPlan right) {
    return left.formatCommand == right.formatCommand &&
        _sameStringList(left.analyzeCommands, right.analyzeCommands) &&
        _sameStringList(left.testCommands, right.testCommands);
  }

  bool _sameStringList(List<String> left, List<String> right) {
    if (left.length != right.length) {
      return false;
    }
    for (var index = 0; index < left.length; index++) {
      if (left[index] != right[index]) {
        return false;
      }
    }
    return true;
  }

  Future<Map<String, Object?>?> _runSmokeActor({
    required String actorName,
    required Directory artifactDirectory,
    required ResolvedWorkflow workflow,
    required ExecutionPlan executionPlan,
    required UserRequest userRequest,
    required String? outputPath,
    required String logPath,
    required String projectRoot,
  }) async {
    final response = switch (actorName) {
      'planner' => _buildSmokePlan(
          artifactDirectory: artifactDirectory,
          userRequest: userRequest,
        ),
      'context_builder' => _buildSmokeContextPack(
          artifactDirectory: artifactDirectory,
        ),
      'generator' => _buildSmokeImplementationResult(
          artifactDirectory: artifactDirectory,
        ),
      'executor' => await _buildSmokeExecutionReport(
          executionPlan: executionPlan,
          projectRoot: projectRoot,
        ),
      'evaluator' => _buildSmokeEvaluationResult(
          artifactDirectory: artifactDirectory,
        ),
      _ => null,
    };

    if (response == null) {
      return null;
    }

    await File(logPath).writeAsString(
      const JsonEncoder.withIndent('  ').convert(response),
    );
    if (outputPath != null) {
      await File(outputPath).writeAsString(_toYaml(response));
    }
    return response;
  }

  Future<Map<String, Object?>?> _runFastPathActor({
    required String actorName,
    required Directory artifactDirectory,
    required ResolvedWorkflow workflow,
    required ExecutionPlan executionPlan,
    required UserRequest userRequest,
    required String? outputPath,
    required String logPath,
    required String projectRoot,
  }) async {
    final likelyFiles = _standardFastPathLikelyFiles(
      userRequest: userRequest,
      projectRoot: projectRoot,
    );
    final validationRoots = _standardFastPathValidationRoots(
      userRequest: userRequest,
      projectRoot: projectRoot,
    );
    final validationTargets = _standardFastPathValidationTargets(
      userRequest: userRequest,
      projectRoot: projectRoot,
    );
    if (userRequest.validationProfile != 'standard' ||
        !_canUseStandardFastPath(
          likelyFiles: likelyFiles,
          validationRoots: validationRoots,
          validationTargets: validationTargets,
        )) {
      return null;
    }

    final response = switch (actorName) {
      'planner' => _buildStandardFastPathPlan(
          userRequest: userRequest,
          likelyFiles: likelyFiles,
          validationRoots: validationRoots,
          validationTargets: validationTargets,
        ),
      'context_builder' => _buildStandardFastPathContextPack(
          artifactDirectory: artifactDirectory,
          userRequest: userRequest,
          likelyFiles: likelyFiles,
        ),
      _ => null,
    };

    if (response == null) {
      return null;
    }

    await File(logPath).writeAsString(
      const JsonEncoder.withIndent('  ').convert(response),
    );
    if (outputPath != null) {
      await File(outputPath).writeAsString(_toYaml(response));
    }
    return response;
  }

  bool _canUseStandardFastPath({
    required List<String> likelyFiles,
    required List<String> validationRoots,
    required List<String> validationTargets,
  }) {
    return likelyFiles.isNotEmpty ||
        validationRoots.isNotEmpty ||
        validationTargets.isNotEmpty;
  }

  List<String> _standardFastPathLikelyFiles({
    required UserRequest userRequest,
    required String projectRoot,
  }) {
    final candidates = <String>{
      ...userRequest.context.suspectedFiles,
      ...userRequest.context.relatedFiles,
      ...userRequest.context.validationTargets,
    };
    final sanitized = <String>{};
    for (final path in candidates) {
      final normalized = _projectRelativeHint(path, projectRoot);
      if (normalized != null) {
        sanitized.add(p.normalize(normalized));
      }
    }
    final sorted = sanitized.toList()..sort();
    return sorted;
  }

  List<String> _standardFastPathValidationRoots({
    required UserRequest userRequest,
    required String projectRoot,
  }) {
    final sanitized = <String>{};
    for (final rootPath in userRequest.context.validationRoots) {
      final normalized = _projectRelativeHint(rootPath, projectRoot);
      if (normalized != null) {
        sanitized.add(p.normalize(normalized));
      }
    }
    final sorted = sanitized.toList()..sort();
    return sorted;
  }

  List<String> _standardFastPathValidationTargets({
    required UserRequest userRequest,
    required String projectRoot,
  }) {
    final sanitized = <String>{};
    for (final targetPath in userRequest.context.validationTargets) {
      final normalized = _projectRelativeHint(targetPath, projectRoot);
      if (normalized != null) {
        sanitized.add(p.normalize(normalized));
      }
    }
    final sorted = sanitized.toList()..sort();
    return sorted;
  }

  Map<String, Object?> _buildStandardFastPathPlan({
    required UserRequest userRequest,
    required List<String> likelyFiles,
    required List<String> validationRoots,
    required List<String> validationTargets,
  }) {
    final refinedAcceptance = <String>[
      ...userRequest.definitionOfDone,
      if (validationRoots.isNotEmpty)
        '검증 범위는 ${validationRoots.join(', ')} 안으로 유지된다',
      if (validationTargets.isNotEmpty)
        '검증 대상은 ${validationTargets.join(', ')} 중심으로 유지된다',
    ];

    return {
      'summary':
          'Fast-path plan for `${userRequest.goal}` using request-provided file and validation hints to avoid broad supervisor bootstrap latency.',
      'likely_files': likelyFiles,
      'assumptions': <String>[
        'Request context already contains enough file and validation detail to seed planner output.',
        'Generator can refine implementation details after this fast-path bootstrap.',
      ],
      'substeps': <String>[
        'Use request-provided file hints as the initial implementation focus.',
        'Use request-provided validation hints as the initial executor scope.',
        'Leave deeper implementation judgment to downstream actors.',
      ],
      'risks': <String>[
        'Fast-path planning may miss secondary files not mentioned in the request.',
      ],
      'acceptance_criteria_refined': refinedAcceptance,
    };
  }

  Map<String, Object?> _buildStandardFastPathContextPack({
    required Directory artifactDirectory,
    required UserRequest userRequest,
    required List<String> likelyFiles,
  }) {
    final forbiddenChanges = _readBulletList(
      File(p.join(artifactDirectory.path, 'inputs', 'forbidden_changes.md')),
    );
    return {
      'relevant_files': likelyFiles
          .map(
            (path) => {
              'path': path,
              'why': userRequest.context.suspectedFiles.contains(path)
                  ? 'Request identifies this as a likely implementation file.'
                  : userRequest.context.validationTargets.contains(path)
                  ? 'Request identifies this as a primary validation target.'
                  : 'Request-related file carried into the fast-path context bootstrap.',
            },
          )
          .toList(growable: false),
      'repo_patterns': <String>[
        'Prefer request-provided file hints as the initial scope before widening.',
        'Keep supervisor orchestration narrow and explicit when validation roots or targets are already known.',
      ],
      'test_patterns': <String>[
        if (userRequest.context.validationTargets.isNotEmpty)
          'Start from explicit validation targets before considering broader package-level tests.',
      ],
      'forbidden_changes': forbiddenChanges,
      'implementation_hints': <String>[
        'Treat this context pack as a fast bootstrap; widen only if generator or evaluator finds a concrete gap.',
      ],
    };
  }

  Map<String, Object?> _buildSmokePlan({
    required Directory artifactDirectory,
    required UserRequest userRequest,
  }) {
    final requestFile = p.join(root.path, userRequest.requestPath);
    return {
      'summary':
          'Smoke-profile plan for `${userRequest.goal}` focused on validating the separated rail control-plane actor chain without broad repository changes.',
      'likely_files': <String>[
        p.join(root.path, 'bin', 'rail.dart'),
        p.join(root.path, '.harness', 'supervisor', 'execution_policy.yaml'),
        requestFile,
      ],
      'assumptions': <String>[
        'Smoke validation should stay inside the rail control-plane repo unless the execution plan explicitly calls the external target repo.',
        'Schema-valid actor outputs are sufficient for this smoke profile.',
      ],
      'substeps': <String>[
        'Produce minimal schema-valid plan/context/implementation artifacts.',
        'Run smoke validation commands from the execution plan.',
        'Decide pass or revise from the smoke execution report.',
      ],
      'risks': <String>[
        'Smoke profile verifies control-plane flow, not full target-repo correctness.',
      ],
      'acceptance_criteria_refined': userRequest.definitionOfDone,
    };
  }

  Map<String, Object?> _buildSmokeContextPack({
    required Directory artifactDirectory,
  }) {
    final planFile = File(p.join(artifactDirectory.path, 'plan.yaml'));
    final planMap = _asMap(_loadYamlValue(planFile), context: planFile.path);
    final likelyFiles = _readOptionalStringList(planMap, 'likely_files');
    final forbiddenChanges = _readBulletList(
      File(p.join(artifactDirectory.path, 'inputs', 'forbidden_changes.md')),
    );
    return {
      'relevant_files': likelyFiles
          .map(
            (path) => {
              'path': path,
              'why': 'Smoke-profile actor chain depends on this control-plane file.',
            },
          )
          .toList(growable: false),
      'repo_patterns': <String>[
        'Smoke validation uses deterministic actor outputs to verify control-plane orchestration quickly.',
        'Executor commands still come from the generated execution plan, even when actor outputs are synthesized.',
      ],
      'test_patterns': <String>[
        'Smoke requests favor reachability and schema validation over full lint/test coverage.',
      ],
      'forbidden_changes': forbiddenChanges,
      'implementation_hints': <String>[
        'Keep smoke artifacts deterministic and scoped to the rail repo.',
      ],
    };
  }

  Map<String, Object?> _buildSmokeImplementationResult({
    required Directory artifactDirectory,
  }) {
    final planFile = File(p.join(artifactDirectory.path, 'plan.yaml'));
    final planMap = _asMap(_loadYamlValue(planFile), context: planFile.path);
    final likelyFiles = _readOptionalStringList(planMap, 'likely_files');
    return {
      'changed_files': <String>[],
      'patch_summary': <String>[
        'Smoke profile skips repository edits and validates orchestration using synthesized actor outputs.',
      ],
      'tests_added_or_updated': <String>[],
      'known_limits': likelyFiles.isEmpty
          ? <String>[]
          : <String>[
              'Likely implementation scope for non-smoke execution: ${likelyFiles.join(', ')}',
            ],
    };
  }

  Future<Map<String, Object?>> _buildSmokeExecutionReport({
    required ExecutionPlan executionPlan,
    required String projectRoot,
  }) async {
    final logs = <String>[];
    final failureDetails = <String>[];
    var formatPass = executionPlan.formatCommand == null;
    var analyzePass = true;
    var passedTests = 0;
    var failedTests = 0;

    if (executionPlan.formatCommand != null) {
      final result = await _runCommand(
        'zsh',
        ['-lc', executionPlan.formatCommand!],
        workingDirectory: projectRoot,
        timeout: const Duration(minutes: 1),
      );
      formatPass = result.exitCode == 0;
      logs.add(_commandSummary(executionPlan.formatCommand!, result.exitCode));
      if (!formatPass) {
        failureDetails.add('Format command failed: ${executionPlan.formatCommand}');
      }
    }

    for (final command in executionPlan.analyzeCommands) {
      final result = await _runCommand(
        'zsh',
        ['-lc', command],
        workingDirectory: projectRoot,
        timeout: const Duration(minutes: 1),
      );
      logs.add(_commandSummary(command, result.exitCode));
      if (result.exitCode != 0) {
        analyzePass = false;
        failureDetails.add('Analyze command failed: $command');
      }
    }

    for (final command in executionPlan.testCommands) {
      final result = await _runCommand(
        'zsh',
        ['-lc', command],
        workingDirectory: projectRoot,
        timeout: const Duration(minutes: 1),
      );
      logs.add(_commandSummary(command, result.exitCode));
      if (result.exitCode == 0) {
        passedTests += 1;
      } else {
        failedTests += 1;
        failureDetails.add('Test command failed: $command');
      }
    }

    return {
      'format': formatPass ? 'pass' : 'fail',
      'analyze': analyzePass ? 'pass' : 'fail',
      'tests': {
        'total': executionPlan.testCommands.length,
        'passed': passedTests,
        'failed': failedTests,
      },
      'failure_details': failureDetails,
      'logs': logs,
    };
  }

  Map<String, Object?> _buildSmokeEvaluationResult({
    required Directory artifactDirectory,
  }) {
    final executionReportFile = File(
      p.join(artifactDirectory.path, 'execution_report.yaml'),
    );
    final executionReport = _asMap(
      _loadYamlValue(executionReportFile),
      context: executionReportFile.path,
    );
    final analyzePass = _readString(executionReport, 'analyze') == 'pass';
    final tests = _readMap(executionReport, 'tests');
    final testFailures = _readInt(tests, 'failed');
    final pass = analyzePass && testFailures == 0;
    return {
      'decision': pass ? 'pass' : 'revise',
      'scores': {
        'requirements': pass ? 1 : 0.5,
        'architecture': 1,
        'regression_risk': pass ? 1 : 0.5,
      },
      'findings': pass
          ? <String>[
              'Smoke-profile actor chain completed with schema-valid artifacts and passing smoke validation commands.',
            ]
          : <String>[
              'Smoke-profile validation reported at least one failed command.',
            ],
      'reason_codes': pass
          ? <String>[]
          : <String>['smoke_validation_failed'],
      if (!pass) 'next_action': 'tighten_validation',
    };
  }

  List<String> _readBulletList(File file) {
    if (!file.existsSync()) {
      return const [];
    }
    return file
        .readAsLinesSync()
        .map((line) => line.trim())
        .where((line) => line.startsWith('- '))
        .map((line) => line.substring(2).trim())
        .where((line) => line.isNotEmpty)
        .toList(growable: false);
  }

  String _commandSummary(String command, int exitCode) {
    return 'exit=$exitCode :: $command';
  }

  String _actorWorkingDirectory({
    required String actorName,
    required Directory artifactDirectory,
    required String projectRoot,
  }) {
    switch (actorName) {
      case 'planner':
      case 'context_builder':
      case 'evaluator':
      case 'integrator':
        return root.path;
      case 'executor':
        return projectRoot;
      case 'generator':
        final likelyFiles = _planLikelyFiles(artifactDirectory);
        final touchesHarnessOnly =
            likelyFiles.isNotEmpty &&
            likelyFiles.every((path) => _isPathWithinRoot(path, root.path)) &&
            likelyFiles.every((path) => !_isPathWithinRoot(path, projectRoot));
        return touchesHarnessOnly ? root.path : projectRoot;
      default:
        return projectRoot;
    }
  }

  List<String> _planLikelyFiles(Directory artifactDirectory) {
    final planFile = File(p.join(artifactDirectory.path, 'plan.yaml'));
    if (!planFile.existsSync()) {
      return const [];
    }

    final planMap = _asMap(_loadYamlValue(planFile), context: planFile.path);
    final rawLikelyFiles = planMap['likely_files'];
    if (rawLikelyFiles is! List) {
      return const [];
    }

    return rawLikelyFiles
        .whereType<String>()
        .map((path) => p.normalize(path))
        .toList(growable: false);
  }

  bool _isPathWithinRoot(String path, String rootPath) {
    if (!p.isAbsolute(path)) {
      return false;
    }
    final normalizedPath = p.normalize(path);
    final normalizedRoot = p.normalize(rootPath);
    return normalizedPath == normalizedRoot ||
        normalizedPath.startsWith('$normalizedRoot${p.separator}');
  }

  String _buildCodexExecutionPrompt({
    required String actorName,
    required String actorBriefPath,
    required String artifactDirectory,
    required String projectRoot,
    required String actorWorkingDirectory,
    required String? outputPath,
    required bool returnsStructuredOutput,
  }) {
    final outputInstruction = outputPath == null
        ? 'Do not write a structured artifact file for this actor.'
        : returnsStructuredOutput
            ? 'Return the actor result as structured JSON matching the provided output schema. Do not edit `$outputPath` directly; the harness runtime will write it.'
            : 'Update `$outputPath` so it contains only YAML matching its schema.';
    final fileScope = switch (actorName) {
      'planner' || 'context_builder' || 'evaluator' || 'integrator' =>
        'Do not modify repository source files outside the artifact directory.',
      'executor' =>
        'Run the planned commands if needed, then return the execution report only. Do not create extra log files inside the artifact directory; summarize evidence in `failure_details` and `logs`.',
      'generator' =>
        'You may modify repository source files if needed for the task, and you must also update the implementation result artifact.',
      _ => 'Stay within the repository and artifact scope described in the brief.',
    };
    return '''
You are executing the `$actorName` actor for a rail harness workflow.

Target project root: `$projectRoot`
Harness artifact root: `$artifactDirectory`
Actor working directory: `$actorWorkingDirectory`

Read and follow the actor brief at `$actorBriefPath`.
$outputInstruction
$fileScope

Requirements:
- Follow the actor instructions and contract exactly.
- If you are returning structured output, the final response must be only the schema-matching JSON object.
- Keep changes minimal and scoped to this actor.
- If the actor requires repository code changes, make them before returning.
''';
  }
}

class UserRequest {
  UserRequest({
    required this.taskType,
    required this.goal,
    required this.context,
    required this.constraints,
    required this.definitionOfDone,
    required this.riskTolerance,
    required this.validationProfile,
    required this.requestPath,
    this.priority,
  });

  factory UserRequest.fromMap(
    Map<String, dynamic> map, {
    required String requestPath,
  }) {
    final taskType = _readString(map, 'task_type');
    final goal = _readString(map, 'goal');
    final contextMap = _readMap(map, 'context');
    final constraints = _readStringList(map, 'constraints');
    final definitionOfDone = _readStringList(map, 'definition_of_done');
    final riskTolerance = _readString(map, 'risk_tolerance');
    final validationProfile =
        map['validation_profile']?.toString() ?? 'standard';
    final priority = map['priority']?.toString();

    const allowedTaskTypes = {
      'bug_fix',
      'feature_addition',
      'safe_refactor',
      'test_repair',
    };
    const allowedRiskTolerance = {'low', 'medium', 'high'};
    const allowedValidationProfiles = {'standard', 'smoke'};

    if (!allowedTaskTypes.contains(taskType)) {
      throw ArgumentError('Unsupported task_type: $taskType');
    }
    if (!allowedRiskTolerance.contains(riskTolerance)) {
      throw ArgumentError('Unsupported risk_tolerance: $riskTolerance');
    }
    if (!allowedValidationProfiles.contains(validationProfile)) {
      throw ArgumentError(
        'Unsupported validation_profile: $validationProfile',
      );
    }

    return UserRequest(
      taskType: taskType,
      goal: goal,
      context: RequestContext.fromMap(contextMap),
      constraints: constraints,
      definitionOfDone: definitionOfDone,
      riskTolerance: riskTolerance,
      validationProfile: validationProfile,
      priority: priority,
      requestPath: requestPath,
    );
  }

  final String taskType;
  final String goal;
  final RequestContext context;
  final List<String> constraints;
  final List<String> definitionOfDone;
  final String riskTolerance;
  final String validationProfile;
  final String? priority;
  final String requestPath;
}

class RequestContext {
  RequestContext({
    required this.relatedFiles,
    required this.suspectedFiles,
    required this.validationRoots,
    required this.validationTargets,
    this.feature,
  });

  factory RequestContext.fromMap(Map<String, dynamic> map) {
    return RequestContext(
      feature: map['feature']?.toString(),
      suspectedFiles: _readOptionalStringList(map, 'suspected_files'),
      relatedFiles: _readOptionalStringList(map, 'related_files'),
      validationRoots: _readOptionalStringList(map, 'validation_roots'),
      validationTargets: _readOptionalStringList(map, 'validation_targets'),
    );
  }

  final String? feature;
  final List<String> suspectedFiles;
  final List<String> relatedFiles;
  final List<String> validationRoots;
  final List<String> validationTargets;
}

class ComposedRequest {
  ComposedRequest({
    required this.file,
    required this.request,
  });

  final File file;
  final Map<String, Object?> request;
}

class CommandResult {
  CommandResult({
    required this.exitCode,
    required this.stdout,
    required this.stderr,
  });

  final int exitCode;
  final String stdout;
  final String stderr;
}

class MaterializedInputs {
  MaterializedInputs({
    required this.architectureRulesPath,
    required this.projectRulesPath,
    required this.forbiddenChangesPath,
    required this.executionPolicyPath,
    required this.rubricPath,
    required this.requestPath,
  });

  final String architectureRulesPath;
  final String projectRulesPath;
  final String forbiddenChangesPath;
  final String executionPolicyPath;
  final String rubricPath;
  final String requestPath;
}

class Registry {
  Registry(this._tasks);

  factory Registry.fromMap(Map<String, dynamic> map) {
    final taskRegistry = _readMap(map, 'task_registry');
    final tasks = <String, TaskConfig>{};
    for (final entry in taskRegistry.entries) {
      tasks[entry.key] = TaskConfig.fromMap(_readMap(taskRegistry, entry.key));
    }
    return Registry(tasks);
  }

  final Map<String, TaskConfig> _tasks;

  TaskConfig taskFor(String taskType) {
    final task = _tasks[taskType];
    if (task == null) {
      throw ArgumentError('No task config found for $taskType');
    }
    return task;
  }
}

class TaskConfig {
  TaskConfig({
    required this.rubric,
    required this.actors,
    required this.generatorMaxRetry,
    required this.requiredOutputs,
  });

  factory TaskConfig.fromMap(Map<String, dynamic> map) {
    final retry = _readMap(map, 'retry');
    return TaskConfig(
      rubric: _readString(map, 'rubric'),
      actors: _readStringList(map, 'actors'),
      generatorMaxRetry: _readInt(retry, 'generator_max_retry'),
      requiredOutputs: _readStringList(map, 'required_output'),
    );
  }

  final String rubric;
  final List<String> actors;
  final int generatorMaxRetry;
  final List<String> requiredOutputs;
}

class TaskRouter {
  TaskRouter({
    required Map<String, TaskRoute> routes,
    required Map<String, int> riskBudgets,
  }) : _routes = routes,
       _riskBudgets = riskBudgets;

  factory TaskRouter.fromMap(Map<String, dynamic> map) {
    final routesMap = _readMap(map, 'routes');
    final routes = <String, TaskRoute>{};
    for (final entry in routesMap.entries) {
      routes[entry.key] = TaskRoute.fromMap(_readMap(routesMap, entry.key));
    }

    final riskToleranceDefaults = _readMap(
      _readMap(map, 'defaults'),
      'risk_tolerance',
    );
    final riskBudgets = <String, int>{};
    for (final entry in riskToleranceDefaults.entries) {
      riskBudgets[entry.key] = _readInt(
        _readMap(riskToleranceDefaults, entry.key),
        'retry_budget',
      );
    }

    return TaskRouter(routes: routes, riskBudgets: riskBudgets);
  }

  final Map<String, TaskRoute> _routes;
  final Map<String, int> _riskBudgets;

  TaskRoute routeFor(String taskType) {
    final route = _routes[taskType];
    if (route == null) {
      throw ArgumentError('No task route found for $taskType');
    }
    return route.copyWithRiskBudgets(_riskBudgets);
  }
}

class TaskRoute {
  TaskRoute({
    required this.actors,
    required this.riskBudgets,
  });

  factory TaskRoute.fromMap(Map<String, dynamic> map) {
    return TaskRoute(
      actors: _readStringList(map, 'actors'),
      riskBudgets: const <String, int>{},
    );
  }

  final List<String> actors;
  final Map<String, int> riskBudgets;

  TaskRoute copyWithRiskBudgets(Map<String, int> value) {
    return TaskRoute(actors: actors, riskBudgets: value);
  }

  int retryBudgetFor(String riskTolerance) {
    return riskBudgets[riskTolerance] ?? 1;
  }
}

class ContextContract {
  ContextContract({
    required this.requiredFields,
    required this.optionalFields,
    required this.actorContracts,
    required this.terminationConditions,
  });

  factory ContextContract.fromMap(Map<String, dynamic> map) {
    final inputMap = _readMap(map, 'input');
    final actorFlowSchema = _readMap(map, 'actor_flow_schema');
    final contracts = <String, ActorContract>{};
    for (final entry in actorFlowSchema.entries) {
      contracts[entry.key] = ActorContract.fromMap(
        _readMap(actorFlowSchema, entry.key),
      );
    }
    return ContextContract(
      requiredFields: _readStringList(inputMap, 'required_fields'),
      optionalFields: _readStringList(inputMap, 'optional_fields'),
      actorContracts: contracts,
      terminationConditions: _readStringList(_readMap(map, 'termination'), 'conditions'),
    );
  }

  final List<String> requiredFields;
  final List<String> optionalFields;
  final Map<String, ActorContract> actorContracts;
  final List<String> terminationConditions;

  bool hasActor(String actorName) => actorContracts.containsKey(actorName);

  ActorContract contractFor(String actorName) {
    final contract = actorContracts[actorName];
    if (contract == null) {
      throw ArgumentError('Missing actor contract for $actorName');
    }
    return contract;
  }
}

class ActorContract {
  ActorContract({
    required this.inputs,
    required this.outputs,
  });

  factory ActorContract.fromMap(Map<String, dynamic> map) {
    return ActorContract(
      inputs: _readStringList(map, 'in'),
      outputs: _readStringList(map, 'out'),
    );
  }

  final List<String> inputs;
  final List<String> outputs;
}

class Policy {
  Policy({
    required this.retryBudgets,
    required this.contextRebuildBudget,
    required this.validationTightenBudget,
    required this.passIf,
    required this.reviseIf,
    required this.rejectIf,
  });

  factory Policy.fromMap(Map<String, dynamic> map) {
    final retryRules = _readMap(map, 'retry_rules');
    final supervisorLoop = _readMap(map, 'supervisor_loop');
    final budgets = <String, int>{};
    for (final entry in retryRules.entries) {
      budgets[entry.key] = _readInt(
        _readMap(retryRules, entry.key),
        'max_generator_retry',
      );
    }
    return Policy(
      retryBudgets: budgets,
      contextRebuildBudget: _readInt(supervisorLoop, 'max_context_rebuild'),
      validationTightenBudget: _readInt(
        supervisorLoop,
        'max_validation_tighten',
      ),
      passIf: _readStringList(map, 'pass_if'),
      reviseIf: _readStringList(map, 'revise_if'),
      rejectIf: _readStringList(map, 'reject_if'),
    );
  }

  final Map<String, int> retryBudgets;
  final int contextRebuildBudget;
  final int validationTightenBudget;
  final List<String> passIf;
  final List<String> reviseIf;
  final List<String> rejectIf;

  int retryBudgetFor(String riskTolerance) {
    return retryBudgets[riskTolerance] ?? 1;
  }
}

class ExecutionPolicy {
  ExecutionPolicy({
    required this.artifactRoot,
    required this.formatCommand,
    required this.packageAnalyzeCommand,
    required this.workspaceAnalyzeCommand,
    required this.smokeAnalyzeCommand,
    required this.packageTestCommand,
    required this.workspaceTestCommand,
    required this.smokeTestCommand,
    required this.createPlaceholders,
    required this.createActorBriefs,
    required this.persistJsonSnapshots,
  });

  factory ExecutionPolicy.fromMap(Map<String, dynamic> map) {
    final runtime = _readMap(map, 'runtime');
    return ExecutionPolicy(
      artifactRoot: _readString(_readMap(map, 'artifacts'), 'root'),
      formatCommand: _readString(_readMap(map, 'format'), 'command'),
      packageAnalyzeCommand: _readString(
        _readMap(map, 'analyze'),
        'package_command',
      ),
      workspaceAnalyzeCommand: _readString(
        _readMap(map, 'analyze'),
        'workspace_fallback',
      ),
      smokeAnalyzeCommand: _readString(_readMap(map, 'analyze'), 'smoke_command'),
      packageTestCommand: _readString(
        _readMap(map, 'tests'),
        'package_command',
      ),
      workspaceTestCommand: _readString(
        _readMap(map, 'tests'),
        'workspace_fallback',
      ),
      smokeTestCommand: _readString(_readMap(map, 'tests'), 'smoke_command'),
      createPlaceholders: _readBool(runtime, 'create_placeholders'),
      createActorBriefs: _readBool(runtime, 'create_actor_briefs'),
      persistJsonSnapshots: _readBool(runtime, 'persist_json_snapshots'),
    );
  }

  final String artifactRoot;
  final String formatCommand;
  final String packageAnalyzeCommand;
  final String workspaceAnalyzeCommand;
  final String smokeAnalyzeCommand;
  final String packageTestCommand;
  final String workspaceTestCommand;
  final String smokeTestCommand;
  final bool createPlaceholders;
  final bool createActorBriefs;
  final bool persistJsonSnapshots;
}

class TestTargetRules {
  TestTargetRules({
    required this.sourceSuffix,
    required this.testSuffix,
    required this.featureTestRoot,
    required this.packageTestRoot,
    required this.pathRules,
  });

  factory TestTargetRules.fromMap(Map<String, dynamic> map) {
    final naming = _readMap(map, 'naming');
    final fallback = _readMap(map, 'fallback');
    final pathRules = _readListOfMaps(map, 'path_rules')
        .map(TestPathRule.fromMap)
        .toList(growable: false);
    return TestTargetRules(
      sourceSuffix: _readString(naming, 'source_suffix'),
      testSuffix: _readString(naming, 'test_suffix'),
      featureTestRoot: _readString(fallback, 'feature_test_root'),
      packageTestRoot: _readString(fallback, 'package_test_root'),
      pathRules: pathRules,
    );
  }

  final String sourceSuffix;
  final String testSuffix;
  final String featureTestRoot;
  final String packageTestRoot;
  final List<TestPathRule> pathRules;

  List<String> inferTargets({
    required String projectRoot,
    required List<String> fileHints,
    String? featureName,
  }) {
    final targets = <String>{};
    final packageRoots = <String>{};
    for (final hint in fileHints) {
      final normalized = p.normalize(hint);
      final segments = p.split(normalized);
      if (segments.contains('test')) {
        targets.add(normalized);
        if (segments.isNotEmpty && segments.first == 'test') {
          packageRoots.add('.');
        }
        continue;
      }

      if (!normalized.endsWith(sourceSuffix)) {
        continue;
      }

      if (segments.isNotEmpty && segments.first == 'lib') {
        packageRoots.add('.');
        final relativeInsideSource = p.joinAll(segments.skip(1));
        final testPath = p.join(
          'test',
          relativeInsideSource.replaceFirst(sourceSuffix, testSuffix),
        );
        if (_projectPathExists(projectRoot, testPath)) {
          targets.add(p.normalize(testPath));
        } else {
          targets.add(p.normalize(p.dirname(testPath)));
        }
        continue;
      }

      final matchedRule = pathRules.firstWhereOrNull(
        (rule) =>
            segments.length >= 2 &&
            segments.first == rule.sourceRoot &&
            segments.contains(rule.sourceSegment),
      );
      if (matchedRule == null) {
        continue;
      }

      final sourceSegmentIndex = segments.indexOf(matchedRule.sourceSegment);
      final packageRoot = p.joinAll(segments.take(sourceSegmentIndex));
      packageRoots.add(packageRoot);
      final relativeInsideSource = p.joinAll(segments.skip(sourceSegmentIndex + 1));
      final testPath = p.join(
        packageRoot,
        matchedRule.testSegment,
        relativeInsideSource.replaceFirst(sourceSuffix, testSuffix),
      );
      final testFile = File(p.join(projectRoot, testPath));
      if (testFile.existsSync()) {
        targets.add(p.normalize(testPath));
      } else {
        targets.add(p.normalize(p.dirname(testPath)));
      }
    }

    if (targets.isEmpty && featureName != null && featureName.isNotEmpty) {
      final rootCandidate = p.join(packageTestRoot, featureName);
      if (_projectPathExists(projectRoot, rootCandidate)) {
        targets.add(p.normalize(rootCandidate));
      }
      final appCandidate = p.join(featureTestRoot, featureName);
      if (_projectPathExists(projectRoot, appCandidate)) {
        targets.add(p.normalize(appCandidate));
      }
      for (final packageRoot in packageRoots) {
        final packageCandidate = p.join(packageRoot, packageTestRoot, featureName);
        if (_projectPathExists(projectRoot, packageCandidate)) {
          targets.add(p.normalize(packageCandidate));
        }
      }
    }

    return targets.toList()..sort();
  }
}

class ResolvedWorkflow {
  ResolvedWorkflow({
    required this.taskId,
    required this.taskType,
    required this.projectRoot,
    required this.actors,
    required this.rubricPath,
    required this.generatorRetryBudget,
    required this.contextRebuildBudget,
    required this.validationTightenBudget,
    required this.changedFileHints,
    required this.inferredTestTargets,
    required this.requiredOutputs,
    required this.requestPath,
    required this.terminationConditions,
    required this.passIf,
    required this.reviseIf,
    required this.rejectIf,
  });

  final String taskId;
  final String taskType;
  final String projectRoot;
  final List<String> actors;
  final String rubricPath;
  final int generatorRetryBudget;
  final int contextRebuildBudget;
  final int validationTightenBudget;
  final List<String> changedFileHints;
  final List<String> inferredTestTargets;
  final List<String> requiredOutputs;
  final String requestPath;
  final List<String> terminationConditions;
  final List<String> passIf;
  final List<String> reviseIf;
  final List<String> rejectIf;

  Map<String, Object?> toJson() {
    return {
      'taskId': taskId,
      'taskType': taskType,
      'projectRoot': projectRoot,
      'actors': actors,
      'rubricPath': rubricPath,
      'generatorRetryBudget': generatorRetryBudget,
      'contextRebuildBudget': contextRebuildBudget,
      'validationTightenBudget': validationTightenBudget,
      'changedFileHints': changedFileHints,
      'inferredTestTargets': inferredTestTargets,
      'requiredOutputs': requiredOutputs,
      'requestPath': requestPath,
      'terminationConditions': terminationConditions,
      'passIf': passIf,
      'reviseIf': reviseIf,
      'rejectIf': rejectIf,
    };
  }

  factory ResolvedWorkflow.fromJson(Map<String, dynamic> map) {
    return ResolvedWorkflow(
      taskId: _readString(map, 'taskId'),
      taskType: _readString(map, 'taskType'),
      projectRoot: _readString(map, 'projectRoot'),
      actors: _readStringList(map, 'actors'),
      rubricPath: _readString(map, 'rubricPath'),
      generatorRetryBudget: _readInt(map, 'generatorRetryBudget'),
      contextRebuildBudget: _readInt(map, 'contextRebuildBudget'),
      validationTightenBudget: _readInt(map, 'validationTightenBudget'),
      changedFileHints: _readStringList(map, 'changedFileHints'),
      inferredTestTargets: _readStringList(map, 'inferredTestTargets'),
      requiredOutputs: _readStringList(map, 'requiredOutputs'),
      requestPath: _readString(map, 'requestPath'),
      terminationConditions: _readStringList(map, 'terminationConditions'),
      passIf: _readStringList(map, 'passIf'),
      reviseIf: _readStringList(map, 'reviseIf'),
      rejectIf: _readStringList(map, 'rejectIf'),
    );
  }
}

class ExecutionPlan {
  ExecutionPlan({
    required this.formatCommand,
    required this.analyzeCommands,
    required this.testCommands,
  });

  final String? formatCommand;
  final List<String> analyzeCommands;
  final List<String> testCommands;

  Map<String, Object?> toJson() {
    return {
      'formatCommand': formatCommand,
      'analyzeCommands': analyzeCommands,
      'testCommands': testCommands,
    };
  }

  factory ExecutionPlan.fromJson(Map<String, dynamic> map) {
    return ExecutionPlan(
      formatCommand: map['formatCommand']?.toString(),
      analyzeCommands: _readStringList(map, 'analyzeCommands'),
      testCommands: _readStringList(map, 'testCommands'),
    );
  }
}

class HarnessState {
  HarnessState({
    required this.taskId,
    required this.status,
    required this.currentActor,
    required this.completedActors,
    required this.generatorRetriesRemaining,
    required this.contextRebuildsRemaining,
    required this.validationTighteningsRemaining,
    required this.lastDecision,
    required this.lastReasonCodes,
    required this.actionHistory,
  });

  factory HarnessState.fromJson(Map<String, dynamic> map) {
    return HarnessState(
      taskId: _readString(map, 'taskId'),
      status: _readString(map, 'status'),
      currentActor: map['currentActor']?.toString(),
      completedActors: _readStringList(map, 'completedActors'),
      generatorRetriesRemaining: _readOptionalInt(
            map,
            'generatorRetriesRemaining',
          ) ??
          0,
      contextRebuildsRemaining:
          _readOptionalInt(map, 'contextRebuildsRemaining') ?? 0,
      validationTighteningsRemaining:
          _readOptionalInt(map, 'validationTighteningsRemaining') ?? 0,
      lastDecision: map['lastDecision']?.toString(),
      lastReasonCodes: _readOptionalStringList(map, 'lastReasonCodes'),
      actionHistory: _readOptionalStringList(map, 'actionHistory'),
    );
  }

  final String taskId;
  final String status;
  final String? currentActor;
  final List<String> completedActors;
  final int generatorRetriesRemaining;
  final int contextRebuildsRemaining;
  final int validationTighteningsRemaining;
  final String? lastDecision;
  final List<String> lastReasonCodes;
  final List<String> actionHistory;

  HarnessState copyWith({
    String? status,
    String? currentActor,
    bool clearCurrentActor = false,
    List<String>? completedActors,
    int? generatorRetriesRemaining,
    int? contextRebuildsRemaining,
    int? validationTighteningsRemaining,
    String? lastDecision,
    List<String>? lastReasonCodes,
    List<String>? actionHistory,
  }) {
    return HarnessState(
      taskId: taskId,
      status: status ?? this.status,
      currentActor: clearCurrentActor ? null : (currentActor ?? this.currentActor),
      completedActors: completedActors ?? this.completedActors,
      generatorRetriesRemaining:
          generatorRetriesRemaining ?? this.generatorRetriesRemaining,
      contextRebuildsRemaining:
          contextRebuildsRemaining ?? this.contextRebuildsRemaining,
      validationTighteningsRemaining:
          validationTighteningsRemaining ?? this.validationTighteningsRemaining,
      lastDecision: lastDecision ?? this.lastDecision,
      lastReasonCodes: lastReasonCodes ?? this.lastReasonCodes,
      actionHistory: actionHistory ?? this.actionHistory,
    );
  }

  Map<String, Object?> toJson() {
    return {
      'taskId': taskId,
      'status': status,
      'currentActor': currentActor,
      'completedActors': completedActors,
      'generatorRetriesRemaining': generatorRetriesRemaining,
      'contextRebuildsRemaining': contextRebuildsRemaining,
      'validationTighteningsRemaining': validationTighteningsRemaining,
      'lastDecision': lastDecision,
      'lastReasonCodes': lastReasonCodes,
      'actionHistory': actionHistory,
    };
  }
}

class TestPathRule {
  TestPathRule({
    required this.sourceRoot,
    required this.sourceSegment,
    required this.testSegment,
  });

  factory TestPathRule.fromMap(Map<String, dynamic> map) {
    return TestPathRule(
      sourceRoot: _readString(map, 'source_root'),
      sourceSegment: _readString(map, 'source_segment'),
      testSegment: _readString(map, 'test_segment'),
    );
  }

  final String sourceRoot;
  final String sourceSegment;
  final String testSegment;
}

class SchemaValidator {
  SchemaValidator(this.schema, {required this.schemaName});

  final Map<String, dynamic> schema;
  final String schemaName;

  void validate(dynamic value, {required String fileLabel}) {
    final errors = _validateNode(
      schema,
      value,
      path: r'$',
    );
    if (errors.isNotEmpty) {
      throw StateError(
        'Schema validation failed for $fileLabel against $schemaName:\n${errors.map((error) => '- $error').join('\n')}',
      );
    }
  }

  List<String> _validateNode(
    Map<String, dynamic> nodeSchema,
    dynamic value, {
    required String path,
  }) {
    final errors = <String>[];

    final oneOf = nodeSchema['oneOf'];
    if (oneOf is List) {
      final branches = oneOf
          .whereType<Map>()
          .map(Map<String, dynamic>.from)
          .toList(growable: false);
      final branchErrors = branches
          .map((branch) => _validateNode(branch, value, path: path))
          .toList(growable: false);
      final anyValid = branchErrors.any((branch) => branch.isEmpty);
      if (!anyValid) {
        errors.add('$path did not match any allowed schema branch');
      }
      return errors;
    }

    final expectedType = nodeSchema['type'];
    if (expectedType case 'object') {
      if (value is! Map<String, dynamic>) {
        errors.add('$path expected object');
        return errors;
      }

      final requiredFields = (nodeSchema['required'] as List?)
              ?.whereType<String>()
              .toList(growable: false) ??
          const <String>[];
      for (final field in requiredFields) {
        if (!value.containsKey(field)) {
          errors.add('$path missing required field `$field`');
        }
      }

      final properties = nodeSchema['properties'];
      if (properties is Map) {
        final propertyMap = Map<String, dynamic>.from(properties);
        for (final entry in propertyMap.entries) {
          if (!value.containsKey(entry.key)) {
            continue;
          }
          final childSchema = Map<String, dynamic>.from(entry.value as Map);
          errors.addAll(
            _validateNode(
              childSchema,
              value[entry.key],
              path: '$path.${entry.key}',
            ),
          );
        }
      }
    } else if (expectedType case 'array') {
      if (value is! List) {
        errors.add('$path expected array');
        return errors;
      }
      final items = nodeSchema['items'];
      if (items is Map) {
        final itemSchema = Map<String, dynamic>.from(items);
        for (var index = 0; index < value.length; index++) {
          errors.addAll(
            _validateNode(
              itemSchema,
              value[index],
              path: '$path[$index]',
            ),
          );
        }
      }
    } else if (expectedType case 'string') {
      if (value is! String) {
        errors.add('$path expected string');
        return errors;
      }
    } else if (expectedType case 'integer') {
      if (value is! int) {
        errors.add('$path expected integer');
        return errors;
      }
    } else if (expectedType case 'number') {
      if (value is! num) {
        errors.add('$path expected number');
        return errors;
      }
    }

    final minimum = nodeSchema['minimum'];
    if (minimum is num && value is num && value < minimum) {
      errors.add('$path expected >= $minimum');
    }

    final maximum = nodeSchema['maximum'];
    if (maximum is num && value is num && value > maximum) {
      errors.add('$path expected <= $maximum');
    }

    final enumValues = nodeSchema['enum'];
    if (enumValues is List && !enumValues.contains(value)) {
      errors.add('$path expected one of ${enumValues.join(', ')}');
    }

    return errors;
  }
}

Map<String, dynamic> _asMap(dynamic value, {required String context}) {
  if (value case final Map<String, dynamic> map) {
    return map;
  }
  throw StateError('Expected map for $context');
}

Map<String, dynamic> _readMap(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value is Map<String, dynamic>) {
    return value;
  }
  throw ArgumentError('Expected `$key` to be a map.');
}

String _readString(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value is String && value.isNotEmpty) {
    return value;
  }
  throw ArgumentError('Expected `$key` to be a non-empty string.');
}

int _readInt(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value is int) {
    return value;
  }
  throw ArgumentError('Expected `$key` to be an int.');
}

int? _readOptionalInt(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value == null) {
    return null;
  }
  if (value is int) {
    return value;
  }
  throw ArgumentError('Expected `$key` to be an int.');
}

bool _readBool(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value is bool) {
    return value;
  }
  throw ArgumentError('Expected `$key` to be a bool.');
}

List<Map<String, dynamic>> _readListOfMaps(
  Map<String, dynamic> source,
  String key,
) {
  final value = source[key];
  if (value is List) {
    return value
        .map((item) => _asMap(item, context: key))
        .toList(growable: false);
  }
  throw ArgumentError('Expected `$key` to be a list of maps.');
}

List<String> _readStringList(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value is List && value.every((item) => item is String)) {
    return value.cast<String>();
  }
  throw ArgumentError('Expected `$key` to be a list of strings.');
}

List<String> _readOptionalStringList(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value == null) {
    return const <String>[];
  }
  if (value is List && value.every((item) => item is String)) {
    return value.cast<String>();
  }
  throw ArgumentError('Expected `$key` to be a list of strings.');
}

String? _readOptionalString(Map<String, dynamic> source, String key) {
  final value = source[key];
  if (value == null) {
    return null;
  }
  if (value is String) {
    return value;
  }
  throw ArgumentError('Expected `$key` to be a string.');
}

dynamic _toNativeValue(dynamic value) {
  if (value is YamlMap) {
    return <String, dynamic>{
      for (final entry in value.entries)
        entry.key.toString(): _toNativeValue(entry.value),
    };
  }
  if (value is YamlList) {
    return value.map(_toNativeValue).toList(growable: false);
  }
  return value;
}

bool _listsEqual(List<String> left, List<String> right) {
  if (left.length != right.length) {
    return false;
  }
  for (var index = 0; index < left.length; index++) {
    if (left[index] != right[index]) {
      return false;
    }
  }
  return true;
}

String _shellQuote(String value) {
  return "'${value.replaceAll("'", r"'\''")}'";
}

String _toYaml(Object? value, {int indent = 0}) {
  final padding = '  ' * indent;
  if (value is Map) {
    if (value.isEmpty) {
      return '$padding{}\n';
    }
    final buffer = StringBuffer();
    for (final entry in value.entries) {
      final key = entry.key.toString();
      final child = entry.value;
      if (child is Map && child.isEmpty) {
        buffer.writeln('$padding$key: {}');
      } else if (child is List && child.isEmpty) {
        buffer.writeln('$padding$key: []');
      } else if (child is Map || child is List) {
        buffer.writeln('$padding$key:');
        buffer.write(_toYaml(child, indent: indent + 1));
      } else {
        buffer.writeln('$padding$key: ${_yamlScalar(child)}');
      }
    }
    return buffer.toString();
  }
  if (value is List) {
    if (value.isEmpty) {
      return '$padding[]\n';
    }
    final buffer = StringBuffer();
    for (final item in value) {
      if (item is Map && item.isEmpty) {
        buffer.writeln('$padding- {}');
      } else if (item is List && item.isEmpty) {
        buffer.writeln('$padding- []');
      } else if (item is Map || item is List) {
        buffer.writeln('$padding-');
        buffer.write(_toYaml(item, indent: indent + 1));
      } else {
        buffer.writeln('$padding- ${_yamlScalar(item)}');
      }
    }
    return buffer.toString();
  }
  return '$padding${_yamlScalar(value)}\n';
}

String _yamlScalar(Object? value) {
  if (value == null) {
    return 'null';
  }
  if (value is num || value is bool) {
    return value.toString();
  }
  final text = value.toString().replaceAll("'", "''");
  return "'$text'";
}

extension<T> on List<T> {
  T? get firstOrNull => isEmpty ? null : first;

  T? firstWhereOrNull(bool Function(T value) predicate) {
    for (final value in this) {
      if (predicate(value)) {
        return value;
      }
    }
    return null;
  }
}
