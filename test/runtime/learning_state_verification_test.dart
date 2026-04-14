import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/cli/rail_cli.dart';
import 'package:rail/src/runtime/harness_runner.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  final repoRoot = Directory.current;

  test('verifyLearningState succeeds for coherent reviewed state', () async {
    final tempRoot = await _createHarnessRoot(repoRoot, 'healthy');
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    await _seedHealthyLearningState(tempRoot);
    final reviewQueueBefore = File(
      p.join(tempRoot.path, '.harness', 'learning', 'review_queue.yaml'),
    ).readAsStringSync();
    final hardeningQueueBefore = File(
      p.join(tempRoot.path, '.harness', 'learning', 'hardening_queue.yaml'),
    ).readAsStringSync();
    final familyEvidenceBefore = File(
      p.join(
        tempRoot.path,
        '.harness',
        'learning',
        'family_evidence_index.yaml',
      ),
    ).readAsStringSync();

    final output = await _verifyLearningState(tempRoot);
    expect(output, contains('Learning state verification passed'));

    expect(
      File(
        p.join(tempRoot.path, '.harness', 'learning', 'review_queue.yaml'),
      ).readAsStringSync(),
      reviewQueueBefore,
    );
    expect(
      File(
        p.join(tempRoot.path, '.harness', 'learning', 'hardening_queue.yaml'),
      ).readAsStringSync(),
      hardeningQueueBefore,
    );
    expect(
      File(
        p.join(
          tempRoot.path,
          '.harness',
          'learning',
          'family_evidence_index.yaml',
        ),
      ).readAsStringSync(),
      familyEvidenceBefore,
    );
  });

  test(
    'verifyLearningState fails when approved memory no longer matches schema',
    () async {
      final tempRoot = await _createHarnessRoot(repoRoot, 'invalid-approved');
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      await _seedHealthyLearningState(tempRoot);
      final approvedFile = File(
        p.join(
          tempRoot.path,
          '.harness',
          'learning',
          'approved',
          'bug_fix.yaml',
        ),
      );
      await approvedFile.writeAsString(_toYaml({'task_family': 'bug_fix'}));

      await expectLater(
        _verifyLearningState(tempRoot),
        throwsA(
          isA<StateError>().having(
            (error) => error.toString(),
            'message',
            allOf(
              contains('.harness/learning/approved/bug_fix.yaml'),
              contains('approved_family_memory'),
            ),
          ),
        ),
      );
    },
  );

  test(
    'verifyLearningState fails when approved memory uses a non-canonical family path',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'approved-path-drift',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      await _seedHealthyLearningState(tempRoot);
      final canonicalFile = File(
        p.join(
          tempRoot.path,
          '.harness',
          'learning',
          'approved',
          'bug_fix.yaml',
        ),
      );
      final nonCanonicalFile = File(
        p.join(
          tempRoot.path,
          '.harness',
          'learning',
          'approved',
          'bug-fix.yaml',
        ),
      );
      await nonCanonicalFile.parent.create(recursive: true);
      await nonCanonicalFile.writeAsString(await canonicalFile.readAsString());

      await expectLater(
        _verifyLearningState(tempRoot),
        throwsA(
          isA<StateError>().having(
            (error) => error.toString(),
            'message',
            allOf(
              contains('.harness/learning/approved/bug-fix.yaml'),
              contains('canonical family path'),
              contains('.harness/learning/approved/bug_fix.yaml'),
            ),
          ),
        ),
      );
    },
  );

  test('verifyLearningState fails when review queue snapshot drifts', () async {
    final tempRoot = await _createHarnessRoot(repoRoot, 'queue-drift');
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    await _seedHealthyLearningState(tempRoot);
    final reviewQueueFile = File(
      p.join(tempRoot.path, '.harness', 'learning', 'review_queue.yaml'),
    );
    final reviewQueue = _loadYamlMap(reviewQueueFile);
    reviewQueue['pending_candidate_groups'] = <Object?>[];
    await reviewQueueFile.writeAsString(_toYaml(reviewQueue));

    await expectLater(
      _verifyLearningState(tempRoot),
      throwsA(
        isA<StateError>().having(
          (error) => error.toString(),
          'message',
          allOf(
            contains('.harness/learning/review_queue.yaml'),
            contains('drift'),
          ),
        ),
      ),
    );
  });

  test(
    'verifyLearningState fails when review queue metadata alone drifts',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'queue-metadata-drift',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      await _seedHealthyLearningState(tempRoot);
      final reviewQueueFile = File(
        p.join(tempRoot.path, '.harness', 'learning', 'review_queue.yaml'),
      );
      final reviewQueue = _loadYamlMap(reviewQueueFile);
      reviewQueue['queue_generated_at'] = '1970-01-01T00:00:00.000Z';
      await reviewQueueFile.writeAsString(_toYaml(reviewQueue));

      await expectLater(
        _verifyLearningState(tempRoot),
        throwsA(
          isA<StateError>().having(
            (error) => error.toString(),
            'message',
            allOf(
              contains('.harness/learning/review_queue.yaml'),
              contains('queue_generated_at'),
              contains('mismatched field(s)'),
            ),
          ),
        ),
      );
    },
  );

  test(
    'verifyLearningState fails when family evidence snapshot drifts',
    () async {
      final tempRoot = await _createHarnessRoot(repoRoot, 'family-evidence');
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      await _seedHealthyLearningState(tempRoot);
      final indexFile = File(
        p.join(
          tempRoot.path,
          '.harness',
          'learning',
          'family_evidence_index.yaml',
        ),
      );
      final index = _loadYamlMap(indexFile);
      index['latest_approved_memory_refs_by_family'] = <String, Object?>{};
      await indexFile.writeAsString(_toYaml(index));

      await expectLater(
        _verifyLearningState(tempRoot),
        throwsA(
          isA<StateError>().having(
            (error) => error.toString(),
            'message',
            allOf(
              contains('.harness/learning/family_evidence_index.yaml'),
              contains('drift'),
            ),
          ),
        ),
      );
    },
  );

  test(
    'verifyLearningState fails when hardening queue snapshot drifts',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'hardening-queue-drift',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      await _seedHealthyLearningState(
        tempRoot,
        includePendingHardeningBacklog: true,
      );
      final hardeningQueueFile = File(
        p.join(tempRoot.path, '.harness', 'learning', 'hardening_queue.yaml'),
      );
      final hardeningQueue = _loadYamlMap(hardeningQueueFile);
      hardeningQueue['pending_hardening_entries'] = <Object?>[];
      await hardeningQueueFile.writeAsString(_toYaml(hardeningQueue));

      await expectLater(
        _verifyLearningState(tempRoot),
        throwsA(
          isA<StateError>().having(
            (error) => error.toString(),
            'message',
            allOf(
              contains('.harness/learning/hardening_queue.yaml'),
              contains('pending_hardening_entries'),
              contains('mismatched field(s)'),
            ),
          ),
        ),
      );
    },
  );

  test('verifyLearningState ignores unapplied draft review files', () async {
    final tempRoot = await _createHarnessRoot(repoRoot, 'ignore-draft-reviews');
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    await _seedHealthyLearningState(tempRoot);
    final reviewQueueFile = File(
      p.join(tempRoot.path, '.harness', 'learning', 'review_queue.yaml'),
    );
    final hardeningQueueFile = File(
      p.join(tempRoot.path, '.harness', 'learning', 'hardening_queue.yaml'),
    );
    final familyEvidenceFile = File(
      p.join(
        tempRoot.path,
        '.harness',
        'learning',
        'family_evidence_index.yaml',
      ),
    );
    final reviewQueueBefore = reviewQueueFile.readAsStringSync();
    final hardeningQueueBefore = hardeningQueueFile.readAsStringSync();
    final familyEvidenceBefore = familyEvidenceFile.readAsStringSync();

    final candidateRef = p.join(
      '.harness',
      'artifacts',
      'healthy-promoted-run',
      'quality_learning_candidates',
      'candidate.yaml',
    );
    await _writeLearningReviewFixture(
      tempRoot,
      candidateRef: candidateRef,
      reviewerDecision: 'reject',
      consideredUserOutcomeFeedbackRefs: const <String>[],
      resultingApprovedMemoryRef: '.harness/learning/approved/bug_fix.yaml',
      guardrailAssessment: 'not_assessed',
      decisionReason: 'Draft only; should not affect live derived state.',
      fileName: 'draft-only-review.yaml',
    );

    final output = await _verifyLearningState(tempRoot);
    expect(output, contains('Learning state verification passed'));
    expect(reviewQueueFile.readAsStringSync(), reviewQueueBefore);
    expect(hardeningQueueFile.readAsStringSync(), hardeningQueueBefore);
    expect(familyEvidenceFile.readAsStringSync(), familyEvidenceBefore);
  });

  test('verifyLearningState aggregates multiple invalid source files', () async {
    final tempRoot = await _createHarnessRoot(
      repoRoot,
      'aggregate-invalid-sources',
    );
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    await _seedHealthyLearningState(
      tempRoot,
      includePendingLearningBacklog: true,
    );
    final invalidCandidateFile = File(
      p.join(
        tempRoot.path,
        '.harness',
        'artifacts',
        'pending-learning-run',
        'quality_learning_candidates',
        'candidate.yaml',
      ),
    );
    await invalidCandidateFile.writeAsString(
      _toYaml({'task_family': 'bug_fix'}),
    );
    final invalidFeedbackFile = File(
      p.join(
        tempRoot.path,
        '.harness',
        'learning',
        'feedback',
        'accepted-feedback.yaml',
      ),
    );
    await invalidFeedbackFile.writeAsString(
      _toYaml({'feedback_classification': 'accepted'}),
    );

    await expectLater(
      _verifyLearningState(tempRoot),
      throwsA(
        isA<StateError>().having(
          (error) => error.toString(),
          'message',
          allOf(
            contains(
              '.harness/artifacts/pending-learning-run/quality_learning_candidates/candidate.yaml',
            ),
            contains('quality_learning_candidate'),
            contains('.harness/learning/feedback/accepted-feedback.yaml'),
            contains('user_outcome_feedback'),
          ),
        ),
      ),
    );
  });

  test(
    'verifyLearningState allows pending learning and hardening backlog when snapshots stay coherent',
    () async {
      final tempRoot = await _createHarnessRoot(repoRoot, 'pending-backlog');
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      await _seedHealthyLearningState(
        tempRoot,
        includePendingLearningBacklog: true,
        includePendingHardeningBacklog: true,
      );

      final output = await _verifyLearningState(tempRoot);
      expect(output, contains('Learning state verification passed'));
    },
  );
}

