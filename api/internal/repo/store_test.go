package repo

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentdb "agentops-workspace/api/internal/db"
	"agentops-workspace/api/internal/domain"
)

func TestApplyAssetPresetToProjectCopiesPinnedTemplateAndSkipsExisting(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	pool, err := agentdb.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := agentdb.Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	store := New(pool)
	ws, err := store.EnsureDefaultWorkspace(ctx)
	if err != nil {
		t.Fatal(err)
	}

	suffix := time.Now().UTC().Format("20060102150405000000")
	project, err := store.CreateProject(ctx, domain.Project{
		WorkspaceID:      ws.ID,
		Name:             "Preset Apply " + suffix,
		Slug:             "preset-apply-" + suffix,
		RepoPath:         filepath.Join(t.TempDir(), "repo"),
		DefaultBranch:    "main",
		Description:      "preset apply test",
		LastAutoImportAt: nil,
	})
	if err != nil {
		t.Fatal(err)
	}

	slug := "preset-skill-" + suffix
	templateAsset, templateVersion, err := store.UpsertAssetWithVersion(ctx, ws.ID, "skill_doc", slug, "Preset Skill", "Pinned template", "1.2.3", "content v1", "markdown", "published", "checksum-v1", []string{"preset"})
	if err != nil {
		t.Fatal(err)
	}
	if templateAsset.Scope != "template" {
		t.Fatalf("expected template scope, got %q", templateAsset.Scope)
	}

	var presetID, presetItemID string
	if err := pool.QueryRow(ctx, `
		INSERT INTO asset_presets(workspace_id,slug,name,description,tags)
		VALUES($1,$2,$3,$4,'["test"]'::jsonb)
		RETURNING id`, ws.ID, "preset-"+suffix, "Preset "+suffix, "Test preset").Scan(&presetID); err != nil {
		t.Fatal(err)
	}
	targetPath := "docs/agentic/skills/" + slug + ".md"
	if err := pool.QueryRow(ctx, `
		INSERT INTO asset_preset_items(preset_id,asset_id,asset_version_id,target_path,sort_order)
		VALUES($1,$2,$3,$4,10)
		RETURNING id`, presetID, templateAsset.ID, templateVersion.ID, targetPath).Scan(&presetItemID); err != nil {
		t.Fatal(err)
	}

	presets, err := store.ListAssetPresets(ctx, ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	foundPreset := false
	for _, preset := range presets {
		if preset.ID == presetID {
			foundPreset = true
			if preset.ItemCount != 1 {
				t.Fatalf("expected preset item_count 1, got %d", preset.ItemCount)
			}
		}
	}
	if !foundPreset {
		t.Fatal("expected preset in list")
	}
	detail, err := store.GetAssetPreset(ctx, ws.ID, presetID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Items) != 1 || detail.Items[0].ID != presetItemID || detail.Items[0].Version.ID != templateVersion.ID {
		t.Fatalf("unexpected preset detail items: %#v", detail.Items)
	}

	applied, err := store.ApplyAssetPresetToProject(ctx, ws.ID, project.ID, presetID)
	if err != nil {
		t.Fatal(err)
	}
	if applied.Created != 1 || applied.Skipped != 0 || len(applied.Items) != 1 || applied.Items[0].Status != "created" {
		t.Fatalf("unexpected first apply result: %#v", applied)
	}
	if applied.Items[0].ProjectAssetID == templateAsset.ID || applied.Items[0].ProjectAssetVersionID == templateVersion.ID {
		t.Fatalf("expected copied project asset/version ids, got %#v", applied.Items[0])
	}

	projectAssets, err := store.ListProjectAssets(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(projectAssets) != 1 {
		t.Fatalf("expected one project-owned asset, got %d", len(projectAssets))
	}
	projectAsset := projectAssets[0]
	if projectAsset.Scope != "project" || projectAsset.ProjectID == nil || *projectAsset.ProjectID != project.ID || projectAsset.TemplateAssetID == nil || *projectAsset.TemplateAssetID != templateAsset.ID {
		t.Fatalf("unexpected project asset scope/source: %#v", projectAsset)
	}

	defaultAssets, err := store.ListAssets(ctx, ws.ID, slug, "skill_doc")
	if err != nil {
		t.Fatal(err)
	}
	for _, asset := range defaultAssets {
		if asset.Scope != "template" {
			t.Fatalf("default ListAssets leaked non-template asset: %#v", asset)
		}
	}
	filteredProjectAssets, err := store.ListAssets(ctx, ws.ID, slug, "skill_doc", domain.AssetListFilter{Scope: "project", ProjectID: project.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(filteredProjectAssets) != 1 || filteredProjectAssets[0].ID != projectAsset.ID {
		t.Fatalf("unexpected project-scoped ListAssets result: %#v", filteredProjectAssets)
	}

	currentProjectVersions, err := store.CurrentProjectAssetVersions(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(currentProjectVersions) != 1 || currentProjectVersions[0].Version.Content != "content v1" {
		t.Fatalf("unexpected current project versions: %#v", currentProjectVersions)
	}

	var bindingStatus string
	if err := pool.QueryRow(ctx, `SELECT sync_status FROM project_asset_bindings WHERE project_id=$1 AND asset_id=$2 AND target_path=$3`, project.ID, projectAsset.ID, targetPath).Scan(&bindingStatus); err != nil {
		t.Fatal(err)
	}
	if bindingStatus != "missing" {
		t.Fatalf("expected missing binding status, got %q", bindingStatus)
	}

	appliedAgain, err := store.ApplyAssetPresetToProject(ctx, ws.ID, project.ID, presetID)
	if err != nil {
		t.Fatal(err)
	}
	if appliedAgain.Created != 0 || appliedAgain.Skipped != 1 || len(appliedAgain.Items) != 1 || appliedAgain.Items[0].Status != "already_exists" || appliedAgain.Items[0].ExistingAssetID != projectAsset.ID {
		t.Fatalf("unexpected second apply result: %#v", appliedAgain)
	}
	var versionCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agentic_asset_versions WHERE asset_id=$1`, projectAsset.ID).Scan(&versionCount); err != nil {
		t.Fatal(err)
	}
	if versionCount != 1 {
		t.Fatalf("expected no duplicate project asset version on re-apply, got %d", versionCount)
	}
}
