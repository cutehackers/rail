from __future__ import annotations

import ast
from pathlib import Path


PYTHON_ROOTS = [Path("src"), Path("tests")]
FORBIDDEN_MODULE_TOKENS = {"utils", "util", "helpers", "helper", "common", "base", "manager"}
FORBIDDEN_CLASS_NAMES = {"Util", "Helper", "Manager"}
FORBIDDEN_CLASS_SUFFIXES = ("Util", "Helper", "Manager")


def test_python_modules_use_specific_responsibility_names():
    findings: list[str] = []
    for path in _python_files():
        if path.name == "__init__.py":
            continue
        tokens = set(path.stem.split("_"))
        forbidden = sorted(tokens & FORBIDDEN_MODULE_TOKENS)
        if forbidden:
            findings.append(f"{path}: forbidden module token(s): {', '.join(forbidden)}")

    assert findings == []


def test_project_owned_classes_use_specific_responsibility_names():
    findings: list[str] = []
    for path in _python_files():
        tree = ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
        for node in ast.walk(tree):
            if not isinstance(node, ast.ClassDef):
                continue
            if node.name in FORBIDDEN_CLASS_NAMES or node.name.endswith(FORBIDDEN_CLASS_SUFFIXES):
                findings.append(f"{path}:{node.lineno}: forbidden class name: {node.name}")

    assert findings == []


def _python_files() -> list[Path]:
    paths: list[Path] = []
    for root in PYTHON_ROOTS:
        paths.extend(
            path
            for path in root.rglob("*.py")
            if ".venv" not in path.parts and ".worktrees" not in path.parts and "__pycache__" not in path.parts
        )
    return paths

