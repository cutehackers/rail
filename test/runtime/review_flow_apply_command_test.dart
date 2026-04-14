import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/cli/rail_cli.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  final repoRoot = Directory.current;

  test(
    'apply-user-outcome-feedback updates the matching quality candidate',
    () async {
      final tempRoot = await _createHarnessRoot(repoRoot, 'apply-feedback');
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      final candidateFile = await _writeQualityCandidateFixture(tempRoot);
      final candidateRef = p.relative(candidateFile.path, from: tempRoot.path);
      final feedbackFile = await _writeUserOutcomeFeedbackFixture(
        tempRoot,
        candidateRef: candidateRef,
      );
      final feedbackRef = p.relative(feedbackFile.path, from: tempRoot.path);

      final exitCode = await RailCli(
        root: tempRoot,
      ).run(['apply-user-outcome-feedback', '--file', feedbackRef]);

      expect(exitCode, 0);

      final updatedCandidate = _loadYamlMap(candidateFile);
      final userOutcomeSignal = Map<String, dynamic>.from(
        updatedCandidate['user_outcome_signal'] as Map,
      );
      expect(
        (userOutcomeSignal['supporting_feedback_refs'] as List<dynamic>)
            .cast<String>(),
        contains(feedbackRef),
      );
    },
  );

  test(
    'apply-learning-review accepts --decision alias and persists the review',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'apply-learning-review',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      final candidateFile = await _writeQualityCandidateFixture(tempRoot);
      final candidateRef = p.relative(candidateFile.path, from: tempRoot.path);
      final decisionFile = await _writeLearningReviewFixture(
        tempRoot,
        candidateRef: candidateRef,
      );
      final decisionRef = p.relative(decisionFile.path, from: tempRoot.path);

      final exitCode = await RailCli(
        root: tempRoot,
      ).run(['apply-learning-review', '--decision', decisionRef]);

      expect(exitCode, 0);
      expect(
        File(
          p.join(
            tempRoot.path,
            '.harness',
            'learning',
            'learning_review_decisions',
            '${_sanitize(candidateRef)}.yaml',
          ),
        ).existsSync(),
        isTrue,
      );
    },
  );

  test(
    'promote writes only the canonical approved-memory path for the family',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'apply-promote-canonical',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      final candidateFile = await _writeQualityCandidateFixture(
        tempRoot,
        runName: 'quality-promote-canonical',
      );
      final candidateRef = p.relative(candidateFile.path, from: tempRoot.path);
      final feedbackFile = await _writeUserOutcomeFeedbackFixture(
        tempRoot,
        candidateRef: candidateRef,
        feedbackName: 'feedback-promote-canonical',
      );
      final feedbackRef = p.relative(feedbackFile.path, from: tempRoot.path);
      final decisionFile = await _writeLearningReviewFixture(
        tempRoot,
        candidateRef: candidateRef,
        reviewerDecision: 'promote',
        consideredUserOutcomeFeedbackRefs: [feedbackRef],
        resultingApprovedMemoryRef: '.harness/learning/approved/bug_fix.yaml',
        guardrailAssessment: 'intervention_cost_does_not_explain_improvement',
        decisionReason:
            'Promote the reviewed same-family candidate into canonical approved memory.',
      );
      final decisionRef = p.relative(decisionFile.path, from: tempRoot.path);

      final exitCode = await RailCli(
        root: tempRoot,
      ).run(['apply-learning-review', '--decision', decisionRef]);

      expect(exitCode, 0);

      final approvedDir = Directory(
        p.join(tempRoot.path, '.harness', 'learning', 'approved'),
      );
      final approvedFiles = approvedDir
          .listSync(recursive: false)
          .whereType<File>()
          .map((file) => p.relative(file.path, from: tempRoot.path))
          .toList(growable: false);

      expect(approvedFiles, hasLength(1));
      expect(approvedFiles.single, '.harness/learning/approved/bug_fix.yaml');

      final decision = _loadYamlMap(decisionFile);
      expect(
        decision['resulting_approved_memory_ref'],
        '.harness/learning/approved/bug_fix.yaml',
      );
    },
  );

  test(
    'promote targeting a non-canonical approved-memory path is rejected',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'apply-promote-rejected',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      final candidateFile = await _writeQualityCandidateFixture(
        tempRoot,
        runName: 'quality-promote-rejected',
      );
      final candidateRef = p.relative(candidateFile.path, from: tempRoot.path);
      final feedbackFile = await _writeUserOutcomeFeedbackFixture(
        tempRoot,
        candidateRef: candidateRef,
        feedbackName: 'feedback-promote-rejected',
      );
      final feedbackRef = p.relative(feedbackFile.path, from: tempRoot.path);
      final decisionFile = await _writeLearningReviewFixture(
        tempRoot,
        candidateRef: candidateRef,
        reviewerDecision: 'promote',
        consideredUserOutcomeFeedbackRefs: [feedbackRef],
        resultingApprovedMemoryRef:
            '.harness/learning/approved/not-canonical.yaml',
        guardrailAssessment: 'intervention_cost_does_not_explain_improvement',
      );
      final decisionRef = p.relative(decisionFile.path, from: tempRoot.path);

      await expectLater(
        RailCli(
          root: tempRoot,
        ).run(['apply-learning-review', '--decision', decisionRef]),
        throwsA(isA<StateError>()),
      );
    },
  );

  test(
    'second same-family promote overwrites the same approved file',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'apply-promote-overwrite',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      final firstApprovedRef = await _promoteReviewedCandidate(
        tempRoot,
        runName: 'quality-promote-overwrite-1',
        candidateIdentifier: 'quality/candidate@1',
        feedbackName: 'feedback-promote-overwrite-1',
      );
      final firstApprovedFile = File(p.join(tempRoot.path, firstApprovedRef));
      final firstApproved = _loadYamlMap(firstApprovedFile);

      final secondApprovedRef = await _promoteReviewedCandidate(
        tempRoot,
        runName: 'quality-promote-overwrite-2',
        candidateIdentifier: 'quality/candidate@2',
        feedbackName: 'feedback-promote-overwrite-2',
      );
      final secondApprovedFile = File(p.join(tempRoot.path, secondApprovedRef));
      final secondApproved = _loadYamlMap(secondApprovedFile);

      final approvedDir = Directory(
        p.join(tempRoot.path, '.harness', 'learning', 'approved'),
      );
      final approvedFiles = approvedDir
          .listSync(recursive: false)
          .whereType<File>()
          .map((file) => p.relative(file.path, from: tempRoot.path))
          .toList(growable: false);

      expect(firstApprovedRef, '.harness/learning/approved/bug_fix.yaml');
      expect(secondApprovedRef, '.harness/learning/approved/bug_fix.yaml');
      expect(approvedFiles, hasLength(1));
      expect(approvedFiles.single, '.harness/learning/approved/bug_fix.yaml');
      expect(
        (secondApproved['originating_candidate_refs'] as List<dynamic>)
            .cast<String>(),
        contains(
          p.join(
            '.harness',
            'artifacts',
            'quality-promote-overwrite-2',
            'quality_learning_candidates',
            'candidate.yaml',
          ),
        ),
      );
      expect(
        (firstApproved['originating_candidate_refs'] as List<dynamic>)
            .cast<String>(),
        contains(
          p.join(
            '.harness',
            'artifacts',
            'quality-promote-overwrite-1',
            'quality_learning_candidates',
            'candidate.yaml',
          ),
        ),
      );
      expect(
        (secondApproved['originating_candidate_refs'] as List<dynamic>)
            .cast<String>(),
        contains(
          p.join(
            '.harness',
            'artifacts',
            'quality-promote-overwrite-2',
            'quality_learning_candidates',
            'candidate.yaml',
          ),
        ),
      );
    },
  );

  for (final disposition in ['hold', 'reject']) {
    test(
      '$disposition persists the review decision without deleting approved memory',
      () async {
        final tempRoot = await _createHarnessRoot(
          repoRoot,
          'apply-$disposition-preserves-approved',
        );
        addTearDown(() async {
          if (tempRoot.existsSync()) {
            await tempRoot.delete(recursive: true);
          }
        });

        final candidateFile = await _writeQualityCandidateFixture(
          tempRoot,
          runName: 'quality-$disposition-preserves-approved',
        );
        final candidateRef = p.relative(
          candidateFile.path,
          from: tempRoot.path,
        );
        final feedbackFile = await _writeUserOutcomeFeedbackFixture(
          tempRoot,
          candidateRef: candidateRef,
          feedbackName: 'feedback-$disposition-preserves-approved',
        );
        final feedbackRef = p.relative(feedbackFile.path, from: tempRoot.path);
        final promoteDecision = await _writeLearningReviewFixture(
          tempRoot,
          candidateRef: candidateRef,
          reviewerDecision: 'promote',
          consideredUserOutcomeFeedbackRefs: [feedbackRef],
          resultingApprovedMemoryRef: '.harness/learning/approved/bug_fix.yaml',
          guardrailAssessment: 'intervention_cost_does_not_explain_improvement',
          decisionReason:
              'Seed canonical approved memory before $disposition review.',
        );
        final promoteRef = p.relative(
          promoteDecision.path,
          from: tempRoot.path,
        );
        final promoteExitCode = await RailCli(
          root: tempRoot,
        ).run(['apply-learning-review', '--decision', promoteRef]);
        expect(promoteExitCode, 0);
        final approvedFile = File(
          p.join(
            tempRoot.path,
            '.harness',
            'learning',
            'approved',
            'bug_fix.yaml',
          ),
        );
        final approvedBeforeFollowUp = _loadYamlMap(approvedFile);

        final followUpDecision = await _writeLearningReviewFixture(
          tempRoot,
          candidateRef: candidateRef,
          reviewerDecision: disposition,
          consideredUserOutcomeFeedbackRefs: [feedbackRef],
          decisionReason:
              'Keep the approved file in place while recording a $disposition decision.',
        );
        final followUpRef = p.relative(
          followUpDecision.path,
          from: tempRoot.path,
        );
        final followUpExitCode = await RailCli(
          root: tempRoot,
        ).run(['apply-learning-review', '--decision', followUpRef]);

        expect(followUpExitCode, 0);
        expect(approvedFile.existsSync(), isTrue);
        expect(_loadYamlMap(approvedFile), approvedBeforeFollowUp);
        expect(
          File(
            p.join(
              tempRoot.path,
              '.harness',
              'learning',
              'learning_review_decisions',
              '${_sanitize(candidateRef)}.yaml',
            ),
          ).existsSync(),
          isTrue,
        );
      },
    );
  }

  test(
    'apply-hardening-review persists the review decision from --file',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'apply-hardening-review',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      final candidateFile = await _writeHardeningCandidateFixture(tempRoot);
      final candidateRef = p.relative(candidateFile.path, from: tempRoot.path);
      final decisionFile = await _writeHardeningReviewFixture(
        tempRoot,
        candidateRef: candidateRef,
      );
      final decisionRef = p.relative(decisionFile.path, from: tempRoot.path);

      final exitCode = await RailCli(
        root: tempRoot,
      ).run(['apply-hardening-review', '--file', decisionRef]);

      expect(exitCode, 0);
      expect(
        File(
          p.join(
            tempRoot.path,
            '.harness',
            'learning',
            'hardening_review_decisions',
            '${_sanitize(candidateRef)}.yaml',
          ),
        ).existsSync(),
        isTrue,
      );
    },
  );
}

