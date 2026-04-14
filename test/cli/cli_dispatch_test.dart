import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/cli/rail_cli.dart';
import 'package:rail/src/runtime/harness_runner.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  final repoRoot = Directory.current;

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
    expect(stdoutBuffer.toString(), contains('init-user-outcome-feedback'));
    expect(stdoutBuffer.toString(), contains('init-learning-review'));
    expect(stdoutBuffer.toString(), contains('init-hardening-review'));
    expect(stdoutBuffer.toString(), contains('apply-user-outcome-feedback'));
    expect(stdoutBuffer.toString(), contains('apply-learning-review'));
    expect(stdoutBuffer.toString(), contains('apply-hardening-review'));
    expect(stdoutBuffer.toString(), contains('verify-learning-state'));
    expect(stderrBuffer.toString(), isEmpty);
  });

  test(
    'RailCli routes init-user-outcome-feedback once v2 surface is enabled',
    () async {
      final tempRoot = await Directory(
        p.join(repoRoot.path, '.dart_tool', 'cli-dispatch-test'),
      ).create(recursive: true);
      final copied = Directory(
        p.join(
          tempRoot.path,
          'artifact-${DateTime.now().microsecondsSinceEpoch}',
        ),
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

      final stdoutBuffer = StringBuffer();
      final stderrBuffer = StringBuffer();

      final exitCode = await RailCli(root: repoRoot).run(
        ['init-user-outcome-feedback', '--artifact', copied.path],
        stdoutSink: stdoutBuffer,
        stderrSink: stderrBuffer,
      );
      final outputPath = stdoutBuffer.toString().trim();
      addTearDown(() async {
        if (outputPath.isNotEmpty) {
          final generated = File(p.join(repoRoot.path, outputPath));
          if (generated.existsSync()) {
            await generated.delete();
          }
        }
      });

      expect(exitCode, 0);
      expect(stdoutBuffer.toString(), contains('.harness/learning/feedback/'));
      expect(stderrBuffer.toString(), isNot(contains('Usage:')));
    },
  );

  test(
    'RailCli routes verify-learning-state through the read-only verifier',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'verify-learning-state',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      await _seedHealthyLearningState(tempRoot);

      final stdoutBuffer = StringBuffer();
      final stderrBuffer = StringBuffer();

      final exitCode = await RailCli(root: tempRoot).run(
        const ['verify-learning-state'],
        stdoutSink: stdoutBuffer,
        stderrSink: stderrBuffer,
      );

      expect(exitCode, 0);
      expect(
        stdoutBuffer.toString(),
        contains('Learning state verification passed'),
      );
      expect(stderrBuffer.toString(), isEmpty);
    },
  );
}

Future<Directory> _createHarnessRoot(Directory repoRoot, String name) async {
  final root = Directory(
    p.join(
      repoRoot.path,
      '.dart_tool',
      'cli-dispatch-test',
      '$name-${DateTime.now().microsecondsSinceEpoch}',
    ),
  );
  await root.create(recursive: true);
  await _copyDirectory(
    Directory(p.join(repoRoot.path, '.harness', 'templates')),
    Directory(p.join(root.path, '.harness', 'templates')),
  );
  await _copyDirectory(
    Directory(p.join(repoRoot.path, '.harness', 'supervisor')),
    Directory(p.join(root.path, '.harness', 'supervisor')),
  );
  return root;
}

Future<void> _seedHealthyLearningState(Directory root) async {
  final runner = HarnessRunner(root);
  final candidateFile = await _writeQualityCandidateFixture(root);
  final candidateRef = p.relative(candidateFile.path, from: root.path);
  final feedbackFile = await _writeUserOutcomeFeedbackFixture(
    root,
    candidateRef: candidateRef,
  );
  final feedbackRef = p.relative(feedbackFile.path, from: root.path);
  final decisionFile = await _writeLearningReviewFixture(
    root,
    candidateRef: candidateRef,
    reviewerDecision: 'promote',
    consideredUserOutcomeFeedbackRefs: [feedbackRef],
    resultingApprovedMemoryRef: '.harness/learning/approved/bug_fix.yaml',
    guardrailAssessment: 'intervention_cost_does_not_explain_improvement',
    decisionReason: 'Promote into canonical approved memory for verification.',
  );
  await runner.applyLearningReview(
    decisionPath: p.relative(decisionFile.path, from: root.path),
  );
}

