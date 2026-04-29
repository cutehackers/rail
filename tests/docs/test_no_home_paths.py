from __future__ import annotations

from pathlib import Path


ACTIVE_DOCS = [
    Path("README.md"),
    Path("docs/ARCHITECTURE.md"),
    Path("skills/rail/SKILL.md"),
    Path("assets/skill/Rail/SKILL.md"),
]

FORBIDDEN_ACTIVE_GUIDANCE = (
    "/Users/",
    "~/",
    "/home/",
    "codex exec",
    "codex_cli",
    "trusted PATH",
    "Homebrew symlink",
    "go test",
    "go build",
    "./build/rail",
    "Codex CLI",
)


def test_active_docs_and_skills_do_not_contain_home_paths_or_stale_runtime_guidance():
    findings: list[str] = []
    for path in ACTIVE_DOCS:
        text = path.read_text(encoding="utf-8")
        for forbidden in FORBIDDEN_ACTIVE_GUIDANCE:
            if forbidden in text:
                findings.append(f"{path}: {forbidden}")

    assert findings == []


def test_repo_skill_and_bundled_skill_are_aligned():
    assert Path("skills/rail/SKILL.md").read_text(encoding="utf-8") == Path("assets/skill/Rail/SKILL.md").read_text(
        encoding="utf-8"
    )
