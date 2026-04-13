import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  test('smoke execute produces a schema-valid v1 execution report', () async {
    final root = Directory.current.path;
    final taskId = 'v1-contract-smoke-test';
    final artifactPath = p.join('.harness', 'artifacts', taskId);
    final artifactDirectory = Directory(p.join(root, artifactPath));

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

    expect(runResult.exitCode, 0, reason: '${runResult.stdout}\n${runResult.stderr}');

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
  });
}
