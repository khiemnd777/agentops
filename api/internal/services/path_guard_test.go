package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathGuardAllowsOnlyRootsAndGeneratedTargets(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	if err := os.Mkdir(repo, 0755); err != nil {
		t.Fatal(err)
	}
	guard := NewPathGuard()
	if _, err := guard.ValidateRepoPath(repo); err != nil {
		t.Fatalf("expected repo to validate: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".codex-write-test")); !os.IsNotExist(err) {
		t.Fatal("expected repo validation to avoid write probes")
	}
	if _, err := guard.SafeTarget(repo, ".codex/skills/repo-architect/SKILL.md"); err != nil {
		t.Fatalf("expected generated target to validate: %v", err)
	}
	if _, err := guard.SafeTargetForRead(repo, "DESIGN.md"); err != nil {
		t.Fatalf("expected managed markdown target to validate for read: %v", err)
	}
	if _, err := guard.SafeTarget(repo, "DESIGN.md"); err == nil {
		t.Fatal("expected managed markdown target write to be rejected")
	}
	if _, err := guard.SafeTarget(repo, "src/main.go"); err == nil {
		t.Fatal("expected src write to be rejected")
	}
	if _, err := guard.SafeTarget(repo, "../escape.md"); err == nil {
		t.Fatal("expected traversal to be rejected")
	}
}

func TestPathGuardRejectsInvalidRepoPaths(t *testing.T) {
	guard := NewPathGuard()
	if _, err := guard.ValidateRepoPath(""); err == nil {
		t.Fatal("expected empty repo path to be rejected")
	}
	if _, err := guard.ValidateRepoPath("bad\x00path"); err == nil {
		t.Fatal("expected NUL repo path to be rejected")
	}
	file := filepath.Join(t.TempDir(), "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := guard.ValidateRepoPath(file); err == nil {
		t.Fatal("expected non-directory repo path to be rejected")
	}
}

func TestPathGuardCanCreateMissingRepoPath(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "missing", "repo")
	guard := NewPathGuard()

	if _, err := guard.ValidateRepoPath(repo); !os.IsNotExist(err) {
		t.Fatalf("expected missing repo path without create to fail with not-exist error, got %v", err)
	}

	real, err := guard.ValidateRepoPathWithCreate(repo, true)
	if err != nil {
		t.Fatalf("expected missing repo path to be created: %v", err)
	}
	expectedReal, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("expected created repo path to resolve: %v", err)
	}
	if real != expectedReal {
		t.Fatalf("expected created repo path %q, got %q", expectedReal, real)
	}
	info, err := os.Stat(repo)
	if err != nil {
		t.Fatalf("expected created repo path to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("expected created repo path to be a directory")
	}
}

func TestPathGuardRejectsSymlinkEscapeTargets(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(repo, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, "AGENTS.md")); err != nil {
		t.Skipf("symlink creation is not available: %v", err)
	}
	if _, err := NewPathGuard().SafeTarget(repo, "AGENTS.md"); err == nil {
		t.Fatal("expected symlink target outside repo to be rejected")
	}
}
