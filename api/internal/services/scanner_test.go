package services

import (
	"os"
	"path/filepath"
	"testing"

	"agentops-workspace/api/internal/domain"
)

func TestRepoScannerTreeShowsDirectoriesAndManagedFiles(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "api"))
	mustMkdir(t, filepath.Join(root, ".git"))
	mustMkdir(t, filepath.Join(root, ".codex"))
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	tree, err := (RepoScanner{}).Tree(root, 3, true)
	if err != nil {
		t.Fatal(err)
	}
	var sawAPI, sawAgents, sawGit bool
	for _, child := range tree.Children {
		sawAPI = sawAPI || child.Path == "api"
		sawAgents = sawAgents || child.Path == "AGENTS.md"
		sawGit = sawGit || child.Path == ".git"
	}
	if !sawAPI || !sawAgents {
		t.Fatalf("expected api directory and AGENTS.md managed file: %+v", tree.Children)
	}
	if sawGit {
		t.Fatal("expected .git to be ignored")
	}
}

func TestRepoScannerTreeShowsGeneratedCodexFiles(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, ".codex", "reports", "run-report-contract.md"), "# Contract")
	mustWrite(t, filepath.Join(root, ".codex", "policies", "noah-contract-sync.md"), "# Policy")
	mustWrite(t, filepath.Join(root, ".codex", "skills", "noah-repo-architect", "SKILL.md"), "# Skill")
	mustWrite(t, filepath.Join(root, ".codex", "workflows", "fullstack-feature-workflow.workflow.yaml"), "name: fullstack")
	mustWrite(t, filepath.Join(root, ".codex", "workflows", "fullstack-feature-workflow.md"), "# Workflow")
	mustWrite(t, filepath.Join(root, ".codex", "notes.txt"), "not managed")

	tree, err := (RepoScanner{}).Tree(root, 5, true)
	if err != nil {
		t.Fatal(err)
	}
	assertTreePath(t, tree, ".codex/reports/run-report-contract.md", "managed_file")
	assertTreePath(t, tree, ".codex/policies/noah-contract-sync.md", "managed_file")
	assertTreePath(t, tree, ".codex/skills/noah-repo-architect/SKILL.md", "managed_file")
	assertTreePath(t, tree, ".codex/workflows/fullstack-feature-workflow.workflow.yaml", "managed_file")
	assertTreePath(t, tree, ".codex/workflows/fullstack-feature-workflow.md", "managed_file")
	if hasTreePath(tree, ".codex/notes.txt", "managed_file") {
		t.Fatal("expected unrelated .codex files to stay hidden")
	}
}

func TestRepoScannerDetectsAgentRelatedFilesAndDirectories(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "agents"))
	mustMkdir(t, filepath.Join(root, ".codex", "agents"))
	mustMkdir(t, filepath.Join(root, ".codex", "skills", "backend"))
	mustMkdir(t, filepath.Join(root, "node_modules", "pkg"))
	mustWrite(t, filepath.Join(root, "AGENTS.md"), "agent instructions")
	mustWrite(t, filepath.Join(root, "DESIGN.md"), "system design")
	mustWrite(t, filepath.Join(root, "agents", "plan.md"), "local workflow")
	mustWrite(t, filepath.Join(root, ".codex", "config.toml"), "[agents]\nmax_threads = 6")
	mustWrite(t, filepath.Join(root, ".codex", "agents", "reviewer.toml"), `name = "reviewer"`)
	mustWrite(t, filepath.Join(root, ".codex", "skills", "backend", "SKILL.md"), "skill")
	mustWrite(t, filepath.Join(root, "node_modules", "pkg", "AGENTS.md"), "ignored")
	result, err := (RepoScanner{}).Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	assertCandidate(t, result, "AGENTS.md", "agents_file")
	assertCandidate(t, result, "DESIGN.md", "design_file")
	assertCandidate(t, result, "agents", "agents_directory")
	assertCandidate(t, result, ".codex", "dot_codex_directory")
	assertCandidate(t, result, "agents/plan.md", "agents_directory_file")
	assertCandidate(t, result, ".codex/config.toml", "dot_codex_directory_file")
	assertCandidate(t, result, ".codex/agents/reviewer.toml", "dot_codex_directory_file")
	assertCandidate(t, result, ".codex/skills/backend/SKILL.md", "dot_codex_directory_file")
	if result.Private == nil || !result.Private.Exists {
		t.Fatalf("expected private .codex scan section: %#v", result.Private)
	}
	assertCandidate(t, domain.RepoScanResult{Candidates: result.Private.Candidates}, "agents/reviewer.toml", "agents_directory_file")
	if hasCandidate(result, "node_modules/pkg/AGENTS.md", "agents_file") {
		t.Fatal("expected ignored directories to be skipped")
	}
}

