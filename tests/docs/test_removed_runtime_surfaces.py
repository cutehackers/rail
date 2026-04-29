from __future__ import annotations

from pathlib import Path


REMOVED_SURFACES = [
    Path("cmd"),
    Path("internal"),
    Path("go.mod"),
    Path("go.sum"),
    Path("assets/defaults/embed.go"),
    Path("assets/scaffold/embed.go"),
    Path("assets/skill/embed.go"),
    Path("packaging/homebrew"),
    Path("tool"),
]

ACTIVE_TEXT_ROOTS = [
    Path("README.md"),
    Path("README-kr.md"),
    Path("docs/ARCHITECTURE.md"),
    Path("docs/ARCHITECTURE-kr.md"),
    Path("skills/rail"),
    Path("assets/skill/Rail"),
    Path("assets/defaults/supervisor"),
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
    "cmd/rail",
    "internal/",
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
