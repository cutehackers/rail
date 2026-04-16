import 'dart:io';

import 'package:test/test.dart';

void main() {
  final root = Directory.current.path;
  final v1ReleaseGate = File('$root/tool/v1_release_gate.sh');
  final v2ReleaseGate = File('$root/tool/v2_release_gate.sh');
  final v1Workflow = File('$root/.github/workflows/v1-release-gate.yml');

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

  test('v1 release gate script is Go-first and smokes the built rail binary', () async {
    final script = await v1ReleaseGate.readAsString();

    expect(script, contains('go test ./...'));
    expect(script, contains('go build -o build/rail ./cmd/rail'));
    expect(script, contains('./build/rail run'));
    expect(script, contains('./build/rail execute'));
    expect(script, isNot(contains('dart pub get')));
    expect(script, isNot(contains('dart analyze')));
    expect(script, isNot(contains('dart test')));
    expect(script, isNot(contains('dart compile exe')));
    expect(script, isNot(contains('dart run bin/rail.dart')));
  });

  test('v1 release workflow provisions Go instead of Dart', () async {
    final workflow = await v1Workflow.readAsString();

    expect(workflow, contains('actions/setup-go@v5'));
    expect(workflow, contains('go-version-file: go.mod'));
    expect(workflow, isNot(contains('dart-lang/setup-dart@v1')));
  });

  test('v2 release gate documents the missing Go parity commands as blockers', () async {
    final script = await v2ReleaseGate.readAsString();

    expect(script, contains('go test ./...'));
    expect(script, contains('go build -o build/rail ./cmd/rail'));
    expect(script, contains('./build/rail run'));
    expect(script, contains('./build/rail execute'));
    expect(script, contains('integrate'));
    expect(script, contains('validate-artifact'));
    expect(script, contains('verify-learning-state'));
    expect(script, contains('Go CLI parity is incomplete for v2 release gate'));
    expect(script, isNot(contains('dart compile exe')));
    expect(script, isNot(contains('dart run bin/rail.dart run')));
    expect(script, isNot(contains('dart run bin/rail.dart execute')));
  });
}
