from __future__ import annotations

import argparse
import datetime as dt
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


def commit_subjects(previous_tag: str | None) -> list[str]:
    return [
        line.split(" ", 1)[1].strip() if " " in line else line.strip()
        for line in git_log(previous_tag).splitlines()
        if line.strip()
    ]


def release_note_for_commit(subject: str) -> tuple[str, str]:
    lowered = subject.lower()
    if lowered.startswith(("feat:", "add:", "added ")):
        return "Added", f"Added {subject.split(':', 1)[-1].strip()}."
    if lowered.startswith("fix:"):
        return "Fixed", f"Fixed {subject.split(':', 1)[-1].strip()}."
    if lowered.startswith(("ci:", "build:")):
        note = f"Updated release and build automation: {subject.split(':', 1)[-1].strip()}."
        return "Changed", note
    if lowered.startswith(("docs:", "doc:")):
        note = f"Updated release documentation: {subject.split(':', 1)[-1].strip()}."
        return "Changed", note
    if lowered.startswith("test:"):
        note = f"Expanded release validation coverage: {subject.split(':', 1)[-1].strip()}."
        return "Changed", note
    return "Changed", f"Updated {subject.rstrip('.')}."


def generated_release_section(tag_name: str, previous_tag: str | None) -> str:
    grouped: dict[str, list[str]] = {"Added": [], "Changed": [], "Fixed": []}
    for subject in commit_subjects(previous_tag):
        title, note = release_note_for_commit(subject)
        if note not in grouped.setdefault(title, []):
            grouped[title].append(note)

    if not any(grouped.values()):
        range_label = f"{previous_tag}..HEAD" if previous_tag else "HEAD"
        grouped["Changed"].append(
            f"Prepared release metadata for changes in `{range_label}`."
        )

    lines = [f"## {tag_name} - {dt.date.today().isoformat()}", ""]
    for title in ("Added", "Changed", "Fixed"):
        notes = grouped.get(title, [])
        if not notes:
            continue
        lines.extend([f"### {title}", ""])
        lines.extend(f"- {note}" for note in notes)
        lines.append("")
    lines.extend(["### Verification", "", "- `scripts/release_gate.sh`", ""])
    return "\n".join(lines)


def insert_top_release_section(changelog_path: Path, section: str) -> None:
    text = changelog_path.read_text(encoding="utf-8")
    lines = text.splitlines()
    insert_at = next(
        (idx for idx, line in enumerate(lines) if line.startswith("## v")),
        len(lines),
    )
    updated_lines = lines[:insert_at]
    while updated_lines and updated_lines[-1] == "":
        updated_lines.pop()
    remaining_lines = lines[insert_at:]
    while remaining_lines and remaining_lines[0] == "":
        remaining_lines.pop(0)
    new_text = "\n".join(
        [*updated_lines, "", section.rstrip(), "", *remaining_lines]
    ).rstrip()
    changelog_path.write_text(f"{new_text}\n", encoding="utf-8")


def ensure_changelog_section(changelog_path: Path, tag_name: str) -> bool:
    if read_target_section(changelog_path, tag_name) is not None:
        return False
    previous_tag = find_previous_tag(tag_name)
    insert_top_release_section(changelog_path, generated_release_section(tag_name, previous_tag))
    return True


def main() -> int:
    parser = argparse.ArgumentParser(
        description="Generate and validate release CHANGELOG authoring."
    )
    parser.add_argument("tag_name")
    parser.add_argument("--changelog", default="CHANGELOG.md")
    parser.add_argument("--spec", default="docs/SPEC.md")
    args = parser.parse_args()

    try:
        version_from_tag(args.tag_name)
        changelog_path = Path(args.changelog)
        created = ensure_changelog_section(changelog_path, args.tag_name)
        validate_changelog_section(changelog_path, args.tag_name)
    except (RuntimeError, ValueError) as exc:
        print(str(exc), file=sys.stderr)
        return 1

    if created:
        print(f"Created CHANGELOG.md entry for {args.tag_name}.")
    print(f"CHANGELOG.md is ready for {args.tag_name}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
