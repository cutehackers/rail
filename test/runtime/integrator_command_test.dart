import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/runtime/harness_runner.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  final repoRoot = Directory.current;

  test(
    'integrate writes a schema-valid post-pass handoff after execute passes',
    () async {
      final root = repoRoot.path;
      final taskId = 'v2-integrator-smoke-test';
      final artifactPath = p.join('.harness', 'artifacts', taskId);
      final artifactDirectory = Directory(p.join(root, artifactPath));
      final schemaRunner = HarnessRunner(Directory(root));

      if (artifactDirectory.existsSync()) {
        artifactDirectory.deleteSync(recursive: true);
      }

      final runResult = await Process.run('dart', [
        'run',
        'bin/rail.dart',
        'run',
        '--request',
        p.join('test', 'fixtures', 'valid_request.yaml'),
        '--project-root',
        root,
        '--task-id',
        taskId,
      ], workingDirectory: root);
      expect(
        runResult.exitCode,
        0,
        reason: '${runResult.stdout}\n${runResult.stderr}',
      );

      final executeResult = await Process.run('dart', [
        'run',
        'bin/rail.dart',
        'execute',
        '--artifact',
        artifactPath,
      ], workingDirectory: root);
      expect(
        executeResult.exitCode,
        0,
        reason: '${executeResult.stdout}\n${executeResult.stderr}',
      );

      final integrateResult = await Process.run('dart', [
        'run',
        'bin/rail.dart',
        'integrate',
        '--artifact',
        artifactPath,
      ], workingDirectory: root);
      expect(
        integrateResult.exitCode,
        0,
        reason: '${integrateResult.stdout}\n${integrateResult.stderr}',
      );
      expect(
        integrateResult.stdout.toString(),
        contains('integration completed'),
      );

      await schemaRunner.validateArtifact(
        filePath: p.join(artifactPath, 'integration_result.yaml'),
        schemaName: 'integration_result',
      );

      final integrationMap = _loadYamlMap(
        File(p.join(root, artifactPath, 'integration_result.yaml')),
      );
      expect(
        integrationMap['evidence_quality'],
        anyOf('draft', 'adequate', 'high_confidence'),
      );
      expect(
        integrationMap['release_readiness'],
        anyOf('ready', 'conditional'),
      );
      expect(integrationMap['blocking_issues'], isA<List<dynamic>>());
      expect(integrationMap['blocking_issues'], isEmpty);

      final validationEntries =
          integrationMap['validation'] as List<dynamic>? ?? const <dynamic>[];
      expect(validationEntries, isNotEmpty);
      expect(
        validationEntries.whereType<Map>().isNotEmpty,
        isTrue,
        reason:
            'integration_result.validation should contain structured entries.',
      );
      final firstValidation = Map<String, dynamic>.from(
        validationEntries.whereType<Map>().first,
      );
      expect(
        firstValidation.keys,
        containsAll(<String>['check_name', 'status', 'evidence']),
      );
    },
    timeout: const Timeout(Duration(minutes: 4)),
  );

  test(
    'integrate fails closed when the integrator reports blockers with conditional readiness',
    () async {
      final root = repoRoot.path;
      final taskId =
          'v2-integrator-blocked-normalization-${DateTime.now().microsecondsSinceEpoch}';
      final artifactPath = p.join('.harness', 'artifacts', taskId);
      final artifactDirectory = Directory(p.join(root, artifactPath));

      addTearDown(() async {
        if (artifactDirectory.existsSync()) {
          await artifactDirectory.delete(recursive: true);
        }
      });

      final runResult = await Process.run('dart', [
        'run',
        'bin/rail.dart',
        'run',
        '--request',
        p.join('test', 'fixtures', 'valid_request.yaml'),
        '--project-root',
        root,
        '--task-id',
        taskId,
      ], workingDirectory: root);
      expect(
        runResult.exitCode,
        0,
        reason: '${runResult.stdout}\n${runResult.stderr}',
      );

      final executeResult = await Process.run('dart', [
        'run',
        'bin/rail.dart',
        'execute',
        '--artifact',
        artifactPath,
      ], workingDirectory: root);
      expect(
        executeResult.exitCode,
        0,
        reason: '${executeResult.stdout}\n${executeResult.stderr}',
      );

      final fakeBin = await Directory(
        p.join(root, '.dart_tool', 'fake-bin', taskId),
      ).create(recursive: true);
      addTearDown(() async {
        if (fakeBin.existsSync()) {
          await fakeBin.delete(recursive: true);
        }
      });

      final fakeCodex = File(p.join(fakeBin.path, 'codex'));
      await fakeCodex.writeAsString(_fakeCodexScript());
      await Process.run('chmod', [
        '+x',
        fakeCodex.path,
      ], workingDirectory: root);

      final integrateResult = await Process.run(
        'dart',
        ['run', 'bin/rail.dart', 'integrate', '--artifact', artifactPath],
        workingDirectory: root,
        environment: <String, String>{
          'PATH': '${fakeBin.path}:${Platform.environment['PATH'] ?? ''}',
        },
        includeParentEnvironment: true,
      );
      expect(
        integrateResult.exitCode,
        0,
        reason: '${integrateResult.stdout}\n${integrateResult.stderr}',
      );

      final integrationMap = _loadYamlMap(
        File(p.join(root, artifactPath, 'integration_result.yaml')),
      );
      expect(integrationMap['release_readiness'], 'blocked');
      expect(
        integrationMap['blocking_issues'],
        contains('Manual approval is still missing.'),
      );
    },
    timeout: const Timeout(Duration(minutes: 4)),
  );

  test('integrate refuses artifacts whose evaluator did not pass', () async {
    final tempRoot = await Directory(
      p.join(repoRoot.path, '.dart_tool', 'integrator-fixtures'),
    ).create(recursive: true);
    final copied = Directory(
      p.join(
        tempRoot.path,
        'split-task-${DateTime.now().microsecondsSinceEpoch}',
      ),
    );
    addTearDown(() async {
      if (copied.existsSync()) {
        await copied.delete(recursive: true);
      }
    });

    final source = Directory(
      p.join(repoRoot.path, 'test', 'fixtures', 'standard_route', 'split_task'),
    );
    await _copyDirectory(source, copied);

    final runner = HarnessRunner(repoRoot);
    expect(
      () => runner.integrate(artifactPath: copied.path),
      throwsA(
        isA<StateError>().having(
          (error) => error.message,
          'message',
          contains('requires evaluator decision `pass`'),
        ),
      ),
    );
  });
}

