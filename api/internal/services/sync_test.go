package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agentops-workspace/api/internal/domain"
)

func TestBuildAssetsLockIncludesProjectAssetVersionAndTemplateAsset(t *testing.T) {
	project := domain.Project{ID: "project-1", Slug: "demo"}
	lock := buildAssetsLock(project, []SyncItem{{
		AssetID:         "project-asset-1",
		AssetVersionID:  "project-version-1",
		TemplateAssetID: "template-asset-1",
		AssetType:       "policy_doc",
		AssetSlug:       "sync-policy",
		Version:         "1.2.3",
		TargetPath:      ".codex/policies/sync-policy.md",
		DBChecksum:      "abc123",
	}})

	for _, expected := range []string{
		`"asset_id": "project-asset-1"`,
		`"asset_version_id": "project-version-1"`,
		`"template_asset_id": "template-asset-1"`,
		`"checksum": "abc123"`,
	} {
		if !strings.Contains(lock, expected) {
			t.Fatalf("lock missing %q:\n%s", expected, lock)
		}
	}
}

func TestRenderManagedFileIncludesTemplateAssetID(t *testing.T) {
	rendered := renderManagedFileWithMetadata(
		domain.Asset{ID: "project-asset-1", Type: "policy_doc", Slug: "sync-policy"},
		domain.AssetVersion{ID: "project-version-1", Version: "1.2.3", ContentFormat: "markdown", Checksum: "abc123", Content: "# Body\n"},
		"sync-policy",
		"template-asset-1",
	)

	for _, expected := range []string{
		`asset_id: "project-asset-1"`,
		`asset_version_id: "project-version-1"`,
		`template_asset_id: "template-asset-1"`,
		`# Body`,
	} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("rendered file missing %q:\n%s", expected, rendered)
		}
	}
}

func TestProjectManifestIncludesMCPHintWithoutToken(t *testing.T) {
	project := domain.Project{ID: "project-1", WorkspaceID: "workspace-1", Slug: "demo", Name: "Demo", RepoPath: "/repo/demo", DefaultBranch: "main"}
	manifest := (SyncService{MCPPublicURL: "http://agentops.test/mcp"}).ProjectManifest(project)
	for _, expected := range []string{
		`"endpoint": "http://agentops.test/mcp"`,
		`"token_env": "AGENTOPS_MCP_TOKEN"`,
		`"auth": "bearer_token_env"`,
	} {
		if !strings.Contains(manifest, expected) {
			t.Fatalf("manifest missing %q:\n%s", expected, manifest)
		}
	}
	if strings.Contains(manifest, "AGENTOPS_MCP_TOKEN=") {
		t.Fatalf("manifest appears to contain a token value:\n%s", manifest)
	}
}

func TestClassifyPrivateCodexContextFiles(t *testing.T) {
	tests := []struct {
		path string
		typ  string
		slug string
	}{
		{".codex/agents/reviewer.toml", "subagent_doc", "reviewer"},
		{".codex/skills/backend/SKILL.md", "skill_doc", "backend"},
		{".codex/policies/db-source-of-truth.md", "policy_doc", "db-source-of-truth"},
		{"openai.yaml", "context_doc", "openai"},
		{"docs/agentic/openai.yml", "context_doc", "docs-agentic-openai"},
		{"docs/architecture.md", "context_doc", "docs-architecture"},
	}
	for _, tt := range tests {
		typ, slug := classifyManagedFile(tt.path)
		if typ != tt.typ || slug != tt.slug {
			t.Fatalf("classifyManagedFile(%q) = %s/%s, want %s/%s", tt.path, typ, slug, tt.typ, tt.slug)
		}
	}
}

func TestScanManagedRepoFilesIncludesOpenAIConfig(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "openai.yaml"), []byte("model: gpt-5.2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "target"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "target", "openai.yaml"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".agentops"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".agentops", "openai.yaml"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".codex", "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".codex", "scripts", "setup.md"), []byte("ignored"), 0644); err != nil {
		t.Fatal(err)
	}
	files, err := scanManagedRepoFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !slicesContains(files, "openai.yaml") {
		t.Fatalf("expected openai.yaml in managed repo files: %#v", files)
	}
	if slicesContains(files, "target/openai.yaml") {
		t.Fatalf("expected ignored directory file to be skipped: %#v", files)
	}
	if slicesContains(files, ".agentops/openai.yaml") || slicesContains(files, ".codex/scripts/setup.md") {
		t.Fatalf("expected unsupported agent directories to be skipped: %#v", files)
	}
}

func slicesContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func TestNormalizeActorDefaultsAndPreservesExplicitValues(t *testing.T) {
	got := normalizeActor(SyncRequest{}, "operator", "http_api")
	if got.ActorType != "operator" || got.Transport != "http_api" {
		t.Fatalf("unexpected default actor metadata: %#v", got)
	}

	got = normalizeActor(SyncRequest{ActorType: "project_repo_agent", ActorName: "codex", Transport: "mcp"}, "operator", "http_api")
	if got.ActorType != "project_repo_agent" || got.ActorName != "codex" || got.Transport != "mcp" {
		t.Fatalf("unexpected explicit actor metadata: %#v", got)
	}
}
