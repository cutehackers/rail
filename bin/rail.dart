import 'dart:io';

import 'package:rail/src/cli/rail_cli.dart';

Future<void> main(List<String> args) async {
  exitCode = await RailCli().run(args);
}
