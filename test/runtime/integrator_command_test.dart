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
