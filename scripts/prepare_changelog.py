from __future__ import annotations

import argparse
import re
import subprocess
import sys
from pathlib import Path


FORBIDDEN_NOTE_PATTERNS = (
    re.compile(r"\bTODO\b", re.IGNORECASE),
    re.compile(r"\bTBD\b", re.IGNORECASE),
    re.compile(r"\bmaybe\b", re.IGNORECASE),
    re.compile(r"\bprobably\b", re.IGNORECASE),
    re.compile(r"/Users/[^\s`]+"),
    re.compile(r"/home/[^\s`]+"),
    re.compile(r"(?m)(^|[\s`])~/[^\s`]+"),
    re.compile(r"sk-[A-Za-z0-9_-]{12,}"),
    re.compile(r"(?i)(api[_-]?key|token|password)\s*[:=]\s*[A-Za-z0-9_.-]{8,}"),
)


def run_git(args: list[str]) -> str:
    result = subprocess.run(
        ["git", *args],
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or f"git {' '.join(args)} failed")
    return result.stdout.strip()


def version_from_tag(tag_name: str) -> str:
    if not tag_name.startswith("v") or len(tag_name) == 1:
        raise ValueError("Release tag must look like v0.6.1.")
    return tag_name[1:]


def find_previous_tag(tag_name: str) -> str | None:
    tags = run_git(["tag", "--merged", "HEAD", "--list", "v[0-9]*", "--sort=-v:refname"])
    for tag in tags.splitlines():
        tag = tag.strip()
        if tag and tag != tag_name:
            return tag
    return None


def read_release_sections(changelog_path: Path) -> list[tuple[str, str]]:
    lines = changelog_path.read_text(encoding="utf-8").splitlines()
    sections: list[tuple[str, str]] = []
    starts = [idx for idx, line in enumerate(lines) if line.startswith("## v")]
    for offset, start in enumerate(starts):
        end = starts[offset + 1] if offset + 1 < len(starts) else len(lines)
        sections.append((lines[start], "\n".join(lines[start + 1 : end]).strip()))
    return sections


def read_target_section(changelog_path: Path, tag_name: str) -> tuple[str, str] | None:
    target = f"## {tag_name} - "
    sections = read_release_sections(changelog_path)
    if not sections:
        return None
    heading, body = sections[0]
    if not heading.startswith(target):
        return None
    return heading, body


def read_note_subsections(body: str) -> dict[str, str]:
    sections: dict[str, str] = {}
    current_title: str | None = None
    current_lines: list[str] = []
    for line in body.splitlines():
        if line.startswith("### "):
            if current_title is not None:
                sections[current_title] = "\n".join(current_lines).strip()
            current_title = line[4:].strip()
            current_lines = []
            continue
        if current_title is not None:
            current_lines.append(line)
    if current_title is not None:
        sections[current_title] = "\n".join(current_lines).strip()
    return sections


def validate_changelog_section(changelog_path: Path, tag_name: str) -> None:
    section = read_target_section(changelog_path, tag_name)
    if section is None:
        raise ValueError(f"CHANGELOG.md does not have a top entry for {tag_name}.")

    heading, body = section
    if not re.match(rf"^## {re.escape(tag_name)} - \d{{4}}-\d{{2}}-\d{{2}}$", heading):
        raise ValueError(
            f"Top CHANGELOG heading must be exactly '## {tag_name} - YYYY-MM-DD'."
        )
    if not body:
        raise ValueError(f"CHANGELOG section for {tag_name} is empty.")
    subsections = read_note_subsections(body)
    if not subsections:
        raise ValueError(
            f"CHANGELOG section for {tag_name} must group notes under markdown subsections."
        )
    for title, content in subsections.items():
        bullets = [line for line in content.splitlines() if line.startswith("- ")]
        if not bullets:
            raise ValueError(
                f"CHANGELOG {title} section for {tag_name} must list at least one bullet."
            )
    for pattern in FORBIDDEN_NOTE_PATTERNS:
        match = pattern.search(body)
        if match:
            raise ValueError(
                f"CHANGELOG section for {tag_name} contains forbidden text: {match.group(0)}"
            )

    verification = subsections.get("Verification")
    if verification is None:
        raise ValueError(f"CHANGELOG section for {tag_name} must include Verification.")
    for bullet in [line for line in verification.splitlines() if line.startswith("- ")]:
        if "`" not in bullet:
            raise ValueError(
                "CHANGELOG Verification entries must be command bullets wrapped in backticks."
            )


