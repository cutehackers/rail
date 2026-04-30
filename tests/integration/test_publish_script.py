from __future__ import annotations

import os
import shutil
import stat
import subprocess
from pathlib import Path


PROJECT_ROOT = Path(__file__).resolve().parents[2]


def run(
    command: list[str],
    *,
    cwd: Path,
    env: dict[str, str] | None = None,
    check: bool = True,
) -> subprocess.CompletedProcess[str]:
    result = subprocess.run(
        command,
        cwd=cwd,
        env=env,
        check=False,
        capture_output=True,
        text=True,
    )
    if check and result.returncode != 0:
        raise AssertionError(
            f"Command failed: {command}\nstdout:\n{result.stdout}\nstderr:\n{result.stderr}"
        )
    return result


def make_executable(path: Path) -> None:
    path.chmod(path.stat().st_mode | stat.S_IXUSR)


def write_uv_stub(bin_dir: Path) -> None:
    bin_dir.mkdir()
    uv = bin_dir / "uv"
    uv.write_text(
        "#!/usr/bin/env bash\n"
        "set -euo pipefail\n"
        'if [[ "${1:-}" == "lock" ]]; then\n'
        "  exit 0\n"
        "fi\n"
        'echo "unexpected uv args: $*" >&2\n'
        "exit 1\n",
        encoding="utf-8",
    )
    make_executable(uv)


def prepare_release_repo(tmp_path: Path, release_gate: str) -> tuple[Path, Path, dict[str, str]]:
    repo = tmp_path / "repo"
    remote = tmp_path / "remote.git"
    bin_dir = tmp_path / "bin"
    repo.mkdir()

    run(["git", "init", "-b", "main"], cwd=repo)
    run(["git", "config", "user.email", "test@example.com"], cwd=repo)
    run(["git", "config", "user.name", "Test User"], cwd=repo)

    shutil.copy2(PROJECT_ROOT / "publish.sh", repo / "publish.sh")
    make_executable(repo / "publish.sh")

    scripts = repo / "scripts"
    scripts.mkdir()
    shutil.copy2(PROJECT_ROOT / "scripts/check_release_metadata.py", scripts)
    gate = scripts / "release_gate.sh"
    gate.write_text(release_gate, encoding="utf-8")
    make_executable(gate)

    (repo / "CHANGELOG.md").write_text(
        "# Changelog\n\n"
        "## v1.2.3 - 2026-04-30\n\n"
        "- Test release.\n\n"
        "## v1.2.2 - 2026-04-29\n\n"
        "- Previous release.\n",
        encoding="utf-8",
    )
    (repo / "pyproject.toml").write_text(
        '[project]\nname = "rail-sdk"\nversion = "0.0.0"\n',
        encoding="utf-8",
    )
    (repo / "uv.lock").write_text("# lock\n", encoding="utf-8")
    (repo / "README.md").write_text("# fixture\n", encoding="utf-8")

    run(["git", "add", "."], cwd=repo)
    run(["git", "commit", "-m", "initial"], cwd=repo)
    run(["git", "init", "--bare", str(remote)], cwd=tmp_path)
    run(["git", "remote", "add", "origin", str(remote)], cwd=repo)
    run(["git", "push", "-u", "origin", "main"], cwd=repo)

    write_uv_stub(bin_dir)
    env = os.environ.copy()
    env["PATH"] = f"{bin_dir}{os.pathsep}{env['PATH']}"
    return repo, remote, env


def test_publish_script_commits_release_metadata_and_pushes_tag(tmp_path: Path):
    repo, _remote, env = prepare_release_repo(
        tmp_path,
        "#!/usr/bin/env bash\nset -euo pipefail\n",
    )

    result = run(["./publish.sh", "v1.2.3"], cwd=repo, env=env)

    assert "Published v1.2.3." in result.stdout
    assert 'version = "1.2.3"' in (repo / "pyproject.toml").read_text(encoding="utf-8")
    assert run(["git", "tag", "--list", "v1.2.3"], cwd=repo).stdout.strip() == "v1.2.3"

    local_head = run(["git", "rev-parse", "HEAD"], cwd=repo).stdout.strip()
    remote_main = run(
        ["git", "ls-remote", "origin", "refs/heads/main"],
        cwd=repo,
    ).stdout.split()[0]
    remote_tag = run(
        ["git", "ls-remote", "--tags", "origin", "refs/tags/v1.2.3"],
        cwd=repo,
    ).stdout

    assert remote_main == local_head
    assert "refs/tags/v1.2.3" in remote_tag


def test_publish_script_refuses_post_gate_unrelated_dirty_file(tmp_path: Path):
    repo, _remote, env = prepare_release_repo(
        tmp_path,
        "#!/usr/bin/env bash\n"
        "set -euo pipefail\n"
        'printf "changed by gate\\n" > README.md\n',
    )

    result = run(["./publish.sh", "v1.2.3"], cwd=repo, env=env, check=False)

    assert result.returncode == 1
    assert "Refusing to publish with unrelated dirty file: README.md" in result.stderr
    assert run(["git", "tag", "--list", "v1.2.3"], cwd=repo).stdout.strip() == ""
    assert (
        run(
            ["git", "ls-remote", "--tags", "origin", "refs/tags/v1.2.3"],
            cwd=repo,
        ).stdout
        == ""
    )


def test_publish_script_refuses_renamed_or_copied_release_files(tmp_path: Path):
    repo, _remote, env = prepare_release_repo(
        tmp_path,
        "#!/usr/bin/env bash\nset -euo pipefail\n",
    )
    run(["git", "mv", "README.md", "RELEASE.md"], cwd=repo)

    result = run(["./publish.sh", "v1.2.3"], cwd=repo, env=env, check=False)

    assert result.returncode == 1
    assert "Refusing to publish with renamed or copied file:" in result.stderr