func TestRepoScannerDetectsAgentReadmesAndMarkdownByKeyword(t *testing.T) {
	root := t.TempDir()
	mustWrite(t, filepath.Join(root, "README.md"), "How to use Codex agents in this repo.")
	mustWrite(t, filepath.Join(root, "docs.md"), "This document mentions an assistant workflow.")
	mustWrite(t, filepath.Join(root, "plain.md"), "General product notes.")
	result, err := (RepoScanner{}).Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	assertCandidate(t, result, "README.md", "agent_readme")
	assertCandidate(t, result, "docs.md", "agent_markdown")
	assertCandidate(t, result, "plain.md", "agent_markdown")
}

func TestRepoScannerDetectIsReadOnlyAndReportsRunReports(t *testing.T) {
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, ".codex", "reports", "runs", "run-1"))
	mustWrite(t, filepath.Join(root, ".codex", "reports", "runs", "run-1", "run.report.json"), `{"run_id":"run-1"}`)
	result, err := (RepoScanner{}).Detect(root)
	if err != nil {
		t.Fatal(err)
	}
	assertCandidate(t, result, ".codex/reports/runs/run-1/run.report.json", "run_report")
	if _, err := os.Stat(filepath.Join(root, ".codex-scan-write-test")); !os.IsNotExist(err) {
		t.Fatal("expected scanner to avoid write probes")
	}
}

func TestManagedFileClassificationAndHeaderStrip(t *testing.T) {
	typ, slug := classifyManagedFile(".codex/skills/repo-architect/SKILL.md")
	if typ != "skill_doc" || slug != "repo-architect" {
		t.Fatalf("unexpected classification: %s %s", typ, slug)
	}
	content := "<!--\ncodex:\n  generated: true\n-->\n\n# Skill\n"
	if got := stripAgentOpsHeader(content); got != "# Skill" {
		t.Fatalf("unexpected stripped content: %q", got)
	}
	header := parseAgentOpsHeader("<!--\ncodex:\n  asset_slug: \"repo-architect\"\n  asset_type: \"skill_doc\"\n-->\n\n# Skill")
	if header["asset_slug"] != "repo-architect" || header["asset_type"] != "skill_doc" {
		t.Fatalf("unexpected parsed header: %#v", header)
	}
}

func TestDiffSummaryCountsChangedAddedRemovedLines(t *testing.T) {
	got := DiffSummary("a\nb\nc", "a\nB\nc\nd")
	if got != "1 changed, 1 added, 0 removed lines" {
		t.Fatalf("unexpected diff summary: %s", got)
	}
	preview := DiffPreview("a\nb", "a\nB\nc", 4)
	if len(preview) != 3 {
		t.Fatalf("unexpected diff preview: %#v", preview)
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func assertCandidate(t *testing.T, result domain.RepoScanResult, path string, kind string) {
	t.Helper()
	for _, candidate := range result.Candidates {
		if candidate.Path == path && candidate.Kind == kind {
			return
		}
	}
	t.Fatalf("expected candidate %s/%s in %#v", path, kind, result.Candidates)
}

func hasCandidate(result domain.RepoScanResult, path string, kind string) bool {
	for _, candidate := range result.Candidates {
		if candidate.Path == path && candidate.Kind == kind {
			return true
		}
	}
	return false
}

func assertTreePath(t *testing.T, root domain.RepoTreeNode, path string, typ string) {
	t.Helper()
	if !hasTreePath(root, path, typ) {
		t.Fatalf("expected tree path %s/%s in %#v", path, typ, root)
	}
}

func hasTreePath(root domain.RepoTreeNode, path string, typ string) bool {
	if root.Path == path && root.Type == typ {
		return true
	}
	for _, child := range root.Children {
		if hasTreePath(child, path, typ) {
			return true
		}
	}
	return false
}
