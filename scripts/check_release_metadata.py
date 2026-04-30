from __future__ import annotations

import argparse
import sys
import tomllib
from pathlib import Path


def _version_from_tag(tag_name: str) -> str:
    if not tag_name.startswith("v"):
        raise ValueError("Release tags must start with v, for example v1.2.3.")
    version = tag_name[1:]
    if not version:
        raise ValueError("Release tag must include a version after v.")
    return version


def _read_package_version(pyproject_path: Path) -> str:
    with pyproject_path.open("rb") as f:
        data = tomllib.load(f)
    version = data.get("project", {}).get("version")
    if not isinstance(version, str) or not version:
        raise ValueError(f"Could not read project.version from {pyproject_path}.")
    return version


def _read_changelog_section(changelog_path: Path, version: str) -> tuple[str, str]:
    lines = changelog_path.read_text(encoding="utf-8").splitlines()
    top_version: str | None = None
    start: int | None = None
    target = f"## v{version} - "

    for idx, line in enumerate(lines):
        if not line.startswith("## v") or " - " not in line:
            continue
        section_version = line.split(" - ", 1)[0][4:]
        if top_version is None:
            top_version = section_version
        if line.startswith(target):
            start = idx + 1
            break

    if top_version is None:
        raise ValueError(f"Could not find a release section in {changelog_path}.")
    if top_version != version:
        raise ValueError(f"Top CHANGELOG entry is v{top_version}, expected v{version}.")
    if start is None:
        raise ValueError(f"No changelog section found for v{version}.")

    end = len(lines)
    for idx in range(start, len(lines)):
        if lines[idx].startswith("## v"):
            end = idx
            break

    notes = "\n".join(lines[start:end]).strip()
    if not notes:
        raise ValueError(f"CHANGELOG section for v{version} is empty.")
    return top_version, notes


def _append_github_output(path: Path, version: str, notes: str) -> None:
    with path.open("a", encoding="utf-8") as f:
        f.write(f"tag_version={version}\n")
        f.write("release_notes<<EOF\n")
        f.write(notes)
        f.write("\nEOF\n")


def main() -> int:
    parser = argparse.ArgumentParser(description="Validate Rail release metadata.")
    parser.add_argument("tag_name")
    parser.add_argument("--pyproject", default="pyproject.toml")
    parser.add_argument("--changelog", default="CHANGELOG.md")
    parser.add_argument("--github-output")
    args = parser.parse_args()

    try:
        version = _version_from_tag(args.tag_name)
        package_version = _read_package_version(Path(args.pyproject))
        if package_version != version:
            raise ValueError(
                f"Version mismatch: pyproject.toml is {package_version}, tag is {version}."
            )
        _, notes = _read_changelog_section(Path(args.changelog), version)
        if args.github_output:
            _append_github_output(Path(args.github_output), version, notes)
    except ValueError as exc:
        print(str(exc), file=sys.stderr)
        return 1

    print(f"Release metadata OK for v{version}.")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
