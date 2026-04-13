String renderTerminalSummary({
  required String status,
  required String action,
  required List<String> reasons,
}) {
  final reasonList = reasons.isEmpty ? 'none' : reasons.join(', ');
  return [
    '# Terminal Outcome',
    '',
    '- status: `$status`',
    '- action: `$action`',
    '- reason_codes: `$reasonList`',
    '',
    '## Summary',
    '',
    _terminalOutcomeSummary(status),
  ].join('\n');
}

String _terminalOutcomeSummary(String status) {
  return switch (status) {
    'passed' =>
      'The supervisor accepted the run and no further evolution step is required.',
    'blocked_environment' =>
      'The supervisor was blocked by environment or tooling issues that prevented credible validation. More code changes would not have fixed this run.',
    'split_required' =>
      'The supervisor stopped because the request is too broad or crosses task boundaries and should be decomposed before continuing.',
    'evolution_exhausted' =>
      'The supervisor stopped because it ran out of bounded evolution actions without finding a credible path forward.',
    'revise_exhausted' =>
      'The supervisor stopped because implementation retries were exhausted without closing the gap.',
    'rejected' =>
      'The evaluator rejected the run because the result violated constraints or carried unacceptable risk.',
    _ => 'The harness recorded a terminal state, but this outcome needs manual review.',
  };
}