String _fakeCodexScript() {
  return '''#!/usr/bin/env python3
import json
import sys

output_path = None
for index, value in enumerate(sys.argv):
    if value == "--output-last-message" and index + 1 < len(sys.argv):
        output_path = sys.argv[index + 1]
        break

if output_path is None:
    raise SystemExit("missing --output-last-message")

payload = {
    "summary": "Conditional handoff with unresolved blocker.",
    "files_changed": [],
    "validation": [],
    "risks": [],
    "follow_up": [],
    "evidence_quality": "adequate",
    "release_readiness": "conditional",
    "blocking_issues": ["Manual approval is still missing."]
}

with open(output_path, "w", encoding="utf-8") as handle:
    json.dump(payload, handle)
''';
}

Future<void> _copyDirectory(Directory source, Directory destination) async {
  await destination.create(recursive: true);
  await for (final entity in source.list(recursive: true)) {
    final relativePath = p.relative(entity.path, from: source.path);
    final targetPath = p.join(destination.path, relativePath);
    if (entity is Directory) {
      await Directory(targetPath).create(recursive: true);
    } else if (entity is File) {
      await File(targetPath).parent.create(recursive: true);
      await entity.copy(targetPath);
    }
  }
}

Map<String, dynamic> _loadYamlMap(File file) {
  final parsed = loadYaml(file.readAsStringSync());
  if (parsed is YamlMap) {
    return <String, dynamic>{
      for (final entry in parsed.entries)
        entry.key.toString(): _toNativeValue(entry.value),
    };
  }
  throw StateError('Expected a YAML map in ${file.path}.');
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
