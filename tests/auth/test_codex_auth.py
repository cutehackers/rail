from __future__ import annotations

import os

import pytest

from rail.auth.credentials import codex_auth_home, validate_codex_auth_material


def test_codex_auth_home_defaults_to_rail_owned_location(tmp_path, monkeypatch):
    monkeypatch.setenv("RAIL_HOME", str(tmp_path / "rail-home"))

    assert codex_auth_home(environ=os.environ) == tmp_path / "rail-home" / "codex"


def test_auth_material_allowlist_rejects_unknown_files(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    (auth_home / "skills").mkdir()

    with pytest.raises(ValueError, match="unknown auth material"):
        validate_codex_auth_material(auth_home)


def test_auth_material_rejects_group_or_world_writable_auth_file(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    auth_file = auth_home / "auth.json"
    auth_file.write_text("{}", encoding="utf-8")
    auth_file.chmod(0o666)

    with pytest.raises(ValueError, match="unsafe auth material permissions"):
        validate_codex_auth_material(auth_home)


def test_auth_material_rejects_group_or_world_writable_auth_home(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    (auth_home / "auth.json").write_text("{}", encoding="utf-8")
    auth_home.chmod(0o777)

    try:
        with pytest.raises(ValueError, match="unsafe auth home permissions"):
            validate_codex_auth_material(auth_home)
    finally:
        auth_home.chmod(0o755)


def test_auth_material_rejects_symlinked_auth_home(tmp_path):
    real_home = tmp_path / "real-auth"
    real_home.mkdir()
    (real_home / "auth.json").write_text("{}", encoding="utf-8")
    auth_home = tmp_path / "auth-link"
    auth_home.symlink_to(real_home, target_is_directory=True)

    with pytest.raises(ValueError, match="symlink"):
        validate_codex_auth_material(auth_home)


def test_auth_material_rejects_non_directory_auth_home(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.write_text("not a directory", encoding="utf-8")

    with pytest.raises(ValueError, match="auth home"):
        validate_codex_auth_material(auth_home)


def test_auth_material_rejects_missing_auth_json(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()

    with pytest.raises(ValueError, match="missing auth.json"):
        validate_codex_auth_material(auth_home)


def test_auth_material_rejects_symlinked_auth_file(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    real_auth = tmp_path / "outside-auth.json"
    real_auth.write_text("{}", encoding="utf-8")
    (auth_home / "auth.json").symlink_to(real_auth)

    with pytest.raises(ValueError, match="symlink"):
        validate_codex_auth_material(auth_home)


def test_valid_auth_material_is_accepted(tmp_path):
    auth_home = tmp_path / "auth"
    auth_home.mkdir()
    auth_file = auth_home / "auth.json"
    auth_file.write_text("{}", encoding="utf-8")
    auth_file.chmod(0o600)

    assert validate_codex_auth_material(auth_home) == [auth_file]
