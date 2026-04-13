import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/runtime/harness_runner.dart';
import 'package:test/test.dart';

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

      final runResult = await Process.run(
        'dart',
        [
          'run',
          'bin/rail.dart',
          'run',
          '--request',
          p.join('test', 'fixtures', 'valid_request.yaml'),
          '--project-root',
          root,
          '--task-id',
          taskId,
        ],
        workingDirectory: root,
      );
      expect(
        runResult.exitCode,
        0,
        reason: '${runResult.stdout}\n${runResult.stderr}',
      );

      final executeResult = await Process.run(
        'dart',
        [
          'run',
          'bin/rail.dart',
          'execute',
          '--artifact',
          artifactPath,
        ],
        workingDirectory: root,
      );
      expect(
        executeResult.exitCode,
        0,
        reason: '${executeResult.stdout}\n${executeResult.stderr}',
      );

      final integrateResult = await Process.run(
        'dart',
        [
          'run',
          'bin/rail.dart',
          'integrate',
          '--artifact',
          artifactPath,
        ],
        workingDirectory: root,
      );
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
    },
    timeout: const Timeout(Duration(minutes: 2)),
  );

  test('integrate refuses artifacts whose evaluator did not pass', () async {
    final tempRoot = await Directory(
      p.join(repoRoot.path, '.dart_tool', 'integrator-fixtures'),
    ).create(recursive: true);
    final copied = Directory(
      p.join(tempRoot.path, 'split-task-${DateTime.now().microsecondsSinceEpoch}'),
    );
    addTearDown(() async {
      if (copied.existsSync()) {
        await copied.delete(recursive: true);
      }
    });

    final source = Directory(
      p.join(
        repoRoot.path,
        'test',
        'fixtures',
        'standard_route',
        'split_task',
      ),
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
