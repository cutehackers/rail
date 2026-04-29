from __future__ import annotations

from pathlib import Path

import pytest

import rail.workspace.validation_runner as validation_runner


@pytest.fixture(autouse=True)
def trusted_test_sandbox_exec(tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
    sandbox_exec = tmp_path / "trusted-bin" / "sandbox-exec"
    sandbox_exec.parent.mkdir()
    sandbox_exec.write_text(
        "#!/bin/sh\n"
        "if [ \"$1\" = \"-p\" ]; then\n"
        "  shift 2\n"
        "fi\n"
        "exec \"$@\"\n",
        encoding="utf-8",
    )
    sandbox_exec.chmod(0o755)
    monkeypatch.setattr(validation_runner, "_TRUSTED_SANDBOX_EXEC", sandbox_exec)
