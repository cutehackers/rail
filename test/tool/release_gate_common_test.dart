import 'dart:io';

import 'package:test/test.dart';

void main() {
  final root = Directory.current.path;

  test('rejects smoke task ids with traversal segments', () async {
    final result = await Process.run('bash', [
      '-lc',
      'source tool/release_gate_common.sh && rail_validate_smoke_task_id "../outside"',
    ], workingDirectory: root);

    expect(result.exitCode, isNonZero);
    expect(result.stderr.toString(), contains('invalid smoke task id'));
  });

  test('accepts smoke task ids made of safe path tokens', () async {
    final result = await Process.run('bash', [
      '-lc',
      'source tool/release_gate_common.sh && rail_validate_smoke_task_id "v2-integrator-smoke-ci_01"',
    ], workingDirectory: root);

    expect(result.exitCode, 0, reason: '${result.stdout}\n${result.stderr}');
    expect(result.stdout.toString().trim(), 'v2-integrator-smoke-ci_01');
  });
}
