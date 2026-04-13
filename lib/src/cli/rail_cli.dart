import 'dart:io';

import 'package:path/path.dart' as p;

import '../runtime/harness_runner.dart';

class RailCli {
  RailCli({Directory? root}) : root = root ?? _resolveScriptRoot();

  final Directory root;

  Future<int> run(
    List<String> args, {
    StringSink? stdoutSink,
    StringSink? stderrSink,
  }) async {
    final out = stdoutSink ?? stdout;
    final err = stderrSink ?? stderr;
    final command = args.isEmpty ? 'help' : args.first;
    final runner = HarnessRunner(root);

    switch (command) {
      case 'init-request':
        final outputPath =
            _readOption(args.skip(1).toList(), '--output') ??
            '.harness/request.template.yaml';
        await runner.initRequestTemplate(outputPath);
        return 0;
      case 'compose-request':
        final tail = args.skip(1).toList();
        final goal = _readRequiredOption(tail, '--goal', usageSink: err);
        final outputPath = _readOption(tail, '--output');
        final taskType = _readRequiredOption(tail, '--task-type', usageSink: err);
        final feature = _readOption(tail, '--feature');
        final riskTolerance = _readOption(tail, '--risk-tolerance') ?? 'low';
        final priority = _readOption(tail, '--priority') ?? 'medium';
        final validationProfile =
            _readOption(tail, '--validation-profile') ?? 'standard';
        final constraints = _readMultiOption(tail, '--constraint');
        final definitionOfDone = _readMultiOption(tail, '--dod');
        final suspectedFiles = _readMultiOption(tail, '--suspected-file');
        final relatedFiles = _readMultiOption(tail, '--related-file');
        final validationRoots = _readMultiOption(tail, '--validation-root');
        final validationTargets = _readMultiOption(tail, '--validation-target');
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
        out.writeln(
          'Request composed at ${p.relative(composedRequest.file.path, from: runner.root.path)}',
        );
        out.writeln(
          'Inferred task_type=${composedRequest.request['task_type']} risk_tolerance=${composedRequest.request['risk_tolerance']} priority=${composedRequest.request['priority']}',
        );
        return 0;
      case 'validate-request':
        await runner.validateRequest(
          _readRequiredOption(args.skip(1).toList(), '--request', usageSink: err),
        );
        return 0;
      case 'validate-artifact':
        await runner.validateArtifact(
          filePath: _readRequiredOption(args.skip(1).toList(), '--file', usageSink: err),
          schemaName: _readRequiredOption(
            args.skip(1).toList(),
            '--schema',
            usageSink: err,
          ),
        );
        return 0;
      case 'run':
        final tail = args.skip(1).toList();
        final artifactPath = await runner.run(
          requestPath: _readRequiredOption(tail, '--request', usageSink: err),
          projectRoot: _readRequiredOption(tail, '--project-root', usageSink: err),
          taskId: _readOption(tail, '--task-id'),
          force: tail.contains('--force'),
        );
        out.writeln(
          'Harness artifacts created at ${p.relative(artifactPath, from: runner.root.path)}',
        );
        return 0;
      case 'execute':
        final tail = args.skip(1).toList();
        final result = await runner.execute(
          artifactPath: _readRequiredOption(tail, '--artifact', usageSink: err),
          projectRoot: _readOption(tail, '--project-root'),
          throughActor: _readOption(tail, '--through'),
        );
        out.writeln(result);
        return 0;
      case 'route-evaluation':
        final result = await runner.routeEvaluation(
          artifactPath: _readRequiredOption(
            args.skip(1).toList(),
            '--artifact',
            usageSink: err,
          ),
        );
        out.writeln(result);
        return 0;
      case 'help':
        _printUsage(out);
        return 0;
      default:
        writeUsage(err);
        return 64;
    }
  }

  void _printUsage(StringSink sink) {
    writeUsage(sink);
  }
}

void writeUsage(StringSink sink) {
  sink.writeln('Usage:');
  sink.writeln('  dart run bin/rail.dart init-request [--output <path>]');
  sink.writeln(
    '  dart run bin/rail.dart compose-request --goal <text> --task-type <bug_fix|feature_addition|safe_refactor|test_repair> [--feature <name>] [--suspected-file <path>] [--related-file <path>] [--validation-root <path>] [--validation-target <path>] [--constraint <text>] [--dod <text>] [--risk-tolerance <low|medium|high>] [--priority <low|medium|high>] [--validation-profile <standard|smoke>] [--output <path>]',
  );
  sink.writeln('  dart run bin/rail.dart validate-request --request <path>');
  sink.writeln(
    '  dart run bin/rail.dart validate-artifact --file <path> --schema <request|plan|context_pack|implementation_result|execution_report|evaluation_result|integration_result|quality_learning_candidate|hardening_candidate|approved_family_memory|learning_review_decision|hardening_review_decision|user_outcome_feedback|family_evidence_index|learning_review_queue|hardening_review_queue|quality_improvement_comparison>',
  );
  sink.writeln(
    '  dart run bin/rail.dart run --request <path> --project-root <path> [--task-id <id>] [--force]',
  );
  sink.writeln(
    '  dart run bin/rail.dart execute --artifact <path> [--project-root <path>] [--through <actor>]',
  );
  sink.writeln('  dart run bin/rail.dart route-evaluation --artifact <path>');
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
  required StringSink usageSink,
}) {
  final value = _readOption(args, name);
  if (value == null || value.isEmpty) {
    writeUsage(usageSink);
    throw ArgumentError('Missing required option: $name');
  }
  return value;
}
