import 'package:rail/src/reporting/terminal_summary.dart';
import 'package:test/test.dart';

void main() {
  test('terminal summary explains blocked_environment state', () {
    final summary = renderTerminalSummary(
      status: 'blocked_environment',
      action: 'block_environment',
      reasons: const ['environment_permission_denied'],
    );

    expect(summary, contains('blocked by environment'));
    expect(summary, contains('block_environment'));
    expect(summary, contains('environment_permission_denied'));
  });
}
