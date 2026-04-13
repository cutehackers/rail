import 'package:rail/src/cli/rail_cli.dart';
import 'package:test/test.dart';

void main() {
  test('RailCli renders usage for empty args', () async {
    final stdoutBuffer = StringBuffer();
    final stderrBuffer = StringBuffer();

    final exitCode = await RailCli().run(
      const [],
      stdoutSink: stdoutBuffer,
      stderrSink: stderrBuffer,
    );

    expect(exitCode, 0);
    expect(stdoutBuffer.toString(), contains('route-evaluation'));
    expect(stdoutBuffer.toString(), contains('integrate'));
    expect(stdoutBuffer.toString(), isNot(contains('apply-learning-review')));
    expect(
      stdoutBuffer.toString(),
      isNot(contains('apply-user-outcome-feedback')),
    );
    expect(stderrBuffer.toString(), isEmpty);
  });

  test('RailCli rejects deferred v2 commands from the v1 surface', () async {
    final stdoutBuffer = StringBuffer();
    final stderrBuffer = StringBuffer();

    final exitCode = await RailCli().run(
      const ['apply-learning-review'],
      stdoutSink: stdoutBuffer,
      stderrSink: stderrBuffer,
    );

    expect(exitCode, 64);
    expect(stdoutBuffer.toString(), isEmpty);
    expect(stderrBuffer.toString(), contains('Usage:'));
    expect(stderrBuffer.toString(), isNot(contains('apply-learning-review')));
  });
}
