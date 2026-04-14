import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/cli/rail_cli.dart';
import 'package:rail/src/runtime/harness_runner.dart';
import 'package:test/test.dart';
import 'package:yaml/yaml.dart';

void main() {
  final repoRoot = Directory.current;

  test('a healthy reviewed same-family approved file yields reuse', () async {
    final tempRoot = await _createHarnessRoot(
      repoRoot,
      'approved-memory-reuse',
    );
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    final projectRoot = await _createProjectRoot(
      tempRoot,
      'approved-memory-reuse-project',
    );
    final requestFile = await _writeRequestFixture(
      tempRoot,
      fileName: 'healthy-request.yaml',
      taskType: 'bug_fix',
      goal: 'reuse approved memory for bug-fix',
      feature: 'bug_fix',
      constraints: const ['task-family'],
    );

    await _seedReviewedApprovedMemory(
      tempRoot,
      runName: 'healthy-review-run',
      candidateIdentifier: 'quality/candidate@healthy',
      feedbackName: 'healthy-feedback.yaml',
    );

    await _alignApprovedMemoryWithCurrentLatestSuccessRef(
      tempRoot,
      familyKey: 'bug_fix::task_type',
      approvedMemoryRef: '.harness/learning/approved/bug_fix.yaml',
    );

    final consideration = await _runAndLoadApprovedMemoryConsideration(
      tempRoot,
      requestFile: requestFile,
      projectRoot: projectRoot,
      taskId: 'approved-memory-reuse-artifact',
    );

    expect(consideration['disposition'], 'reuse');
  });

  test('request incompatibility causes drop', () async {
    final tempRoot = await _createHarnessRoot(
      repoRoot,
      'approved-memory-request-conflict',
    );
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    final projectRoot = await _createProjectRoot(
      tempRoot,
      'approved-memory-request-conflict-project',
    );
    final requestFile = await _writeRequestFixture(
      tempRoot,
      fileName: 'request-conflict-request.yaml',
      taskType: 'bug_fix',
      goal: 'reuse approved memory for bug-fix',
      feature: 'feature_addition',
      constraints: const ['task-family'],
    );

    await _seedReviewedApprovedMemory(
      tempRoot,
      runName: 'request-conflict-review-run',
      candidateIdentifier: 'quality/candidate@request-conflict',
      feedbackName: 'request-conflict-feedback.yaml',
    );

    await _alignApprovedMemoryWithCurrentLatestSuccessRef(
      tempRoot,
      familyKey: 'bug_fix::task_type',
      approvedMemoryRef: '.harness/learning/approved/bug_fix.yaml',
    );

    final consideration = await _runAndLoadApprovedMemoryConsideration(
      tempRoot,
      requestFile: requestFile,
      projectRoot: projectRoot,
      taskId: 'approved-memory-request-conflict-artifact',
    );

    expect(consideration['disposition'], 'drop');
    expect(
      (consideration['reasons'] as List<dynamic>).cast<String>(),
      contains('request_conflict'),
    );
  });

  test(
    'a canonical file whose latest evidence is stale or conflicting yields quarantine',
    () async {
      final tempRoot = await _createHarnessRoot(
        repoRoot,
        'approved-memory-quarantine',
      );
      addTearDown(() async {
        if (tempRoot.existsSync()) {
          await tempRoot.delete(recursive: true);
        }
      });

      final projectRoot = await _createProjectRoot(
        tempRoot,
        'approved-memory-quarantine-project',
      );
      final requestFile = await _writeRequestFixture(
        tempRoot,
        fileName: 'quarantine-request.yaml',
        taskType: 'feature_addition',
        goal: 'approved-memory bounded reuse',
        feature: 'quality-learning',
        constraints: const ['bounded'],
      );

      await _seedReviewedApprovedMemory(
        tempRoot,
        runName: 'quarantine-review-run',
        candidateIdentifier: 'quality/candidate@quarantine',
        feedbackName: 'quarantine-feedback.yaml',
        taskFamily: 'feature_addition',
        approvedMemoryRef: '.harness/learning/approved/feature_addition.yaml',
      );

      await File(
        p.join(
          tempRoot.path,
          '.harness',
          'learning',
          'approved',
          'feature_addition.yaml',
        ),
      ).writeAsString(
        await File(
          p.join(
            repoRoot.path,
            '.harness',
            'fixtures',
            'approved-memory',
            'latest-evidence-conflict.yaml',
          ),
        ).readAsString(),
      );

      final consideration = await _runAndLoadApprovedMemoryConsideration(
        tempRoot,
        requestFile: requestFile,
        projectRoot: projectRoot,
        taskId: 'approved-memory-quarantine-artifact',
      );

      expect(consideration['disposition'], 'quarantine');
      expect(
        (consideration['reasons'] as List<dynamic>).cast<String>(),
        contains('latest_success_ref_mismatch'),
      );
    },
  );

  test('repository incompatibility causes quarantine', () async {
    final tempRoot = await _createHarnessRoot(
      repoRoot,
      'approved-memory-repository-conflict',
    );
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    final projectRoot = await _createProjectRoot(
      tempRoot,
      'approved-memory-repository-conflict-project',
    );
    final requestFile = await _writeRequestFixture(
      tempRoot,
      fileName: 'repository-conflict-request.yaml',
      taskType: 'feature_addition',
      goal: 'approved-memory bounded reuse',
      feature: 'quality-learning',
      constraints: const ['bounded'],
    );

    await _seedReviewedApprovedMemory(
      tempRoot,
      runName: 'repository-conflict-review-run',
      candidateIdentifier: 'quality/candidate@repository-conflict',
      feedbackName: 'repository-conflict-feedback.yaml',
      taskFamily: 'feature_addition',
      approvedMemoryRef: '.harness/learning/approved/feature_addition.yaml',
    );

    await File(
      p.join(
        tempRoot.path,
        '.harness',
        'learning',
        'approved',
        'feature_addition.yaml',
      ),
    ).writeAsString(
      await File(
        p.join(
          repoRoot.path,
          '.harness',
          'fixtures',
          'approved-memory',
          'repository-condition-mismatch.yaml',
        ),
      ).readAsString(),
    );

    await _alignApprovedMemoryWithCurrentLatestSuccessRef(
      tempRoot,
      familyKey: 'feature_addition::task_type',
      approvedMemoryRef: '.harness/learning/approved/feature_addition.yaml',
    );

    final consideration = await _runAndLoadApprovedMemoryConsideration(
      tempRoot,
      requestFile: requestFile,
      projectRoot: projectRoot,
      taskId: 'approved-memory-repository-conflict-artifact',
    );

    expect(consideration['disposition'], 'quarantine');
    expect(
      (consideration['reasons'] as List<dynamic>).cast<String>(),
      contains('repository_condition_mismatch'),
    );
  });

  test('a family/path mismatch yields drop', () async {
    final tempRoot = await _createHarnessRoot(repoRoot, 'approved-memory-drop');
    addTearDown(() async {
      if (tempRoot.existsSync()) {
        await tempRoot.delete(recursive: true);
      }
    });

    final projectRoot = await _createProjectRoot(
      tempRoot,
      'approved-memory-drop-project',
    );
    final requestFile = await _writeRequestFixture(
      tempRoot,
      fileName: 'drop-request.yaml',
      taskType: 'bug_fix',
      goal: 'reuse approved memory for bug_fix',
      feature: 'bug_fix',
    );

    await _seedReviewedApprovedMemory(
      tempRoot,
      runName: 'drop-review-run',
      candidateIdentifier: 'quality/candidate@drop',
      feedbackName: 'drop-feedback.yaml',
    );

    final approvedFile = File(
      p.join(tempRoot.path, '.harness', 'learning', 'approved', 'bug_fix.yaml'),
    );
    final approvedMemory = _loadYamlMap(approvedFile);
    approvedMemory['task_family'] = 'feature_addition';
    await approvedFile.writeAsString(_toYaml(approvedMemory));

    final consideration = await _runAndLoadApprovedMemoryConsideration(
      tempRoot,
      requestFile: requestFile,
      projectRoot: projectRoot,
      taskId: 'approved-memory-drop-artifact',
    );

    expect(consideration['disposition'], 'drop');
    expect(
      (consideration['reasons'] as List<dynamic>).cast<String>(),
      contains('task_family_mismatch'),
    );
  });
}

