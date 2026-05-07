package services

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type PathGuard struct {
}

var ErrUnsafePath = errors.New("unsafe path")

func NewPathGuard() PathGuard {
	return PathGuard{}
}

func (g PathGuard) ValidateRepoPath(path string) (string, error) {
	return g.ValidateRepoPathWithCreate(path, false)
}

func (g PathGuard) ValidateRepoPathWithCreate(path string, createMissing bool) (string, error) {
	if strings.TrimSpace(path) == "" || strings.Contains(path, "\x00") {
		return "", ErrUnsafePath
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		if !createMissing || !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err := os.MkdirAll(filepath.Clean(abs), 0755); err != nil {
			return "", err
		}
		real, err = filepath.EvalSymlinks(abs)
		if err != nil {
			return "", err
		}
	}
	info, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", errors.New("repo path is not a directory")
	}
	f, err := os.Open(real)
	if err != nil {
		return "", errors.New("repo path is not readable")
	}
	_ = f.Close()
	return real, nil
}

func (g PathGuard) SafeTarget(repoPath, targetPath string) (string, error) {
	return g.safeTarget(repoPath, targetPath, true)
}

func (g PathGuard) SafeTargetForRead(repoPath, targetPath string) (string, error) {
	return g.safeTarget(repoPath, targetPath, false)
}

func (g PathGuard) CanWriteTarget(targetPath string) bool {
	return isAllowedGeneratedTarget(filepath.ToSlash(filepath.Clean(targetPath)))
}

func (g PathGuard) safeTarget(repoPath, targetPath string, createParent bool) (string, error) {
	if strings.Contains(targetPath, "\x00") || filepath.IsAbs(targetPath) || hasTraversal(targetPath) {
		return "", ErrUnsafePath
	}
	cleanTarget := filepath.ToSlash(filepath.Clean(targetPath))
	if createParent && !isAllowedGeneratedTarget(cleanTarget) {
		return "", ErrUnsafePath
	}
	if !createParent && !isAllowedReadableTarget(cleanTarget) {
		return "", ErrUnsafePath
	}
	repoReal, err := filepath.EvalSymlinks(repoPath)
	if err != nil {
		return "", err
	}
	full := filepath.Join(repoReal, filepath.FromSlash(cleanTarget))
	parent := filepath.Dir(full)
	if createParent {
		if err := os.MkdirAll(parent, 0755); err != nil {
			return "", err
		}
	}
	parentReal := parent
	if real, err := filepath.EvalSymlinks(parent); err == nil {
		parentReal = real
	}
	if !strings.HasPrefix(parentReal+string(os.PathSeparator), repoReal+string(os.PathSeparator)) && parentReal != repoReal {
		return "", ErrUnsafePath
	}
	if info, err := os.Lstat(full); err == nil && info.Mode()&os.ModeSymlink != 0 {
		targetReal, err := filepath.EvalSymlinks(full)
		if err != nil {
			return "", err
		}
		if !insidePath(repoReal, targetReal) {
			return "", ErrUnsafePath
		}
	}
	return full, nil
}

func hasTraversal(path string) bool {
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if part == ".." {
			return true
		}
	}
	return false
}

func insidePath(root, path string) bool {
	cleanRoot := filepath.Clean(root)
	cleanPath := filepath.Clean(path)
	return cleanPath == cleanRoot || strings.HasPrefix(cleanPath+string(os.PathSeparator), cleanRoot+string(os.PathSeparator))
}

func isAllowedReadableTarget(path string) bool {
	return isAllowedGeneratedTarget(path) || isManagedTreeFile(path)
}

func isAllowedGeneratedTarget(path string) bool {
	switch {
	case path == "AGENTS.md", path == "README.md":
		return true
	case strings.HasPrefix(path, ".codex/"):
		return true
	default:
		return false
	}
}
