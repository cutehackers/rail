from __future__ import annotations

import sys
from collections.abc import Sequence
from pathlib import Path

_HARNESS_DEFAULT_SUBDIRS = ("actors", "rules", "rubrics", "supervisor", "templates")


def find_alignment_drift(pairs: Sequence[tuple[Path, Path]]) -> list[str]:
    drift: list[str] = []
    for source, packaged in pairs:
        source_files = _relative_files(source)
        packaged_files = _relative_files(packaged)
        for missing in sorted(source_files - packaged_files):
            drift.append(f"{source} != {packaged}: missing packaged file {missing.as_posix()}")
        for extra in sorted(packaged_files - source_files):
            drift.append(f"{source} != {packaged}: extra packaged file {extra.as_posix()}")
        for relative_path in sorted(source_files & packaged_files):
            if (source / relative_path).read_bytes() != (packaged / relative_path).read_bytes():
                drift.append(f"{source} != {packaged}: content drift {relative_path.as_posix()}")
    return drift


def default_alignment_pairs(repo_root: Path) -> list[tuple[Path, Path]]:
    pairs: list[tuple[Path, Path]] = [
        (repo_root / "skills" / "rail", repo_root / "assets" / "skill" / "Rail"),
        (repo_root / "assets" / "defaults", repo_root / "src" / "rail" / "package_assets" / "defaults"),
        (repo_root / "assets" / "skill" / "Rail", repo_root / "src" / "rail" / "package_assets" / "skill" / "Rail"),
    ]
    pairs.extend(
        (repo_root / ".harness" / subdir, repo_root / "assets" / "defaults" / subdir)
        for subdir in _HARNESS_DEFAULT_SUBDIRS
    )
    return pairs


def main(argv: Sequence[str] | None = None) -> int:
    args = list(argv if argv is not None else sys.argv[1:])
    repo_root = Path(args[0]).resolve() if args else Path(__file__).resolve().parents[1]
    drift = find_alignment_drift(default_alignment_pairs(repo_root))
    if drift:
        for item in drift:
            print(item, file=sys.stderr)
        return 1
    print("Package asset alignment passed.")
    return 0


def _relative_files(root: Path) -> set[Path]:
    if not root.is_dir():
        return set()
    return {path.relative_to(root) for path in root.rglob("*") if path.is_file()}


if __name__ == "__main__":
    raise SystemExit(main())
