from __future__ import annotations

from importlib.resources import files
from typing import Any

import yaml


def read_default_asset_text(relative_path: str) -> str:
    return _default_asset(relative_path).read_text(encoding="utf-8")


def load_default_asset_yaml(relative_path: str) -> dict[str, Any]:
    payload = yaml.safe_load(read_default_asset_text(relative_path)) or {}
    if not isinstance(payload, dict):
        raise ValueError(f"default asset must be a mapping: {relative_path}")
    return payload


def _default_asset(relative_path: str):
    if relative_path.startswith("/") or ".." in relative_path.split("/"):
        raise ValueError(f"unsafe default asset path: {relative_path}")
    return files("rail").joinpath("package_assets", relative_path)
