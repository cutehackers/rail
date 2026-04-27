//go:build darwin || linux || freebsd || netbsd || openbsd

package auth

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

func platformCurrentUID() (uint32, error) {
	value, err := user.Current()
	if err != nil {
		return 0, fmt.Errorf("resolve current user: %w", err)
	}
	uid, err := strconv.ParseUint(value.Uid, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse current user UID: %w", err)
	}
	return uint32(uid), nil
}

func ensurePlatformSupported() error {
	return nil
}

func ownerUID(info os.FileInfo) (uint32, bool) {
	value, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint32(value.Uid), true
}

func copyPrivateRegularFile(sourceHome string, destinationHome string, name string) error {
	sourceDir, err := openPrivateDirectory(sourceHome, "codex auth home")
	if err != nil {
		return err
	}
	defer sourceDir.Close()
	destinationDir, err := openPrivateDirectory(destinationHome, "actor codex home")
	if err != nil {
		return err
	}
	defer destinationDir.Close()

	sourceFile, err := openRelativeNoFollow(sourceDir, name, unix.O_RDONLY|unix.O_CLOEXEC, 0, false)
	if err != nil {
		return err
	}
	defer sourceFile.Close()
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return fmt.Errorf("inspect auth material: %w", err)
	}
	if !sourceInfo.Mode().IsRegular() {
		return fmt.Errorf("auth material must be a regular file: %s", filepath.Join(sourceHome, name))
	}
	if sourceInfo.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("auth material permissions must be 0600 or stricter: %s", filepath.Join(sourceHome, name))
	}

	if err := rejectExistingRelativeAuthFile(destinationDir, destinationHome, name); err != nil {
		return err
	}
	destinationFile, err := openRelativeNoFollow(destinationDir, name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC, 0o600, true)
	if err != nil {
		return err
	}
	defer destinationFile.Close()
	if _, err := io.Copy(destinationFile, sourceFile); err != nil {
		return fmt.Errorf("write actor auth material: %w", err)
	}
	if err := destinationFile.Chmod(0o600); err != nil {
		return fmt.Errorf("chmod actor auth material: %w", err)
	}
	return nil
}

func openPrivateDirectory(path string, label string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		if err == unix.ELOOP {
			return nil, fmt.Errorf("%s must not be a symlink: %s", label, path)
		}
		return nil, fmt.Errorf("open %s: %w", label, err)
	}
	file := os.NewFile(uintptr(fd), path)
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("inspect %s: %w", label, err)
	}
	if !info.IsDir() {
		_ = file.Close()
		return nil, fmt.Errorf("%s must be a directory: %s", label, path)
	}
	if info.Mode().Perm()&0o077 != 0 {
		_ = file.Close()
		return nil, fmt.Errorf("%s permissions must be 0700 or stricter: %s", label, path)
	}
	return file, nil
}

func openRelativeNoFollow(dir *os.File, name string, flags int, perm uint32, destination bool) (*os.File, error) {
	fd, err := unix.Openat(int(dir.Fd()), name, flags|unix.O_NOFOLLOW, perm)
	if err != nil {
		if err == unix.ELOOP {
			if destination {
				return nil, fmt.Errorf("destination auth material must not be a symlink: %s", filepath.Join(dir.Name(), name))
			}
			return nil, fmt.Errorf("auth material must not be a symlink: %s", filepath.Join(dir.Name(), name))
		}
		if destination {
			return nil, fmt.Errorf("write actor auth material: %w", err)
		}
		return nil, err
	}
	return os.NewFile(uintptr(fd), filepath.Join(dir.Name(), name)), nil
}

func rejectExistingRelativeAuthFile(dir *os.File, dirPath string, name string) error {
	var stat unix.Stat_t
	err := unix.Fstatat(int(dir.Fd()), name, &stat, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		if err == unix.ENOENT {
			return nil
		}
		return fmt.Errorf("inspect destination auth material: %w", err)
	}
	mode := uint32(stat.Mode) & unix.S_IFMT
	path := filepath.Join(dirPath, name)
	switch mode {
	case unix.S_IFLNK:
		return fmt.Errorf("destination auth material must not be a symlink: %s", path)
	case unix.S_IFDIR:
		return fmt.Errorf("destination auth material must not be a directory: %s", path)
	default:
		return fmt.Errorf("destination auth material already exists: %s", path)
	}
}
