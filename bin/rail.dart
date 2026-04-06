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
      final composedRequest = await runner.composeRequest(
        goal: goal,
        outputPath: outputPath,
        taskType: taskType,
        feature: feature,
        riskTolerance: riskTolerance,
        priority: priority,
        constraints: constraints,
        definitionOfDone: definitionOfDone,
        suspectedFiles: suspectedFiles,
        relatedFiles: relatedFiles,
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
    '  dart run bin/rail.dart compose-request --goal <text> --task-type <bug_fix|feature_addition|safe_refactor|test_repair> [--feature <name>] [--suspected-file <path>] [--related-file <path>] [--constraint <text>] [--dod <text>] [--risk-tolerance <low|medium|high>] [--priority <low|medium|high>] [--output <path>]',
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
    required List<String> constraints,
    required List<String> definitionOfDone,
    required List<String> suspectedFiles,
    required List<String> relatedFiles,
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
      },
      'constraints': normalizedConstraints,
      'definition_of_done': normalizedDefinitionOfDone,
      'priority': priority,
      'risk_tolerance': riskTolerance,
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

    final analyzePackages = _inferPackageRoots(fileHints);
    final testTargets = testRules.inferTargets(
      projectRoot: projectDirectory.path,
      fileHints: fileHints,
      featureName: userRequest.context.feature,
    );
    final executionPlan = ExecutionPlan(
      formatCommand: fileHints.isEmpty
          ? null
          : executionPolicy.formatCommand.replaceAll(
              '{files}',
              fileHints.map(_shellQuote).join(' '),
            ),
      analyzeCommands: analyzePackages.isEmpty
          ? [
              'cd ${_shellQuote(projectDirectory.path)} && ${executionPolicy.workspaceAnalyzeCommand}',
            ]
          : analyzePackages
              .map(
                (packageRoot) =>
                    'cd ${_shellQuote(p.join(projectDirectory.path, packageRoot))} && ${executionPolicy.packageAnalyzeCommand}',
              )
              .toList(growable: false),
      testCommands: testTargets.isEmpty
          ? [
              'cd ${_shellQuote(projectDirectory.path)} && ${executionPolicy.workspaceTestCommand}',
            ]
          : _groupTargetsByPackage(testTargets).entries
              .map(
                (entry) =>
                    'cd ${_shellQuote(p.join(projectDirectory.path, entry.key))} && ${executionPolicy.packageTestCommand.replaceAll('{targets}', entry.value.map(_shellQuote).join(' '))}',
              )
              .toList(growable: false),
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
            'lastDecision': null,
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
    final projectDirectory = _resolveProjectDirectory(
      projectRoot ?? workflow.projectRoot,
    );
    final _ = ExecutionPlan.fromJson(
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

      final prompt = _buildCodexExecutionPrompt(
        actorName: actorName,
        actorBriefPath: actorBriefPath,
        artifactDirectory: artifactDirectory.path,
        projectRoot: projectDirectory.path,
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
        workingDirectory: projectDirectory.path,
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
        final responseObject = _decodeStructuredResponse(
          filePath: logPath,
          fallbackText: result.stdout,
        );
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

      if (stopActor != null && actorName == stopActor) {
        break;
      }
      if (_shouldTerminate(currentState)) {
        break;
      }
    }

    return 'Harness execution updated at ${p.relative(artifactDirectory.path, from: root.path)} (status=${currentState.status}, currentActor=${currentState.currentActor ?? 'none'})';
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
      ..writeln('- Retry budget: `${workflow.generatorRetryBudget}`')
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
          'next_action': <String>[],
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
      final decision = _readString(
        _asMap(_loadYamlValue(File(evaluationPath)), context: evaluationPath),
        'decision',
      );
      if (decision == 'pass') {
        final integratorIndex = workflow.actors.indexOf('integrator');
        if (integratorIndex != -1 && !completedActors.contains('integrator')) {
          return state.copyWith(
            status: 'awaiting_integrator',
            currentActor: 'integrator',
            completedActors: completedActors,
            lastDecision: decision,
          );
        }
          return state.copyWith(
            status: 'passed',
            clearCurrentActor: true,
            completedActors: completedActors,
            lastDecision: decision,
          );
      }
      if (decision == 'reject') {
        return state.copyWith(
          status: 'rejected',
          clearCurrentActor: true,
          completedActors: completedActors,
          lastDecision: decision,
        );
      }
      final retriesRemaining = state.generatorRetriesRemaining - 1;
      if (retriesRemaining < 0 || !workflow.actors.contains('generator')) {
        return state.copyWith(
          status: 'revise_exhausted',
          clearCurrentActor: true,
          completedActors: completedActors,
          generatorRetriesRemaining: retriesRemaining,
          lastDecision: decision,
        );
      }
      return state.copyWith(
        status: 'revising',
        currentActor: 'generator',
        completedActors: completedActors,
        generatorRetriesRemaining: retriesRemaining,
        lastDecision: decision,
      );
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
        state.status == 'revise_exhausted';
  }

  String _buildCodexExecutionPrompt({
    required String actorName,
    required String actorBriefPath,
    required String artifactDirectory,
    required String projectRoot,
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
        'Run the planned commands if needed, then update only the execution report and any logs inside the artifact directory.',
      'generator' =>
        'You may modify repository source files if needed for the task, and you must also update the implementation result artifact.',
      _ => 'Stay within the repository and artifact scope described in the brief.',
    };
    return '''
You are executing the `$actorName` actor for a rail harness workflow.

Target project root: `$projectRoot`
Harness artifact root: `$artifactDirectory`

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
    final priority = map['priority']?.toString();

    const allowedTaskTypes = {
      'bug_fix',
      'feature_addition',
      'safe_refactor',
      'test_repair',
    };
    const allowedRiskTolerance = {'low', 'medium', 'high'};

    if (!allowedTaskTypes.contains(taskType)) {
      throw ArgumentError('Unsupported task_type: $taskType');
    }
    if (!allowedRiskTolerance.contains(riskTolerance)) {
      throw ArgumentError('Unsupported risk_tolerance: $riskTolerance');
    }

    return UserRequest(
      taskType: taskType,
      goal: goal,
      context: RequestContext.fromMap(contextMap),
      constraints: constraints,
      definitionOfDone: definitionOfDone,
      riskTolerance: riskTolerance,
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
  final String? priority;
  final String requestPath;
}

class RequestContext {
  RequestContext({
    required this.relatedFiles,
    required this.suspectedFiles,
    this.feature,
  });

  factory RequestContext.fromMap(Map<String, dynamic> map) {
    return RequestContext(
      feature: map['feature']?.toString(),
      suspectedFiles: _readOptionalStringList(map, 'suspected_files'),
      relatedFiles: _readOptionalStringList(map, 'related_files'),
    );
  }

  final String? feature;
  final List<String> suspectedFiles;
  final List<String> relatedFiles;
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
    required this.passIf,
    required this.reviseIf,
    required this.rejectIf,
  });

  factory Policy.fromMap(Map<String, dynamic> map) {
    final retryRules = _readMap(map, 'retry_rules');
    final budgets = <String, int>{};
    for (final entry in retryRules.entries) {
      budgets[entry.key] = _readInt(
        _readMap(retryRules, entry.key),
        'max_generator_retry',
      );
    }
    return Policy(
      retryBudgets: budgets,
      passIf: _readStringList(map, 'pass_if'),
      reviseIf: _readStringList(map, 'revise_if'),
      rejectIf: _readStringList(map, 'reject_if'),
    );
  }

  final Map<String, int> retryBudgets;
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
    required this.packageTestCommand,
    required this.workspaceTestCommand,
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
      packageTestCommand: _readString(
        _readMap(map, 'tests'),
        'package_command',
      ),
      workspaceTestCommand: _readString(
        _readMap(map, 'tests'),
        'workspace_fallback',
      ),
      createPlaceholders: _readBool(runtime, 'create_placeholders'),
      createActorBriefs: _readBool(runtime, 'create_actor_briefs'),
      persistJsonSnapshots: _readBool(runtime, 'persist_json_snapshots'),
    );
  }

  final String artifactRoot;
  final String formatCommand;
  final String packageAnalyzeCommand;
  final String workspaceAnalyzeCommand;
  final String packageTestCommand;
  final String workspaceTestCommand;
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
    required this.lastDecision,
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
      lastDecision: map['lastDecision']?.toString(),
    );
  }

  final String taskId;
  final String status;
  final String? currentActor;
  final List<String> completedActors;
  final int generatorRetriesRemaining;
  final String? lastDecision;

  HarnessState copyWith({
    String? status,
    String? currentActor,
    bool clearCurrentActor = false,
    List<String>? completedActors,
    int? generatorRetriesRemaining,
    String? lastDecision,
  }) {
    return HarnessState(
      taskId: taskId,
      status: status ?? this.status,
      currentActor: clearCurrentActor ? null : (currentActor ?? this.currentActor),
      completedActors: completedActors ?? this.completedActors,
      generatorRetriesRemaining:
          generatorRetriesRemaining ?? this.generatorRetriesRemaining,
      lastDecision: lastDecision ?? this.lastDecision,
    );
  }

  Map<String, Object?> toJson() {
    return {
      'taskId': taskId,
      'status': status,
      'currentActor': currentActor,
      'completedActors': completedActors,
      'generatorRetriesRemaining': generatorRetriesRemaining,
      'lastDecision': lastDecision,
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
