import 'dart:convert';
import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:rail/src/runtime/harness_runner.dart';
import 'package:test/test.dart';

void main() {
  final repoRoot = Directory.current;
  final schemaRunner = HarnessRunner(repoRoot);
  final scenarios = <_RouteScenario>[
    _RouteScenario(
      name: 'rebuild_context',
      expectedStatus: 'rebuilding_context',
      expectedAction: 'rebuild_context',
      expectedCurrentActor: 'context_builder',
      expectedTraceText: 'rebuild_context',
      expectedContextRefreshTrigger: 'reason_codes',
      expectedReasonFamily: 'context',
    ),
    _RouteScenario(
      name: 'tighten_validation',
      expectedStatus: 'tightening_validation',
      expectedAction: 'tighten_validation',
      expectedCurrentActor: 'executor',
      expectedTraceText: 'tighten_validation',
    ),
    _RouteScenario(
      name: 'split_task',
      expectedStatus: 'split_required',
      expectedAction: 'split_task',
      expectedCurrentActor: null,
      expectedTraceText: 'split_task',
      expectedTerminalSummaryText: 'should be decomposed before continuing',
    ),
    _RouteScenario(
      name: 'blocked_environment',
      expectedStatus: 'blocked_environment',
      expectedAction: 'block_environment',
      expectedCurrentActor: null,
      expectedTraceText: 'block_environment',
      expectedTerminalSummaryText: 'blocked by environment',
    ),
  ];

  group('standard route fixtures', () {
    for (final scenario in scenarios) {
      test('${scenario.name} fixtures stay schema-valid', () async {
        final baseRelativePath = p.join(
          'test',
          'fixtures',
          'standard_route',
          scenario.name,
        );
        await schemaRunner.validateArtifact(
          filePath: p.join(baseRelativePath, 'evaluation_result.yaml'),
          schemaName: 'evaluation_result',
        );
        await schemaRunner.validateArtifact(
          filePath: p.join(baseRelativePath, 'execution_report.yaml'),
          schemaName: 'execution_report',
        );
      });

      test('${scenario.name} routes with the expected runtime state', () async {
        final tempRoot = await Directory(
          p.join(repoRoot.path, '.dart_tool', 'standard-route-fixtures'),
        ).create(recursive: true);
        final copied = Directory(
          p.join(
            tempRoot.path,
            '${scenario.name}-${DateTime.now().microsecondsSinceEpoch}',
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
            scenario.name,
          ),
        );
        await _copyDirectory(source, copied);

        final summary = await schemaRunner.routeEvaluation(artifactPath: copied.path);
        final state = HarnessState.fromJson(
          jsonDecode(
                await File(p.join(copied.path, 'state.json')).readAsString(),
              )
              as Map<String, dynamic>,
        );

        expect(summary, contains('status=${scenario.expectedStatus}'));
        expect(summary, contains('action=${scenario.expectedAction}'));
        expect(state.status, scenario.expectedStatus);
        expect(state.currentActor, scenario.expectedCurrentActor);
        expect(state.actionHistory, contains(scenario.expectedAction));

        final trace = await File(
          p.join(copied.path, 'supervisor_trace.md'),
        ).readAsString();
        expect(trace, contains(scenario.expectedTraceText));

        if (scenario.expectedContextRefreshTrigger != null) {
          expect(
            state.pendingContextRefreshTrigger,
            scenario.expectedContextRefreshTrigger,
          );
          expect(
            state.pendingContextRefreshReasonFamily,
            scenario.expectedReasonFamily,
          );
        }

        final terminalSummaryFile = File(p.join(copied.path, 'terminal_summary.md'));
        if (scenario.expectedTerminalSummaryText == null) {
          expect(terminalSummaryFile.existsSync(), isFalse);
        } else {
          expect(terminalSummaryFile.existsSync(), isTrue);
          expect(
            await terminalSummaryFile.readAsString(),
            contains(scenario.expectedTerminalSummaryText!),
          );
        }
      });
    }
  });
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

class _RouteScenario {
  const _RouteScenario({
    required this.name,
    required this.expectedStatus,
    required this.expectedAction,
    required this.expectedCurrentActor,
    required this.expectedTraceText,
    this.expectedTerminalSummaryText,
    this.expectedContextRefreshTrigger,
    this.expectedReasonFamily,
  });

  final String name;
  final String expectedStatus;
  final String expectedAction;
  final String? expectedCurrentActor;
  final String expectedTraceText;
  final String? expectedTerminalSummaryText;
  final String? expectedContextRefreshTrigger;
  final String? expectedReasonFamily;
}
