from __future__ import annotations

import errno
import os
import stat
import uuid
from pathlib import Path

from rail.workspace.isolation import is_hardlink, tree_digest
from rail.workspace.patch_bundle import PatchBundle, PatchValidationPolicy, validate_patch_bundle


def apply_patch_bundle(bundle: PatchBundle, target_root: Path, *, policy: PatchValidationPolicy | None = None) -> None:
    target_root = target_root.resolve(strict=True)
    validate_patch_bundle(bundle, policy=policy)
    if tree_digest(target_root) != bundle.base_tree_digest:
        raise ValueError("target base tree digest does not match patch bundle")

    prepared: list[tuple[str, str | None, int | None, str, bool]] = []
    for operation in bundle.operations:
        target_path = _safe_target_path(target_root, operation.path)
        if not target_path.is_relative_to(target_root):
            raise ValueError("patch operation escapes target root")
        if is_hardlink(target_path):
            raise ValueError("patch operation targets a hardlink")
        old_content, old_mode = _read_text_without_following_links(target_root, operation.path)
        prepared.append((operation.path, old_content, old_mode, operation.content, operation.executable))

    if tree_digest(target_root) != bundle.base_tree_digest:
        raise ValueError("target base tree changed during patch apply")

    applied: list[tuple[str, str | None, int | None, str]] = []
    created_dirs: list[tuple[str, ...]] = []
    try:
        for relative_path, old_content, old_mode, new_content, executable in prepared:
            _write_text_without_following_links(
                target_root,
                relative_path,
                new_content,
                expected_old_content=old_content,
                mode=_replacement_mode(old_mode, executable=executable),
                created_dirs=created_dirs,
            )
            applied.append((relative_path, old_content, old_mode, new_content))
    except Exception:
        for relative_path, old_content, old_mode, new_content in reversed(applied):
            if old_content is None:
                _unlink_without_following_links(target_root, relative_path)
            else:
                _write_text_without_following_links(
                    target_root,
                    relative_path,
                    old_content,
                    expected_old_content=new_content,
                    mode=old_mode,
                    created_dirs=created_dirs,
                )
        _remove_created_dirs(target_root, created_dirs)
        raise


def _safe_target_path(target_root: Path, relative_path: str) -> Path:
    current = target_root
    for part in Path(relative_path).parts:
        current = current / part
        if current.is_symlink():
            raise ValueError("patch operation targets a symlink")
    return current.resolve(strict=False)


def _read_text_without_following_links(target_root: Path, relative_path: str) -> tuple[str | None, int | None]:
    parts = Path(relative_path).parts
    parent_fd = _open_parent_dir(target_root, parts, create=False)
    if parent_fd is None:
        return None, None
    try:
        try:
            fd = _open_final(parts[-1], os.O_RDONLY, dir_fd=parent_fd)
        except FileNotFoundError:
            return None, None
        with os.fdopen(fd, "rb") as stream:
            file_stat = _assert_regular_unlinked_file(stream.fileno())
            return stream.read().decode("utf-8"), stat.S_IMODE(file_stat.st_mode)
    finally:
        os.close(parent_fd)


def _write_text_without_following_links(
    target_root: Path,
    relative_path: str,
    content: str,
    *,
    expected_old_content: str | None,
    mode: int | None,
    created_dirs: list[tuple[str, ...]],
) -> None:
    parts = Path(relative_path).parts
    parent_fd = _open_parent_dir(target_root, parts, create=True, created_dirs=created_dirs)
    if parent_fd is None:
        raise ValueError("patch operation parent is missing")
    try:
        payload = content.encode("utf-8")
        if expected_old_content is None:
            _create_from_temp_file(parent_fd, parts[-1], payload, mode=mode or 0o644)
            return
        fd = _open_final(parts[-1], os.O_RDWR, dir_fd=parent_fd)
        with os.fdopen(fd, "rb") as stream:
            _assert_regular_unlinked_file(stream.fileno())
            current = stream.read().decode("utf-8")
            if current != expected_old_content:
                raise ValueError("target base tree changed during patch apply")
        _replace_with_temp_file(parent_fd, parts[-1], payload, mode=mode or 0o644)
    except FileExistsError as exc:
        raise ValueError("target base tree changed during patch apply") from exc
    finally:
        os.close(parent_fd)


def _unlink_without_following_links(target_root: Path, relative_path: str) -> None:
    parts = Path(relative_path).parts
    parent_fd = _open_parent_dir(target_root, parts, create=False)
    if parent_fd is None:
        return
    try:
        try:
            os.unlink(parts[-1], dir_fd=parent_fd)
        except FileNotFoundError:
            pass
    finally:
        os.close(parent_fd)


