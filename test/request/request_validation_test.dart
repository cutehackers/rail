import 'dart:io';

import 'package:path/path.dart' as p;
import 'package:test/test.dart';

void main() {
  test('validation request fixture remains valid', () async {
    final result = await Process.run(
      'dart',
      [
        'run',
        'bin/rail.dart',
        'validate-request',
        '--request',
        p.join('test', 'fixtures', 'valid_request.yaml'),
      ],
      workingDirectory: Directory.current.path,
    );

    expect(result.exitCode, 0, reason: '${result.stdout}\n${result.stderr}');
    expect(result.stdout, contains('Request is valid'));
  });
}