Future<String> _verifyLearningState(Directory root) async {
  final stdoutBuffer = StringBuffer();
  final stderrBuffer = StringBuffer();
  final exitCode = await RailCli(root: root).run(
    const ['verify-learning-state'],
    stdoutSink: stdoutBuffer,
    stderrSink: stderrBuffer,
  );

  expect(exitCode, 0);
  expect(stderrBuffer.toString(), isEmpty);
  return stdoutBuffer.toString();
}

Future<Directory> _createHarnessRoot(Directory repoRoot, String name) async {
  final root = Directory(
    p.join(
      repoRoot.path,
      '.dart_tool',
      'learning-state-verification',
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

Future<void> _seedHealthyLearningState(
  Directory root, {
  bool includePendingLearningBacklog = false,
  bool includePendingHardeningBacklog = false,
}) async {
  final runner = HarnessRunner(root);
  if (includePendingLearningBacklog) {
    await _writeQualityCandidateFixture(
      root,
      runName: 'pending-learning-run',
      candidateIdentifier: 'quality/pending@1',
    );
  }
  if (includePendingHardeningBacklog) {
    await _writeHardeningCandidateFixture(
      root,
      runName: 'pending-hardening-run',
    );
  }

  final candidateFile = await _writeQualityCandidateFixture(
    root,
    runName: 'healthy-promoted-run',
    candidateIdentifier: 'quality/promoted@1',
  );
  final candidateRef = p.relative(candidateFile.path, from: root.path);
  final feedbackFile = await _writeUserOutcomeFeedbackFixture(
    root,
    candidateRef: candidateRef,
    feedbackName: 'accepted-feedback.yaml',
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
  required String runName,
  required String candidateIdentifier,
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
    'candidate_identifier': candidateIdentifier,
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

Future<File> _writeHardeningCandidateFixture(
  Directory root, {
  required String runName,
}) async {
  final artifactDir = Directory(
    p.join(root.path, '.harness', 'artifacts', runName, 'hardening_candidates'),
  );
  await artifactDir.create(recursive: true);
  final candidateFile = File(p.join(artifactDir.path, 'candidate.yaml'));
  final runRef = p.relative(artifactDir.parent.path, from: root.path);
  final candidate = {
    'originating_run_artifact_identity': {
      'run_ref': runRef,
      'artifact_ref': p.join(runRef, 'evaluation_result.yaml'),
    },
    'candidate_identifier': 'hardening/pending@1',
    'task_family': 'bug_fix',
    'task_family_source': 'task_type',
    'policy_affecting_observation':
        'This observation requires explicit hardening review.',
    'why_it_must_not_become_reusable_family_memory':
        'The note changes operator policy rather than same-family execution guidance.',
    'supporting_evidence_refs': <String>[
      p.join(runRef, 'evaluation_result.yaml'),
    ],
    'hardening_recommendation': 'hold',
  };
  await candidateFile.writeAsString(_toYaml(candidate));
  return candidateFile;
}

Future<File> _writeUserOutcomeFeedbackFixture(
  Directory root, {
  required String candidateRef,
  required String feedbackName,
}) async {
  final feedbackDir = Directory(
    p.join(root.path, '.harness', 'learning', 'feedback'),
  );
  await feedbackDir.create(recursive: true);
  final feedbackFile = File(p.join(feedbackDir.path, feedbackName));
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
  String fileName = 'decision.yaml',
}) async {
  final decisionDir = Directory(
    p.join(root.path, '.harness', 'learning', 'reviews'),
  );
  await decisionDir.create(recursive: true);
  final decisionFile = File(p.join(decisionDir.path, fileName));
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
