import 'package:rail/src/runtime/route_decision.dart';
import 'package:test/test.dart';

void main() {
  test('route evaluation maps validation evidence to tighten_validation', () {
    final action = routeFromEvaluationResult(
      decision: 'revise',
      reasonCodes: const ['validation_evidence_missing'],
      nextAction: null,
    );

    expect(action, 'tighten_validation');
  });
}