Future<Directory> _createHarnessRoot(Directory repoRoot, String name) async {
  final root = Directory(
    p.join(
      repoRoot.path,
      '.dart_tool',
      'review-flow-apply',
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

Future<File> _writeQualityCandidateFixture(
  Directory root, {
  String runName = 'quality-run',
  String candidateIdentifier = 'quality/candidate@1',
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
      'summary': 'No reviewed user confirmation yet.',
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

Future<File> _writeHardeningCandidateFixture(Directory root) async {
  final artifactDir = Directory(
    p.join(
      root.path,
      '.harness',
      'artifacts',
      'hardening-run',
      'hardening_candidates',
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
    'candidate_identifier': 'hardening/candidate@1',
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
  String feedbackName = 'feedback.yaml',
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
  String reviewerDecision = 'hold',
  List<String> consideredUserOutcomeFeedbackRefs = const <String>[],
  String? resultingApprovedMemoryRef,
  String guardrailAssessment = 'not_assessed',
  String guardrailRationale =
      'No promotion decision is being made in this test.',
  String decisionReason =
      'Keep the candidate in review until more evidence accumulates.',
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
      'rationale': guardrailRationale,
    },
  };
  if (resultingApprovedMemoryRef != null) {
    decision['resulting_approved_memory_ref'] = resultingApprovedMemoryRef;
  }
  await decisionFile.writeAsString(_toYaml(decision));
  return decisionFile;
}

Future<String> _promoteReviewedCandidate(
  Directory root, {
  required String runName,
  required String candidateIdentifier,
  required String feedbackName,
}) async {
  final candidateFile = await _writeQualityCandidateFixture(
    root,
    runName: runName,
    candidateIdentifier: candidateIdentifier,
  );
  final candidateRef = p.relative(candidateFile.path, from: root.path);
  final feedbackFile = await _writeUserOutcomeFeedbackFixture(
    root,
    candidateRef: candidateRef,
    feedbackName: feedbackName,
  );
  final feedbackRef = p.relative(feedbackFile.path, from: root.path);
  final decisionFile = await _writeLearningReviewFixture(
    root,
    candidateRef: candidateRef,
    reviewerDecision: 'promote',
    consideredUserOutcomeFeedbackRefs: [feedbackRef],
    resultingApprovedMemoryRef: '.harness/learning/approved/bug_fix.yaml',
    guardrailAssessment: 'intervention_cost_does_not_explain_improvement',
    decisionReason:
        'Promote the reviewed same-family candidate into canonical approved memory.',
  );
  final decisionRef = p.relative(decisionFile.path, from: root.path);
  final exitCode = await RailCli(
    root: root,
  ).run(['apply-learning-review', '--decision', decisionRef]);
  expect(exitCode, 0);
  return '.harness/learning/approved/bug_fix.yaml';
}

Future<File> _writeHardeningReviewFixture(
  Directory root, {
  required String candidateRef,
}) async {
  final decisionDir = Directory(
    p.join(root.path, '.harness', 'learning', 'hardening-reviews'),
  );
  await decisionDir.create(recursive: true);
  final decisionFile = File(p.join(decisionDir.path, 'decision.yaml'));
  final candidate = _loadYamlMap(_resolve(root, candidateRef));
  final decision = {
    'hardening_candidate_ref': candidateRef,
    'reviewer_decision': 'hold',
    'reviewer_identity': 'reviewer@example.com',
    'decision_timestamp': DateTime.now().toUtc().toIso8601String(),
    'decision_reason': 'Keep the observation open for future hardening work.',
    'reviewed_observation_refs': candidate['supporting_evidence_refs'],
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

String _sanitize(String value) {
  final sanitized = value.replaceAll(RegExp(r'[^A-Za-z0-9._-]+'), '_');
  return sanitized.isEmpty ? 'decision' : sanitized;
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