Future<File> _writeQualityCandidateFixture(
  Directory root, {
  String runName = 'cli-verify-run',
}) async {
  final artifactDir = Directory(
    p.join(
      root.path,
      '.harness',
      'artifacts',
      runName,
      'quality_learning_candidates',
    ),
  );
  await artifactDir.create(recursive: true);
  final candidateFile = File(p.join(artifactDir.path, 'candidate.yaml'));
  final runRef = p.relative(artifactDir.parent.path, from: root.path);
  final candidate = {
    'originating_run_artifact_identity': {
      'run_ref': runRef,
      'artifact_ref': p.join(runRef, 'evaluation_result.yaml'),
    },
    'candidate_identifier': 'quality/cli-verify@1',
    'task_family': 'bug_fix',
    'task_family_source': 'task_type',
    'quality_outcome_summary': 'Reusable same-family improvement candidate.',
    'user_outcome_signal': {
      'status': 'provisional',
      'summary': 'Awaiting explicit confirmation.',
    },
    'effective_context_signal': {
      'summary': 'Context remained stable.',
      'helped_context_factors': <String>[],
      'failed_context_factors': <String>[],
      'neutral_context_factors': <String>['Baseline context was enough.'],
      'evidence_refs': <String>[p.join(runRef, 'execution_report.yaml')],
      'context_factor_refs': <String>['state.contextRefreshCount'],
    },
    'effective_validation_signal': {
      'summary': 'Validation remained supportive.',
      'materially_supporting_validation_evidence': <String>['Analyze passed.'],
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
      'supporting_evaluator_notes': <String>['Bounded pass'],
    },
    'candidate_claim': 'Promote only after explicit learning review.',
    'supporting_evidence_refs': <String>[
      p.join(runRef, 'evaluation_result.yaml'),
    ],
    'guardrail_cost': {
      'summary': 'Intervention cost stayed low.',
      'intervention_count': 1,
      'intervention_refs': <String>[p.join(runRef, 'state.json')],
    },
    'runtime_recommendation': 'hold',
  };
  await candidateFile.writeAsString(_toYaml(candidate));
  return candidateFile;
}

Future<File> _writeUserOutcomeFeedbackFixture(
  Directory root, {
  required String candidateRef,
}) async {
  final feedbackDir = Directory(
    p.join(root.path, '.harness', 'learning', 'feedback'),
  );
  await feedbackDir.create(recursive: true);
  final feedbackFile = File(p.join(feedbackDir.path, 'feedback.yaml'));
  final candidate = _loadYamlMap(_resolve(root, candidateRef));
  final identity = Map<String, dynamic>.from(
    candidate['originating_run_artifact_identity'] as Map,
  );
  final feedback = {
    'originating_run_artifact_identity': identity,
    'candidate_ref_hint': candidateRef,
    'task_family': candidate['task_family'],
    'task_family_source': candidate['task_family_source'],
    'feedback_classification': 'accepted',
    'feedback_summary': 'The user confirmed the fix works in the target flow.',
    'submitted_at': DateTime.now().toUtc().toIso8601String(),
    'evidence_refs': {
      'run_refs': <String>[identity['artifact_ref']],
      'follow_up_refs': <String>[],
    },
  };
  await feedbackFile.writeAsString(_toYaml(feedback));
  return feedbackFile;
}

Future<File> _writeLearningReviewFixture(
  Directory root, {
  required String candidateRef,
  required String reviewerDecision,
  required List<String> consideredUserOutcomeFeedbackRefs,
  required String resultingApprovedMemoryRef,
  required String guardrailAssessment,
  required String decisionReason,
}) async {
  final decisionDir = Directory(
    p.join(root.path, '.harness', 'learning', 'reviews'),
  );
  await decisionDir.create(recursive: true);
  final decisionFile = File(p.join(decisionDir.path, 'decision.yaml'));
  final candidate = _loadYamlMap(_resolve(root, candidateRef));
  final userOutcomeSignal = Map<String, dynamic>.from(
    candidate['user_outcome_signal'] as Map,
  );
  final decision = {
    'candidate_ref': candidateRef,
    'candidate_user_outcome_status_at_review': userOutcomeSignal['status'],
    'reviewer_decision': reviewerDecision,
    'reviewer_identity': 'reviewer@example.com',
    'decision_timestamp': DateTime.now().toUtc().toIso8601String(),
    'decision_reason': decisionReason,
    'considered_user_outcome_feedback_refs': consideredUserOutcomeFeedbackRefs,
    'guardrail_cost_predicate': {
      'assessment': guardrailAssessment,
      'rationale': 'Promotion is justified for the verification fixture.',
    },
    'resulting_approved_memory_ref': resultingApprovedMemoryRef,
  };
  await decisionFile.writeAsString(_toYaml(decision));
  return decisionFile;
}

File _resolve(Directory root, String relativePath) =>
    File(p.join(root.path, relativePath));

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

String _toYaml(Object? value) =>
    const JsonEncoder.withIndent('  ').convert(value);

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
