package services

import (
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
		TargetPath:      "docs/agentic/policies/sync-policy.md",
		DBChecksum:      "abc123",
	}})

	for _, expected := range []string{
		`asset_id: "project-asset-1"`,
		`asset_version_id: "project-version-1"`,
		`template_asset_id: "template-asset-1"`,
		`checksum: "abc123"`,
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
