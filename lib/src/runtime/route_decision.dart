String? routeFromEvaluationResult({
  required String decision,
  required List<String> reasonCodes,
  required String? nextAction,
}) {
  if (decision == 'pass') {
    return 'pass';
  }

  if (_hasEnvironmentFailure(reasonCodes)) {
    return 'block_environment';
  }
  if (_hasScopeFailure(reasonCodes)) {
    return 'split_task';
  }
  if (_hasContextFailure(reasonCodes)) {
    return 'rebuild_context';
  }
  if (_hasValidationScopeFailure(reasonCodes)) {
    return 'tighten_validation';
  }
  if (_hasValidationEvidenceFailure(reasonCodes) ||
      _hasValidationRequirementFailure(reasonCodes) ||
      _hasRequirementsCoverageFailure(reasonCodes) ||
      _hasRequirementsBehaviorFailure(reasonCodes) ||
      _hasValidationFailure(reasonCodes) ||
      _hasRequirementsFailure(reasonCodes) ||
      _hasImplementationFailure(reasonCodes) ||
      _hasArchitectureFailure(reasonCodes)) {
    return 'revise_generator';
  }

  return nextAction;
}

bool _hasEnvironmentFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) {
    return code.startsWith('environment_') ||
        code.contains('permission_error') ||
        code.contains('sandbox') ||
        code.contains('tooling_unavailable') ||
        code.contains('sdk_cache');
  });
}

bool _hasScopeFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('scope_'));
}

bool _hasContextFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('context_'));
}

bool _hasValidationScopeFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) {
    return code.startsWith('validation_scope_') ||
        code.startsWith('validation_target_') ||
        code.startsWith('validation_mismatch_');
  });
}

bool _hasValidationEvidenceFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('validation_evidence_'));
}

bool _hasValidationRequirementFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('validation_requirement_'));
}

bool _hasRequirementsCoverageFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('requirements_coverage_'));
}

bool _hasRequirementsBehaviorFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('requirements_behavior_'));
}

bool _hasValidationFailure(List<String> reasonCodes) {
  return reasonCodes.any(
    (code) =>
        code.startsWith('validation_') &&
        !code.startsWith('validation_scope_') &&
        !code.startsWith('validation_target_') &&
        !code.startsWith('validation_mismatch_') &&
        !code.startsWith('validation_evidence_') &&
        !code.startsWith('validation_requirement_'),
  );
}

bool _hasRequirementsFailure(List<String> reasonCodes) {
  return reasonCodes.any(
    (code) =>
        code.startsWith('requirements_') &&
        !code.startsWith('requirements_coverage_') &&
        !code.startsWith('requirements_behavior_'),
  );
}

bool _hasImplementationFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('implementation_'));
}

bool _hasArchitectureFailure(List<String> reasonCodes) {
  return reasonCodes.any((code) => code.startsWith('architecture_'));
}
