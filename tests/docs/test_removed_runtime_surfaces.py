from __future__ import annotations

import subprocess
import sys
from pathlib import Path


REMOVED_SURFACES = [
    Path(".goreleaser.yaml"),
    Path("cmd"),
    Path("internal"),
    Path("go.mod"),
    Path("go.sum"),
    Path("assets/defaults/embed.go"),
    Path("assets/scaffold/embed.go"),
    Path("assets/skill/embed.go"),
    Path("packaging/homebrew"),
    Path("scripts/install_skill.sh"),
    Path("tool"),
]

ACTIVE_TEXT_ROOTS = [
    Path("AGENTS.md"),
    Path("README.md"),
    Path("README-kr.md"),
    Path("docs/SPEC.md"),
    Path(".harness/README.md"),
    Path("docs/ARCHITECTURE.md"),
    Path("docs/ARCHITECTURE-kr.md"),
    Path("docs/CONVENTIONS.md"),
    Path("skills/rail"),
    Path("assets/skill/Rail"),
    Path("assets/defaults/rules"),
    Path("assets/defaults/supervisor"),
    Path(".harness/rules"),
    Path(".harness/supervisor"),
    Path(".github/workflows"),
]

FORBIDDEN_RUNTIME_TEXT = (
    "codex exec",
    "codex_cli",
    "trusted PATH",
    "Homebrew symlink",
    "go test",
    "go build",
    "./build/rail",
    "rail compose-request",
    "rail validate-request",
    "rail run",
    "rail execute",
    "rail route-evaluation",
    "rail supervise",
    "rail status",
    "rail result",
    "rail integrate",
    "cmd/rail",
    "internal/",
    "Go control-plane",
    "Go CLI",
    "Go runtime",
    "gofmt",
    "goreleaser",
    ".goreleaser",
    "install_skill",
    "_test.go",
)


def test_go_and_codex_cli_runtime_surfaces_are_removed():
    present = [str(path) for path in REMOVED_SURFACES if path.exists()]
    go_files = [
        str(path)
        for path in Path(".").rglob("*.go")
        if ".git" not in path.parts and ".venv" not in path.parts and ".worktrees" not in path.parts
    ]

    assert present + go_files == []


def test_active_product_surfaces_do_not_reference_removed_runtime():
    findings: list[str] = []
    for root in ACTIVE_TEXT_ROOTS:
        paths = [root] if root.is_file() else [path for path in root.rglob("*") if path.is_file()]
        for path in paths:
            text = path.read_text(encoding="utf-8", errors="ignore")
            for forbidden in FORBIDDEN_RUNTIME_TEXT:
                if forbidden in text:
                    findings.append(f"{path}: {forbidden}")

    assert findings == []


def test_release_gate_exists_and_uses_python_runtime_only():
    path = Path("scripts/release_gate.sh")

    assert path.is_file()
    text = path.read_text(encoding="utf-8")
    assert "uv run --python 3.12 pytest -q" in text
    assert "--ignore=tests/e2e/test_optional_live_sdk_smoke.py" in text
    assert "uv run --python 3.12 ruff check src tests" in text
    assert "uv run --python 3.12 mypy src/rail" in text
    assert "uv build" in text
    assert "scripts/check_python_package_assets.py" in text
    assert "scripts/check_installed_wheel.py" in text
    assert "RAIL_ACTOR_RUNTIME_LIVE_SMOKE" in text
    for forbidden in FORBIDDEN_RUNTIME_TEXT:
        assert forbidden not in text


def test_optional_live_smoke_and_distribution_are_documented():
    readme = Path("README.md").read_text(encoding="utf-8")
    architecture = Path("docs/ARCHITECTURE.md").read_text(encoding="utf-8")

    assert "RAIL_ACTOR_RUNTIME_LIVE_SMOKE" in readme
    assert "rail-sdk" in readme
    assert "rail migrate" in readme
    assert "rail doctor" in readme
    assert "Python package" in readme
    assert "package asset" in readme
    assert "package asset" in architecture
    assert "no command-line product contract" in architecture


def test_rail_migration_script_exists():
    path = Path("scripts/migration_v0.6.0.sh")

    assert path.is_file()
    text = path.read_text(encoding="utf-8")
    assert "rail-sdk" in text
    assert "rail-harness" in text
    assert "rail.cli.main" in text
    assert "OPENAI_API_KEY" in text
    assert "skills/rail" in text
    assert "export RAIL_ACTOR_RUNTIME_LIVE" not in text