Future<Directory> _createHarnessRoot(Directory repoRoot, String name) async {
  final root = Directory(
    p.join(
      repoRoot.path,
      '.dart_tool',
      'approved-memory-consideration',
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
  await _copyDirectory(
    Directory(p.join(repoRoot.path, '.harness', 'rules')),
    Directory(p.join(root.path, '.harness', 'rules')),
  );
  await _copyDirectory(
    Directory(p.join(repoRoot.path, '.harness', 'rubrics')),
    Directory(p.join(root.path, '.harness', 'rubrics')),
  );
  return root;
}

Future<Directory> _createProjectRoot(Directory parent, String name) async {
  final root = Directory(p.join(parent.path, name));
  await root.create(recursive: true);
  final binDir = Directory(p.join(root.path, 'bin'));
  await binDir.create(recursive: true);
  await File(
    p.join(binDir.path, 'rail.dart'),
  ).writeAsString('// project root stub\n');
  return root;
}

Future<File> _writeRequestFixture(
  Directory root, {
  required String fileName,
  required String taskType,
  required String goal,
  required String feature,
  List<String> constraints = const <String>[],
}) async {
  final requestDir = Directory(p.join(root.path, '.harness', 'requests'));
  await requestDir.create(recursive: true);
  final requestFile = File(p.join(requestDir.path, fileName));
  final request = {
    'task_type': taskType,
    'goal': goal,
    'context': {'feature': feature},
    'constraints': constraints,
    'definition_of_done': <String>['approved memory is considered correctly'],
    'risk_tolerance': 'low',
    'validation_profile': 'standard',
  };
  await requestFile.writeAsString(_toYaml(request));
  return requestFile;
}

Future<void> _seedReviewedApprovedMemory(
  Directory root, {
  required String runName,
  required String candidateIdentifier,
  required String feedbackName,
  String taskFamily = 'bug_fix',
  String approvedMemoryRef = '.harness/learning/approved/bug_fix.yaml',
}) async {
  final candidateFile = await _writeQualityCandidateFixture(
    root,
    runName: runName,
    candidateIdentifier: candidateIdentifier,
    taskFamily: taskFamily,
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
    resultingApprovedMemoryRef: approvedMemoryRef,
    guardrailAssessment: 'intervention_cost_does_not_explain_improvement',
  );
  final decisionRef = p.relative(decisionFile.path, from: root.path);
  final exitCode = await RailCli(
    root: root,
  ).run(['apply-learning-review', '--file', decisionRef]);
  expect(exitCode, 0);
}

Future<void> _alignApprovedMemoryWithCurrentLatestSuccessRef(
  Directory root, {
  required String familyKey,
  required String approvedMemoryRef,
}) async {
  final evidenceIndexFile = File(
    p.join(root.path, '.harness', 'learning', 'family_evidence_index.yaml'),
  );
  final approvedFile = File(p.join(root.path, approvedMemoryRef));
  final evidenceIndex = _loadYamlMap(evidenceIndexFile);
  final latestSuccessMap =
      evidenceIndex['latest_confirmed_success_refs_by_family'];
  final latestSuccessRef =
      latestSuccessMap is Map && latestSuccessMap[familyKey] is Map
      ? Map<String, dynamic>.from(
          latestSuccessMap[familyKey] as Map,
        )['ref']?.toString()
      : null;
  if (latestSuccessRef == null || latestSuccessRef.isEmpty) {
    return;
  }

  final approvedMemory = _loadYamlMap(approvedFile);
  final latestExpectations = Map<String, dynamic>.from(
    approvedMemory['latest_family_evidence_expectations'] as Map,
  );
  latestExpectations['required_latest_success_ref'] = latestSuccessRef;
  approvedMemory['latest_family_evidence_expectations'] = latestExpectations;
  await approvedFile.writeAsString(_toYaml(approvedMemory));
}

Future<Map<String, dynamic>> _runAndLoadApprovedMemoryConsideration(
  Directory root, {
  required File requestFile,
  required Directory projectRoot,
  required String taskId,
}) async {
  final artifactPath = await HarnessRunner(root).run(
    requestPath: p.relative(requestFile.path, from: root.path),
    projectRoot: projectRoot.path,
    force: true,
    taskId: taskId,
  );
  final executionReport = _loadYamlMap(
    File(p.join(artifactPath, 'execution_report.yaml')),
  );
  final consideration = executionReport['approved_memory_consideration'];
  if (consideration is! Map) {
    throw StateError(
      'Expected approved_memory_consideration in $artifactPath/execution_report.yaml.',
    );
  }
  return Map<String, dynamic>.from(consideration);
}

Future<File> _writeQualityCandidateFixture(
  Directory root, {
  required String runName,
  required String candidateIdentifier,
  required String taskFamily,
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
    'task_family': taskFamily,
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
    'decision_reason': 'Promote reviewed memory for consideration tests.',
    'considered_user_outcome_feedback_refs': consideredUserOutcomeFeedbackRefs,
    'resulting_approved_memory_ref': resultingApprovedMemoryRef,
    'guardrail_cost_predicate': {
      'assessment': guardrailAssessment,
      'rationale': 'Promotion is explicitly allowed for this test.',
    },
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
