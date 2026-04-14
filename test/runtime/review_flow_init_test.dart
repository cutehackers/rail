import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/runtime/harness_runner.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  final repoRoot = Directory.current;

  test('initUserOutcomeFeedback writes a schema-valid draft from an artifact', () async {
    final tempRoot = await Directory(
      p.join(repoRoot.path, '.dart_tool', 'review-flow-init'),
    ).create(recursive: true);
    final copied = Directory(
      p.join(tempRoot.path, 'artifact-${DateTime.now().microsecondsSinceEpoch}'),
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
        'tighten_validation',
      ),
    );
    await _copyDirectory(source, copied);

    final runner = HarnessRunner(repoRoot);
    final outputPath = await runner.initUserOutcomeFeedback(
      artifactPath: copied.path,
    );
    addTearDown(() async {
      final generated = File(outputPath);
      if (generated.existsSync()) {
        await generated.delete();
      }
    });

    expect(outputPath, contains('.harness/learning/feedback/'));
    await runner.validateArtifact(
      filePath: p.relative(outputPath, from: repoRoot.path),
      schemaName: 'user_outcome_feedback',
    );

    final map = _loadYamlMap(File(outputPath));
    final identity = Map<String, dynamic>.from(
      map['originating_run_artifact_identity'] as Map,
    );
    expect(identity['run_ref'], p.relative(copied.path, from: repoRoot.path));
    expect(map['feedback_classification'], 'unresolved');
  });

  test('initLearningReview writes a schema-valid draft from a quality candidate', () async {
    final candidateFile = await _writeQualityCandidateFixture(repoRoot);
    addTearDown(() async {
      if (candidateFile.parent.parent.parent.existsSync()) {
        await candidateFile.parent.parent.parent.delete(recursive: true);
      }
    });

    final runner = HarnessRunner(repoRoot);
    final candidateRef = p.relative(candidateFile.path, from: repoRoot.path);
    final outputPath = await runner.initLearningReview(candidateRef: candidateRef);
    addTearDown(() async {
      final generated = File(outputPath);
      if (generated.existsSync()) {
        await generated.delete();
      }
    });

    expect(outputPath, contains('.harness/learning/reviews/'));
    await runner.validateArtifact(
      filePath: p.relative(outputPath, from: repoRoot.path),
      schemaName: 'learning_review_decision',
    );

    final map = _loadYamlMap(File(outputPath));
    expect(map['candidate_ref'], candidateRef);
    expect(map['reviewer_decision'], 'hold');
  });

  test('initHardeningReview writes a schema-valid draft from a hardening candidate', () async {
    final candidateFile = await _writeHardeningCandidateFixture(repoRoot);
    addTearDown(() async {
      if (candidateFile.parent.parent.parent.existsSync()) {
        await candidateFile.parent.parent.parent.delete(recursive: true);
      }
    });

    final runner = HarnessRunner(repoRoot);
    final candidateRef = p.relative(candidateFile.path, from: repoRoot.path);
    final outputPath = await runner.initHardeningReview(candidateRef: candidateRef);
    addTearDown(() async {
      final generated = File(outputPath);
      if (generated.existsSync()) {
        await generated.delete();
      }
    });

    expect(outputPath, contains('.harness/learning/hardening-reviews/'));
    await runner.validateArtifact(
      filePath: p.relative(outputPath, from: repoRoot.path),
      schemaName: 'hardening_review_decision',
    );

    final map = _loadYamlMap(File(outputPath));
    expect(map['hardening_candidate_ref'], candidateRef);
    expect(map['reviewer_decision'], 'hold');
  });
}

Future<File> _writeQualityCandidateFixture(Directory repoRoot) async {
  final artifactDir = Directory(
    p.join(
      repoRoot.path,
      '.dart_tool',
      'review-flow-init',
      'quality-artifact-${DateTime.now().microsecondsSinceEpoch}',
      'quality_learning_candidates',
    ),
  );
  await artifactDir.create(recursive: true);
  final candidateFile = File(p.join(artifactDir.path, 'candidate.yaml'));
  final runRef = p.relative(artifactDir.parent.path, from: repoRoot.path);
  final candidate = {
    'originating_run_artifact_identity': {
      'run_ref': runRef,
      'artifact_ref': p.join(runRef, 'evaluation_result.yaml'),
    },
    'candidate_identifier': 'quality/candidate@1',
    'task_family': 'bug_fix',
    'task_family_source': 'task_type',
    'quality_outcome_summary': 'Validation evidence suggests the fix should be reusable.',
    'user_outcome_signal': {
      'status': 'provisional',
      'summary': 'No accepted user confirmation has been reviewed yet.',
    },
    'effective_context_signal': {
      'summary': 'Context stayed stable.',
      'helped_context_factors': <String>[],
      'failed_context_factors': <String>[],
      'neutral_context_factors': <String>['Baseline repo context was sufficient.'],
      'evidence_refs': <String>[p.join(runRef, 'execution_report.yaml')],
      'context_factor_refs': <String>['state.contextRefreshCount'],
    },
    'effective_validation_signal': {
      'summary': 'Validation evidence is supportive.',
      'materially_supporting_validation_evidence': <String>['Static analysis passed.'],
      'failed_to_support_validation_evidence': <String>[],
      'contradicting_validation_evidence': <String>[],
      'evidence_refs': <String>[p.join(runRef, 'execution_report.yaml')],
      'validation_step_refs': <String>['execution_report.analyze'],
    },
    'evaluator_support_signal': {
      'quality_confidence': 0.6,
      'final_reason_codes': <String>['requirements_coverage_resolved'],
      'validation_sufficiency_assessment': 'sufficient',
      'terminal_outcome_class': 'passed',
      'supporting_evaluator_notes': <String>['Evaluator pass was bounded and explicit.'],
    },
    'candidate_claim': 'This change looks reusable for the same task family.',
    'supporting_evidence_refs': <String>[p.join(runRef, 'evaluation_result.yaml')],
    'guardrail_cost': {
      'summary': 'No excessive intervention cost was recorded.',
      'intervention_count': 1,
      'intervention_refs': <String>[p.join(runRef, 'state.json')],
    },
    'runtime_recommendation': 'hold',
  };
  await candidateFile.writeAsString(_toYaml(candidate));
  return candidateFile;
}

Future<File> _writeHardeningCandidateFixture(Directory repoRoot) async {
  final artifactDir = Directory(
    p.join(
      repoRoot.path,
      '.dart_tool',
      'review-flow-init',
      'hardening-artifact-${DateTime.now().microsecondsSinceEpoch}',
      'hardening_candidates',
    ),
  );
  await artifactDir.create(recursive: true);
  final candidateFile = File(p.join(artifactDir.path, 'candidate.yaml'));
  final runRef = p.relative(artifactDir.parent.path, from: repoRoot.path);
  final candidate = {
    'originating_run_artifact_identity': {
      'run_ref': runRef,
      'artifact_ref': p.join(runRef, 'evaluation_result.yaml'),
    },
    'candidate_identifier': 'hardening/candidate@1',
    'task_family': 'bug_fix',
    'task_family_source': 'task_type',
    'policy_affecting_observation': 'A policy-sensitive regression note needs human hardening review.',
    'why_it_must_not_become_reusable_family_memory':
        'This observation would alter policy rather than reusable execution guidance.',
    'supporting_evidence_refs': <String>[p.join(runRef, 'evaluation_result.yaml')],
    'hardening_recommendation': 'hold',
  };
  await candidateFile.writeAsString(_toYaml(candidate));
  return candidateFile;
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

String _toYaml(Object? value) => const JsonEncoder.withIndent('  ').convert(value);