def test_publish_pipeline_is_tag_driven_and_gate_gated():
    path = Path(".github/workflows/publish.yml")

    assert path.is_file()
    text = path.read_text(encoding="utf-8")

    assert "on:" in text
    assert "push:" in text
    assert "tags:" in text
    assert '\"v*\"' in text
    assert "scripts/release_gate.sh" in text
    assert "scripts/check_release_metadata.py" in text
    assert "CHANGELOG.md" in text
    assert "id-token: write" in text
    assert "PYPI_API_TOKEN" not in text
    assert "password:" not in text
    assert "pypa/gh-action-pypi-publish@release/v1" in text
    assert "softprops/action-gh-release@v3" in text


def test_publish_script_runs_release_gate_and_pushes_tag():
    path = Path("publish.sh")

    assert path.is_file()
    text = path.read_text(encoding="utf-8")
    assert "./publish.sh vX.Y.Z" in text
    assert "scripts/check_release_metadata.py" in text
    assert "scripts/prepare_changelog.py" in text
    assert "scripts/release_gate.sh" in text
    assert "git push origin HEAD:main" in text
    assert 'git push origin "${TAG}"' in text
    assert "PYPI_API_TOKEN" not in text
    assert "TWINE_PASSWORD" not in text
    assert "<pypi_token>" not in text


def test_changelog_preparation_generates_notes_without_token_uploads():
    path = Path("scripts/prepare_changelog.py")

    assert path.is_file()
    text = path.read_text(encoding="utf-8")
    assert "git_log(previous_tag)" in text
    assert "generated_release_section" in text
    assert "insert_top_release_section" in text
    assert "Created CHANGELOG.md entry" in text
    assert "scripts/release_gate.sh" in text
    assert "TODO" in text
    assert "TBD" in text
    assert "/Users/" in text
    assert "/home/" in text
    assert "PYPI_API_TOKEN" not in text
    assert "TWINE_PASSWORD" not in text
    assert "<pypi_token>" not in text


def test_release_metadata_check_accepts_matching_tag_and_changelog(tmp_path: Path):
    pyproject = tmp_path / "pyproject.toml"
    changelog = tmp_path / "CHANGELOG.md"
    github_output = tmp_path / "github_output"

    pyproject.write_text(
        '[project]\nname = "rail-sdk"\nversion = "1.2.3"\n',
        encoding="utf-8",
    )
    changelog.write_text(
        "# Changelog\n\n"
        "## v1.2.3 - 2026-04-30\n\n"
        "### Added\n\n"
        "- Release metadata validation.\n"
        "- Preserved release note formatting.\n\n"
        "## v1.2.2 - 2026-04-29\n\n"
        "- Previous release.\n",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            "scripts/check_release_metadata.py",
            "v1.2.3",
            "--pyproject",
            str(pyproject),
            "--changelog",
            str(changelog),
            "--github-output",
            str(github_output),
        ],
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 0
    output = github_output.read_text(encoding="utf-8")
    assert "tag_version=1.2.3" in output
    assert "release_notes<<EOF\n" in output
    assert "### Added\n\n- Release metadata validation." in output
    assert "\\n" not in output


def test_release_metadata_check_rejects_top_changelog_mismatch(tmp_path: Path):
    pyproject = tmp_path / "pyproject.toml"
    changelog = tmp_path / "CHANGELOG.md"

    pyproject.write_text(
        '[project]\nname = "rail-sdk"\nversion = "1.2.3"\n',
        encoding="utf-8",
    )
    changelog.write_text(
        "# Changelog\n\n"
        "## v1.2.2 - 2026-04-29\n\n"
        "- Previous release.\n\n"
        "## v1.2.3 - 2026-04-30\n\n"
        "- Current release in the wrong position.\n",
        encoding="utf-8",
    )

    result = subprocess.run(
        [
            sys.executable,
            "scripts/check_release_metadata.py",
            "v1.2.3",
            "--pyproject",
            str(pyproject),
            "--changelog",
            str(changelog),
        ],
        check=False,
        capture_output=True,
        text=True,
    )

    assert result.returncode == 1
    assert "Top CHANGELOG entry is v1.2.2, expected v1.2.3." in result.stderr