def _replace_with_temp_file(parent_fd: int, final_name: str, payload: bytes, *, mode: int) -> None:
    temp_name = f".rail-tmp-{uuid.uuid4().hex}"
    fd = _open_final(temp_name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, dir_fd=parent_fd, mode=mode)
    replaced = False
    try:
        with os.fdopen(fd, "wb") as stream:
            _assert_regular_unlinked_file(stream.fileno())
            os.fchmod(stream.fileno(), mode)
            stream.write(payload)
        os.replace(temp_name, final_name, src_dir_fd=parent_fd, dst_dir_fd=parent_fd)
        replaced = True
    finally:
        if not replaced:
            try:
                os.unlink(temp_name, dir_fd=parent_fd)
            except FileNotFoundError:
                pass


def _create_from_temp_file(parent_fd: int, final_name: str, payload: bytes, *, mode: int) -> None:
    temp_name = f".rail-tmp-{uuid.uuid4().hex}"
    fd = _open_final(temp_name, os.O_WRONLY | os.O_CREAT | os.O_EXCL, dir_fd=parent_fd, mode=mode)
    linked = False
    try:
        with os.fdopen(fd, "wb") as stream:
            _assert_regular_unlinked_file(stream.fileno())
            os.fchmod(stream.fileno(), mode)
            stream.write(payload)
        os.link(temp_name, final_name, src_dir_fd=parent_fd, dst_dir_fd=parent_fd)
        linked = True
    except FileExistsError as exc:
        raise ValueError("target base tree changed during patch apply") from exc
    finally:
        try:
            os.unlink(temp_name, dir_fd=parent_fd)
        except FileNotFoundError:
            pass
        if not linked:
            pass


def _open_parent_dir(
    target_root: Path,
    parts: tuple[str, ...],
    *,
    create: bool,
    created_dirs: list[tuple[str, ...]] | None = None,
) -> int | None:
    if not parts:
        raise ValueError("patch operation path is empty")
    current_fd = os.open(target_root, os.O_RDONLY | _directory_flag())
    current_parts: list[str] = []
    try:
        for part in parts[:-1]:
            current_parts.append(part)
            try:
                next_fd = _open_final(part, os.O_RDONLY | _directory_flag(), dir_fd=current_fd)
            except FileNotFoundError:
                if not create:
                    os.close(current_fd)
                    return None
                os.mkdir(part, dir_fd=current_fd)
                if created_dirs is not None:
                    created_dirs.append(tuple(current_parts))
                next_fd = _open_final(part, os.O_RDONLY | _directory_flag(), dir_fd=current_fd)
            _assert_directory(next_fd)
            os.close(current_fd)
            current_fd = next_fd
        return current_fd
    except Exception:
        os.close(current_fd)
        raise


def _open_final(name: str, flags: int, *, dir_fd: int, mode: int = 0o666) -> int:
    try:
        return os.open(name, flags | _nofollow_flag(), mode, dir_fd=dir_fd)
    except OSError as exc:
        if exc.errno == errno.ELOOP:
            raise ValueError("patch operation targets a symlink") from exc
        raise


def _assert_regular_unlinked_file(fd: int) -> os.stat_result:
    file_stat = os.fstat(fd)
    if not stat.S_ISREG(file_stat.st_mode):
        raise ValueError("patch operation target is not a regular file")
    if file_stat.st_nlink > 1:
        raise ValueError("patch operation targets a hardlink")
    return file_stat


def _assert_directory(fd: int) -> None:
    if not stat.S_ISDIR(os.fstat(fd).st_mode):
        raise ValueError("patch operation parent is not a directory")


def _nofollow_flag() -> int:
    flag = getattr(os, "O_NOFOLLOW", None)
    if flag is None:
        raise ValueError("safe no-follow patch apply is unavailable")
    return int(flag)


def _directory_flag() -> int:
    return int(getattr(os, "O_DIRECTORY", 0))


def _replacement_mode(old_mode: int | None, *, executable: bool) -> int:
    if old_mode is not None:
        return old_mode
    return 0o755 if executable else 0o644


def _remove_created_dirs(target_root: Path, created_dirs: list[tuple[str, ...]]) -> None:
    for parts in reversed(created_dirs):
        parent_fd = _open_parent_dir(target_root, parts, create=False)
        if parent_fd is None:
            continue
        try:
            try:
                os.rmdir(parts[-1], dir_fd=parent_fd)
            except (FileNotFoundError, OSError):
                pass
        finally:
            os.close(parent_fd)