def git_log(previous_tag: str | None) -> str:
    range_arg = f"{previous_tag}..HEAD" if previous_tag else "HEAD"
    return run_git(["log", range_arg, "--first-parent", "--oneline", "--decorate=no"])


def git_diff_name_status(previous_tag: str | None) -> str:
    range_arg = f"{previous_tag}..HEAD" if previous_tag else "HEAD"
    return run_git(["diff", "--name-status", range_arg])


def recent_changelog_style(changelog_path: Path) -> str:
    sections = read_release_sections(changelog_path)[:2]
    if not sections:
        return "(No previous CHANGELOG sections found.)"
    return "\n\n".join(f"{heading}\n\n{body}".strip() for heading, body in sections)


def release_contract_excerpt(spec_path: Path) -> str:
    text = spec_path.read_text(encoding="utf-8")
    marker = "## Release Publishing (operator)"
    start = text.find(marker)
    if start == -1:
        return "(Release Publishing section not found in docs/SPEC.md.)"
    end = text.find("\n## ", start + len(marker))
    return text[start : end if end != -1 else len(text)].strip()


def print_agent_guide(tag_name: str, changelog_path: Path, spec_path: Path) -> None:
    previous_tag = find_previous_tag(tag_name)
    range_label = f"{previous_tag}..HEAD" if previous_tag else "HEAD"
    print(
        f"""CHANGELOG.md has no top entry for {tag_name}.

Stop before publishing. Ask the agent to write the new top CHANGELOG section,
then rerun:

  ./publish.sh {tag_name}

Agent task:
Write the top CHANGELOG.md section for {tag_name}. Use the context below.
Do not edit any file other than CHANGELOG.md.

Required format:

## {tag_name} - YYYY-MM-DD

### Added
- User- or operator-visible change.

### Changed
- User- or operator-visible change.

### Fixed
- User- or operator-visible fix.

### Migration
- Include only when users need action.

### Verification
- `scripts/release_gate.sh`

Rules:
- Use only sections that are relevant.
- Summarize user/operator impact; do not dump commit hashes.
- Do not claim release-ready status except through actual gate evidence.
- Include Migration only when installation or upgrade behavior changed.
- Verification must list commands that were or will be run by this publish flow.
- Do not include TODO, TBD, maybe, probably, secrets, tokens, or home paths.
- Keep the style consistent with the recent CHANGELOG examples.

Release range: {range_label}

Commits:
{git_log(previous_tag) or "(No commits found.)"}

Changed files:
{git_diff_name_status(previous_tag) or "(No changed files found.)"}

Release contract excerpt:
{release_contract_excerpt(spec_path)}

Recent CHANGELOG style:
{recent_changelog_style(changelog_path)}
"""
    )


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Guide and validate release CHANGELOG authoring."
    )
    parser.add_argument("tag_name")
    parser.add_argument("--changelog", default="CHANGELOG.md")
    parser.add_argument("--spec", default="docs/SPEC.md")
    args = parser.parse_args()

    try:
        version_from_tag(args.tag_name)
        changelog_path = Path(args.changelog)
        spec_path = Path(args.spec)
        if read_target_section(changelog_path, args.tag_name) is None:
            print_agent_guide(args.tag_name, changelog_path, spec_path)
            return 1
        validate_changelog_section(changelog_path, args.tag_name)
    except (RuntimeError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        return 1

    print(f"CHANGELOG.md is ready for {args.tag_name}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
