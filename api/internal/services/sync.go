package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"agentops-workspace/api/internal/domain"
	"agentops-workspace/api/internal/repo"

	"github.com/jackc/pgx/v5"
)

type SyncService struct {
	Store        repo.Store
	Guard        PathGuard
	MCPPublicURL string
}

type SyncRequest struct {
	Mode           string   `json:"mode"`
	AssetIDs       []string `json:"asset_ids"`
	AssetTypes     []string `json:"asset_types"`
	TargetPaths    []string `json:"target_paths"`
	Preview        bool     `json:"preview"`
	ConflictAction string   `json:"conflict_action"`
	ActorType      string   `json:"actor_type"`
	ActorName      string   `json:"actor_name"`
	Transport      string   `json:"transport"`
	Actions        []struct {
		TargetPath string `json:"target_path"`
		Action     string `json:"action"`
	} `json:"actions"`
}

type SyncItem struct {
	TargetPath      string   `json:"target_path"`
	Action          string   `json:"action"`
	Status          string   `json:"status"`
	DBChecksum      string   `json:"db_checksum"`
	RepoChecksum    string   `json:"repo_checksum"`
	Message         string   `json:"message"`
	RenderedPreview string   `json:"rendered_preview,omitempty"`
	AssetType       string   `json:"asset_type,omitempty"`
	AssetSlug       string   `json:"asset_slug,omitempty"`
	AssetID         string   `json:"asset_id,omitempty"`
	AssetVersionID  string   `json:"asset_version_id,omitempty"`
	TemplateAssetID string   `json:"template_asset_id,omitempty"`
	Version         string   `json:"version,omitempty"`
	DiffSummary     string   `json:"diff_summary,omitempty"`
	DiffPreview     []string `json:"diff_preview,omitempty"`
	BackupPath      string   `json:"backup_path,omitempty"`
	AllowedActions  []string `json:"allowed_actions,omitempty"`
}

type SyncResponse struct {
	Mode  string     `json:"mode"`
	Items []SyncItem `json:"items"`
}

func (s SyncService) Preview(ctx context.Context, project domain.Project, mode string) (SyncResponse, error) {
	return s.PreviewWithRequest(ctx, project, SyncRequest{Mode: mode})
}

func (s SyncService) PreviewWithRequest(ctx context.Context, project domain.Project, req SyncRequest) (SyncResponse, error) {
	mode := strings.ToUpper(req.Mode)
	if mode == "" {
		mode = "DB_TO_REPO"
	}
	if mode == "REPO_TO_DB" {
		return s.PreviewRepoToDB(project, req)
	}
	items, err := s.plan(ctx, project, req)
	if err != nil {
		return SyncResponse{}, err
	}
	for i := range items {
		full, err := s.Guard.SafeTargetForRead(project.RepoPath, items[i].TargetPath)
		if err != nil {
			items[i].Status = "conflicted"
			items[i].Message = err.Error()
			continue
		}
		repoChecksum, err := FileChecksum(full)
		if os.IsNotExist(err) {
			items[i].Status = "missing"
			items[i].Action = "create"
		} else if err == nil {
			items[i].RepoChecksum = repoChecksum
			if repoChecksum == items[i].DBChecksum {
				items[i].Status = "synced"
				items[i].Action = "noop"
				items[i].AllowedActions = []string{"noop"}
			} else {
				items[i].Status = "drifted"
				items[i].Action = "overwrite_repo"
				items[i].AllowedActions = []string{"overwrite_repo", "backup_then_overwrite", "import_repo_as_draft", "ignore_file"}
				if data, readErr := os.ReadFile(full); readErr == nil {
					items[i].DiffSummary = DiffSummary(string(data), items[i].RenderedPreview)
					items[i].DiffPreview = DiffPreview(string(data), items[i].RenderedPreview, 10)
				}
			}
		} else {
			items[i].Status = "conflicted"
			items[i].Message = err.Error()
			items[i].AllowedActions = []string{"ignore_file"}
		}
	}
	repoManaged, _ := scanManagedRepoFiles(project.RepoPath)
	for _, repoFile := range repoManaged {
		if repoFile == ".codex/project.json" || repoFile == ".codex/sync/lock.json" {
			continue
		}
		if !plannedTarget(items, repoFile) {
			checksum, _ := FileChecksum(filepath.Join(project.RepoPath, filepath.FromSlash(repoFile)))
			items = append(items, SyncItem{TargetPath: repoFile, Action: "ignore", Status: "orphaned", RepoChecksum: checksum, Message: "Managed-looking repo file is not bound to a current DB asset.", AllowedActions: []string{"ignore_file", "import_repo_as_draft"}})
		}
	}
	return SyncResponse{Mode: mode, Items: items}, nil
}

func (s SyncService) ApplyDBToRepo(ctx context.Context, project domain.Project) (SyncResponse, error) {
	return s.ApplyDBToRepoWithRequest(ctx, project, SyncRequest{Mode: "DB_TO_REPO"})
}

func (s SyncService) ApplyDBToRepoWithRequest(ctx context.Context, project domain.Project, req SyncRequest) (SyncResponse, error) {
	req = normalizeActor(req, "operator", "http_api")
	req.Mode = "DB_TO_REPO"
	preview, err := s.PreviewWithRequest(ctx, project, req)
	if err != nil {
		return preview, err
	}
	tx, err := s.Store.DB.Begin(ctx)
	if err != nil {
		return preview, err
	}
	defer tx.Rollback(ctx)
	var syncRunID string
	if err := tx.QueryRow(ctx, `INSERT INTO sync_runs(project_id,direction,status,actor_type,actor_name,transport) VALUES($1,'DB_TO_REPO','running',$2,$3,$4) RETURNING id`, project.ID, req.ActorType, req.ActorName, req.Transport).Scan(&syncRunID); err != nil {
		return preview, err
	}
	for i, item := range preview.Items {
		action := resolveSyncAction(item, req)
		preview.Items[i].Action = action
		if action == "noop" || action == "ignore_file" || item.Status == "synced" {
			if action == "ignore_file" {
				preview.Items[i].Status = "ignored"
				_, _ = tx.Exec(ctx, `INSERT INTO sync_run_items(sync_run_id,target_path,action,status,before_checksum,after_checksum,message) VALUES($1,$2,$3,$4,$5,$6,$7)`, syncRunID, item.TargetPath, action, "ignored", item.RepoChecksum, item.RepoChecksum, "Ignored by sync action.")
			}
			continue
		}
		if action == "import_repo_as_draft" {
			imported, err := s.importRepoFileAsDraft(ctx, tx, project, item.TargetPath, i)
			if err != nil {
				preview.Items[i].Status = "failed"
				preview.Items[i].Message = err.Error()
			} else {
				preview.Items[i] = mergeSyncItem(preview.Items[i], imported)
			}
			continue
		}
		full, err := s.Guard.SafeTarget(project.RepoPath, item.TargetPath)
		if err != nil {
			preview.Items[i].Status = "conflicted"
			continue
		}
		if action == "backup_then_overwrite" {
			backup, err := s.backupFile(project, item.TargetPath)
			if err != nil {
				preview.Items[i].Status = "failed"
				preview.Items[i].Message = err.Error()
				continue
			}
			preview.Items[i].BackupPath = backup
		}
		if err := os.WriteFile(full, []byte(item.RenderedPreview), 0644); err != nil {
			preview.Items[i].Status = "failed"
			preview.Items[i].Message = err.Error()
			continue
		}
		after, _ := FileChecksum(full)
		preview.Items[i].RepoChecksum = after
		preview.Items[i].Status = "synced"
		_, _ = tx.Exec(ctx, `INSERT INTO sync_run_items(sync_run_id,target_path,action,status,before_checksum,after_checksum,message) VALUES($1,$2,$3,$4,$5,$6,$7)`, syncRunID, item.TargetPath, action, preview.Items[i].Status, item.RepoChecksum, after, item.Message)
	}
	if err := s.writeProjectFiles(ctx, tx, project, preview.Items); err != nil {
		return preview, err
	}
	if _, err := tx.Exec(ctx, `UPDATE sync_runs SET status='completed', summary=$1, finished_at=now() WHERE id=$2`, fmt.Sprintf("Synced %d items", len(preview.Items)), syncRunID); err != nil {
		return preview, err
	}
	if err := tx.Commit(ctx); err != nil {
		return preview, err
	}
	return preview, nil
}

func (s SyncService) PreviewRepoToDB(project domain.Project, req SyncRequest) (SyncResponse, error) {
	files, err := scanManagedRepoFiles(project.RepoPath)
	if err != nil {
		return SyncResponse{}, err
	}
	items := make([]SyncItem, 0, len(files))
	for _, target := range files {
		full, err := s.Guard.SafeTargetForRead(project.RepoPath, target)
		if err != nil {
			items = append(items, SyncItem{TargetPath: target, Action: "ignore", Status: "conflicted", Message: err.Error()})
			continue
		}
		data, err := os.ReadFile(full)
		if err != nil {
			items = append(items, SyncItem{TargetPath: target, Action: "ignore", Status: "conflicted", Message: err.Error()})
			continue
		}
		typ, slug := classifyManagedFile(target)
		if typ == "" {
			items = append(items, SyncItem{TargetPath: target, Action: "ignore", Status: "ignored", RepoChecksum: SHA256Bytes(data)})
			continue
		}
		if header := parseAgentOpsHeader(string(data)); header != nil {
			if header["asset_type"] != "" {
				typ = header["asset_type"]
			}
			if header["asset_slug"] != "" {
				slug = header["asset_slug"]
			}
		}
		if len(req.TargetPaths) > 0 && !slices.Contains(req.TargetPaths, target) {
			continue
		}
		items = append(items, SyncItem{
			TargetPath: target, Action: "import_draft", Status: "draft_import_available",
			RepoChecksum: SHA256Bytes(data), AssetType: typ, AssetSlug: slug,
			RenderedPreview: stripAgentOpsHeader(string(data)),
			Message:         "Repo content will be imported as a draft asset version and will not overwrite a published DB version.",
			AllowedActions:  []string{"import_draft", "ignore_file"},
		})
	}
	return SyncResponse{Mode: "REPO_TO_DB", Items: items}, nil
}

func (s SyncService) ApplyRepoToDB(ctx context.Context, project domain.Project) (SyncResponse, error) {
	return s.ApplyRepoToDBWithRequest(ctx, project, SyncRequest{Mode: "REPO_TO_DB"})
}

func (s SyncService) ApplyRepoToDBWithRequest(ctx context.Context, project domain.Project, req SyncRequest) (SyncResponse, error) {
	req = normalizeActor(req, "operator", "http_api")
	preview, err := s.PreviewRepoToDB(project, req)
	if err != nil {
		return preview, err
	}
	tx, err := s.Store.DB.Begin(ctx)
	if err != nil {
		return preview, err
	}
	defer tx.Rollback(ctx)
	var syncRunID string
	if err := tx.QueryRow(ctx, `INSERT INTO sync_runs(project_id,direction,status,actor_type,actor_name,transport) VALUES($1,'REPO_TO_DB','running',$2,$3,$4) RETURNING id`, project.ID, req.ActorType, req.ActorName, req.Transport).Scan(&syncRunID); err != nil {
		return preview, err
	}
	for idx, item := range preview.Items {
		action := resolveSyncAction(item, req)
		if action == "ignore_file" {
			preview.Items[idx].Status = "ignored"
			_, _ = tx.Exec(ctx, `INSERT INTO sync_run_items(sync_run_id,target_path,action,status,before_checksum,after_checksum,message) VALUES($1,$2,$3,$4,$5,$6,$7)`, syncRunID, item.TargetPath, "ignore_file", "ignored", item.RepoChecksum, item.RepoChecksum, "Ignored by sync action.")
			continue
		}
		if item.Status != "draft_import_available" {
			continue
		}
		imported, err := s.importRepoFileAsDraft(ctx, tx, project, item.TargetPath, idx)
		if err != nil {
			preview.Items[idx].Status = "failed"
			preview.Items[idx].Message = err.Error()
			_, _ = tx.Exec(ctx, `INSERT INTO sync_run_items(sync_run_id,target_path,action,status,before_checksum,after_checksum,message) VALUES($1,$2,$3,$4,$5,$6,$7)`, syncRunID, item.TargetPath, "import_draft", "failed", item.RepoChecksum, "", err.Error())
			continue
		}
		preview.Items[idx] = mergeSyncItem(preview.Items[idx], imported)
		_, _ = tx.Exec(ctx, `INSERT INTO sync_run_items(sync_run_id,target_path,action,status,before_checksum,after_checksum,message) VALUES($1,$2,$3,$4,$5,$6,$7)`, syncRunID, item.TargetPath, "import_draft", "imported_draft", item.RepoChecksum, imported.DBChecksum, preview.Items[idx].Message)
	}
	if _, err := tx.Exec(ctx, `UPDATE sync_runs SET status='completed', summary=$1, finished_at=now() WHERE id=$2`, fmt.Sprintf("Imported %d repo files as drafts", len(preview.Items)), syncRunID); err != nil {
		return preview, err
	}
	if err := tx.Commit(ctx); err != nil {
		return preview, err
	}
	return preview, nil
}

func (s SyncService) plan(ctx context.Context, project domain.Project, req SyncRequest) ([]SyncItem, error) {
	projectAssets, err := s.projectAssetVersions(ctx, project)
	if err != nil {
		return nil, err
	}
	var items []SyncItem
	for _, pa := range projectAssets {
		if len(req.AssetIDs) > 0 && !slices.Contains(req.AssetIDs, pa.Asset.ID) {
			continue
		}
		if len(req.AssetTypes) > 0 && !slices.Contains(req.AssetTypes, pa.Asset.Type) {
			continue
		}
		target := pa.TargetPath
		if target == "" {
			target = targetPath(pa.Asset.Type, pa.Asset.Slug)
		}
		if target == "" {
			continue
		}
		if len(req.TargetPaths) > 0 && !slices.Contains(req.TargetPaths, target) {
			continue
		}
		slug := pa.Asset.Slug
		if _, targetSlug := classifyManagedFile(target); targetSlug != "" {
			slug = targetSlug
		}
		rendered := renderManagedFileWithMetadata(pa.Asset, pa.Version, slug, pa.TemplateAssetID)
		items = append(items, SyncItem{TargetPath: target, DBChecksum: SHA256String(rendered), RenderedPreview: rendered, AssetType: pa.Asset.Type, AssetSlug: slug, AssetID: pa.Asset.ID, AssetVersionID: pa.Version.ID, TemplateAssetID: pa.TemplateAssetID, Version: pa.Version.Version, AllowedActions: []string{"create", "overwrite_repo", "backup_then_overwrite", "ignore_file"}})
	}
	return items, nil
}

func (s SyncService) writeProjectFiles(ctx context.Context, tx pgx.Tx, project domain.Project, items []SyncItem) error {
	projectManifest := s.ProjectManifest(project)
	lock := buildAssetsLock(project, items)
	for _, item := range items {
		if item.AssetID == "" || item.AssetVersionID == "" || item.DBChecksum == "" {
			continue
		}
		if err := upsertProjectAssetBindingTx(ctx, tx, project.ID, item.AssetID, item.AssetVersionID, item.TargetPath, item.DBChecksum); err != nil {
			return err
		}
	}
	files := map[string]string{".codex/project.json": projectManifest, ".codex/sync/lock.json": lock}
	for target, content := range files {
		full, err := s.Guard.SafeTarget(project.RepoPath, target)
		if err != nil {
			return err
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (s SyncService) ProjectManifest(project domain.Project) string {
	return buildProjectManifest(project, s.MCPPublicURL)
}

func normalizeActor(req SyncRequest, actorType, transport string) SyncRequest {
	if strings.TrimSpace(req.ActorType) == "" {
		req.ActorType = actorType
	}
	if strings.TrimSpace(req.Transport) == "" {
		req.Transport = transport
	}
	req.ActorType = strings.TrimSpace(req.ActorType)
	req.ActorName = strings.TrimSpace(req.ActorName)
	req.Transport = strings.TrimSpace(req.Transport)
	return req
}

type projectAssetVersion struct {
	Asset           domain.Asset
	Version         domain.AssetVersion
	TargetPath      string
	TemplateAssetID string
}

func (s SyncService) projectAssetVersions(ctx context.Context, project domain.Project) ([]projectAssetVersion, error) {
	rows, err := s.Store.DB.Query(ctx, `
		SELECT COALESCE(b.target_path, ''),
		       COALESCE(a.template_asset_id::text, ''),
		       a.id, a.workspace_id, a.type, a.slug, a.name, a.description, a.current_version_id, a.status, a.tags, a.created_at, a.updated_at,
		       v.id, v.asset_id, v.version, v.content, v.content_format, v.checksum, v.metadata, v.status, v.created_at, v.published_at
		FROM agentic_assets a
		JOIN agentic_asset_versions v ON v.id=a.current_version_id
		LEFT JOIN project_asset_bindings b ON b.project_id=$2 AND b.asset_id=a.id
		WHERE a.workspace_id=$1
		  AND a.project_id=$2
		  AND a.scope='project'
		  AND a.status='active'
		ORDER BY COALESCE(NULLIF(b.target_path, ''), a.type || '/' || a.slug)`, project.WorkspaceID, project.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []projectAssetVersion
	for rows.Next() {
		var item projectAssetVersion
		if err := rows.Scan(
			&item.TargetPath, &item.TemplateAssetID,
			&item.Asset.ID, &item.Asset.WorkspaceID, &item.Asset.Type, &item.Asset.Slug, &item.Asset.Name, &item.Asset.Description, &item.Asset.CurrentVersionID, &item.Asset.Status, &item.Asset.Tags, &item.Asset.CreatedAt, &item.Asset.UpdatedAt,
			&item.Version.ID, &item.Version.AssetID, &item.Version.Version, &item.Version.Content, &item.Version.ContentFormat, &item.Version.Checksum, &item.Version.Metadata, &item.Version.Status, &item.Version.CreatedAt, &item.Version.PublishedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func buildAssetsLock(project domain.Project, items []SyncItem) string {
	type lockedAssetJSON struct {
		AssetID         string `json:"asset_id"`
		AssetVersionID  string `json:"asset_version_id"`
		TemplateAssetID string `json:"template_asset_id,omitempty"`
		Type            string `json:"type"`
		Slug            string `json:"slug"`
		Version         string `json:"version"`
		TargetPath      string `json:"target_path"`
		Checksum        string `json:"checksum"`
		Generated       bool   `json:"generated"`
	}
	lock := struct {
		Version int `json:"version"`
		Project struct {
			ID   string `json:"id"`
			Slug string `json:"slug"`
		} `json:"project"`
		Assets []lockedAssetJSON `json:"assets"`
	}{Version: 1}
	lock.Project.ID = project.ID
	lock.Project.Slug = project.Slug
	for _, item := range items {
		if item.AssetID == "" || item.AssetVersionID == "" || item.DBChecksum == "" {
			continue
		}
		lock.Assets = append(lock.Assets, lockedAssetJSON{
			AssetID:         item.AssetID,
			AssetVersionID:  item.AssetVersionID,
			TemplateAssetID: item.TemplateAssetID,
			Type:            item.AssetType,
			Slug:            item.AssetSlug,
			Version:         item.Version,
			TargetPath:      item.TargetPath,
			Checksum:        item.DBChecksum,
			Generated:       true,
		})
	}
	data, _ := json.MarshalIndent(lock, "", "  ")
	return string(data) + "\n"
}

func buildProjectManifest(project domain.Project, mcpPublicURL string) string {
	if strings.TrimSpace(mcpPublicURL) == "" {
		mcpPublicURL = "/mcp"
	}
	manifest := map[string]any{
		"version": 1,
		"project": map[string]string{
			"id":           project.ID,
			"slug":         project.Slug,
			"workspace_id": project.WorkspaceID,
		},
		"repo": map[string]string{
			"name":           project.Name,
			"path_hint":      pathHint(project),
			"default_branch": project.DefaultBranch,
		},
		"sync": map[string]string{
			"source_of_truth": "database",
			"generated_at":    time.Now().UTC().Format(time.RFC3339),
			"generated_by":    "agentops-workspace",
			"filesystem":      "codex",
		},
		"mcp": map[string]string{
			"server_name": "agentops",
			"endpoint":    mcpPublicURL,
			"auth":        "bearer_token_env",
			"token_env":   "AGENTOPS_MCP_TOKEN",
		},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	return string(data) + "\n"
}

type lockedAsset struct {
	AssetID         string `json:"asset_id" yaml:"asset_id"`
	AssetVersionID  string `json:"asset_version_id" yaml:"asset_version_id"`
	TemplateAssetID string `json:"template_asset_id" yaml:"template_asset_id"`
	Type            string `json:"type" yaml:"type"`
	Slug            string `json:"slug" yaml:"slug"`
	TargetPath      string `json:"target_path" yaml:"target_path"`
	Checksum        string `json:"checksum" yaml:"checksum"`
}

func readLockedAsset(repoPath, target string) *lockedAsset {
	data, err := os.ReadFile(filepath.Join(repoPath, ".codex", "sync", "lock.json"))
	if err != nil {
		return nil
	}
	var lock struct {
		Assets []lockedAsset `json:"assets" yaml:"assets"`
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil
	}
	for _, asset := range lock.Assets {
		if asset.TargetPath == target {
			return &asset
		}
	}
	return nil
}

func templateAssetIDForImport(ctx context.Context, tx pgx.Tx, project domain.Project, target string, header map[string]string, locked *lockedAsset) string {
	if header != nil && validUUIDLike(header["template_asset_id"]) {
		return header["template_asset_id"]
	}
	if locked != nil {
		if validUUIDLike(locked.TemplateAssetID) {
			return locked.TemplateAssetID
		}
	}
	var templateAssetID string
	_ = tx.QueryRow(ctx, `
		SELECT COALESCE(a.template_asset_id::text, '')
		FROM project_asset_bindings b
		JOIN agentic_assets a ON a.id=b.asset_id
		WHERE b.project_id=$1 AND b.target_path=$2 AND a.scope='project'
		LIMIT 1`, project.ID, target).Scan(&templateAssetID)
	if templateAssetID != "" {
		return templateAssetID
	}
	candidates := []string{valueFromMap(header, "asset_id")}
	if locked != nil {
		candidates = append(candidates, locked.AssetID)
	}
	for _, candidate := range candidates {
		if !validUUIDLike(candidate) {
			continue
		}
		_ = tx.QueryRow(ctx, `
			SELECT CASE
				WHEN scope='template' THEN id::text
				WHEN scope='project' THEN COALESCE(template_asset_id::text, '')
				ELSE ''
			END
			FROM agentic_assets
			WHERE id=$1 AND workspace_id=$2`, candidate, project.WorkspaceID).Scan(&templateAssetID)
		if templateAssetID != "" {
			return templateAssetID
		}
	}
	return ""
}

func valueFromMap(values map[string]string, key string) string {
	if values == nil {
		return ""
	}
	return values[key]
}

func validUUIDLike(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return false
			}
		}
	}
	return true
}

func upsertProjectAssetDraftTx(ctx context.Context, tx pgx.Tx, project domain.Project, typ, slug, name, description, version, content, format, checksum, templateAssetID string) (domain.Asset, domain.AssetVersion, error) {
	var templateID any
	if templateAssetID != "" {
		templateID = templateAssetID
	}
	var a domain.Asset
	err := tx.QueryRow(ctx, `
		INSERT INTO agentic_assets(workspace_id,scope,project_id,template_asset_id,type,slug,name,description,tags)
		VALUES($1,'project',$2,$3,$4,$5,$6,$7,'[]'::jsonb)
		ON CONFLICT(project_id,type,slug) WHERE scope='project' DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, template_asset_id=COALESCE(agentic_assets.template_asset_id, EXCLUDED.template_asset_id), updated_at=now()
		RETURNING id, workspace_id, scope, project_id, template_asset_id, type, slug, name, description, current_version_id, status, tags, created_at, updated_at`,
		project.WorkspaceID, project.ID, templateID, typ, slug, name, description,
	).Scan(&a.ID, &a.WorkspaceID, &a.Scope, &a.ProjectID, &a.TemplateAssetID, &a.Type, &a.Slug, &a.Name, &a.Description, &a.CurrentVersionID, &a.Status, &a.Tags, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return domain.Asset{}, domain.AssetVersion{}, err
	}
	var v domain.AssetVersion
	err = tx.QueryRow(ctx, `
		INSERT INTO agentic_asset_versions(asset_id,version,content,content_format,checksum,status)
		VALUES($1,$2,$3,$4,$5,'draft')
		ON CONFLICT(asset_id,version) DO UPDATE SET content=EXCLUDED.content, content_format=EXCLUDED.content_format, checksum=EXCLUDED.checksum, status='draft'
		RETURNING id, asset_id, version, content, content_format, checksum, metadata, status, created_at, published_at`,
		a.ID, version, content, format, checksum,
	).Scan(&v.ID, &v.AssetID, &v.Version, &v.Content, &v.ContentFormat, &v.Checksum, &v.Metadata, &v.Status, &v.CreatedAt, &v.PublishedAt)
	if err != nil {
		return domain.Asset{}, domain.AssetVersion{}, err
	}
	return a, v, nil
}

func upsertProjectAssetBindingTx(ctx context.Context, tx pgx.Tx, projectID, assetID, assetVersionID, targetPath, checksum string) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO project_asset_bindings(project_id,asset_id,asset_version_id,target_path,sync_status,last_db_checksum,last_repo_checksum,last_synced_at)
		VALUES($1,$2,$3,$4,'synced',$5,$5,now())
		ON CONFLICT(project_id,target_path) DO UPDATE SET asset_id=EXCLUDED.asset_id, asset_version_id=EXCLUDED.asset_version_id, sync_status='synced', last_db_checksum=EXCLUDED.last_db_checksum, last_repo_checksum=EXCLUDED.last_repo_checksum, last_synced_at=now(), updated_at=now()
		`, projectID, assetID, assetVersionID, targetPath, checksum)
	return err
}

func renderManagedFile(asset domain.Asset, version domain.AssetVersion) string {
	return renderManagedFileWithMetadata(asset, version, asset.Slug, "")
}

func renderManagedFileWithMetadata(asset domain.Asset, version domain.AssetVersion, slug, templateAssetID string) string {
	if slug == "" {
		slug = asset.Slug
	}
	templateLine := ""
	if templateAssetID != "" {
		templateLine = fmt.Sprintf("  template_asset_id: %q\n", templateAssetID)
	}
	if version.ContentFormat == "markdown" || strings.HasSuffix(targetPath(asset.Type, asset.Slug), ".md") {
		return fmt.Sprintf("<!--\ncodex:\n  generated: true\n  asset_id: %q\n  asset_slug: %q\n  asset_type: %q\n  asset_version_id: %q\n%s  version: %q\n  checksum: %q\n  source_of_truth: database\n  do_not_edit: false\n-->\n\n%s", asset.ID, slug, asset.Type, version.ID, templateLine, version.Version, version.Checksum, version.Content)
	}
	commentTemplateLine := ""
	if templateAssetID != "" {
		commentTemplateLine = fmt.Sprintf("#   template_asset_id: %q\n", templateAssetID)
	}
	return fmt.Sprintf("# codex:\n#   generated: true\n#   asset_id: %q\n#   asset_slug: %q\n#   asset_type: %q\n#   asset_version_id: %q\n%s#   version: %q\n#   checksum: %q\n#   source_of_truth: database\n#   do_not_edit: false\n\n%s", asset.ID, slug, asset.Type, version.ID, commentTemplateLine, version.Version, version.Checksum, version.Content)
}

func targetPath(assetType, slug string) string {
	switch assetType {
	case "agents_md":
		return "AGENTS.md"
	case "readme":
		return "README.md"
	case "skill_doc":
		return filepath.ToSlash(filepath.Join(".codex/skills", slug, "SKILL.md"))
	case "subagent_doc":
		return filepath.ToSlash(filepath.Join(".codex/agents", slug+".toml"))
	case "policy_doc":
		return filepath.ToSlash(filepath.Join(".codex/policies", slug+".md"))
	case "workflow_doc":
		return filepath.ToSlash(filepath.Join(".codex/workflows", slug+".workflow.yaml"))
	case "prompt_template":
		return filepath.ToSlash(filepath.Join(".codex/prompts", slug+".md"))
	case "checklist":
		return filepath.ToSlash(filepath.Join(".codex/checklists", slug+".md"))
	case "run_report_contract":
		return ".codex/reports/run-report-contract.md"
	case "context_doc":
		return filepath.ToSlash(filepath.Join(".codex/assets/context", slug+".md"))
	default:
		return ""
	}
}

func TargetPathForAsset(assetType, slug string) string {
	return targetPath(assetType, slug)
}

func RenderManagedFile(asset domain.Asset, version domain.AssetVersion) string {
	return renderManagedFile(asset, version)
}

func pathHint(project domain.Project) string {
	if project.HostPathHint != nil && *project.HostPathHint != "" {
		return *project.HostPathHint
	}
	return project.RepoPath
}

func plannedTarget(items []SyncItem, target string) bool {
	return slices.ContainsFunc(items, func(item SyncItem) bool { return item.TargetPath == target })
}

func scanManagedRepoFiles(repoPath string) ([]string, error) {
	var out []string
	for _, rootTarget := range []string{"AGENTS.md", "README.md", ".codex/project.json", ".codex/sync/lock.json"} {
		if _, err := os.Stat(filepath.Join(repoPath, filepath.FromSlash(rootTarget))); err == nil {
			out = append(out, rootTarget)
		}
	}
	for _, base := range []string{".codex"} {
		root := filepath.Join(repoPath, base)
		if _, err := os.Stat(root); err != nil {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(repoPath, path)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if d.IsDir() {
				if shouldIgnoreDirPath(rel) || scanDepth(rel) > maxScanDepth {
					return filepath.SkipDir
				}
				return nil
			}
			if shouldIgnoreFile(rel) {
				return nil
			}
			if typ, _ := classifyManagedFile(rel); typ != "" {
				out = append(out, rel)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	err := filepath.WalkDir(repoPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(repoPath, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			return nil
		}
		if d.IsDir() {
			if shouldIgnoreDirPath(rel) || scanDepth(rel) > maxScanDepth {
				return filepath.SkipDir
			}
			if rel == ".codex" {
				return filepath.SkipDir
			}
			return nil
		}
		if shouldIgnoreFile(rel) {
			return nil
		}
		if typ, _ := classifyManagedFile(rel); typ != "" {
			out = append(out, rel)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(out)
	out = slices.Compact(out)
	return out, nil
}

func classifyManagedFile(target string) (string, string) {
	target = filepath.ToSlash(target)
	switch {
	case target == "AGENTS.md":
		return "agents_md", "repo-agents"
	case target == "README.md":
		return "readme", "repo-readme"
	case strings.HasPrefix(target, ".codex/agents/") && strings.HasSuffix(target, ".toml"):
		return "subagent_doc", trimExt(filepath.Base(target))
	case strings.HasPrefix(target, ".codex/skills/") && strings.HasSuffix(target, "/SKILL.md"):
		return "skill_doc", safeAssetSlug(strings.TrimSuffix(strings.TrimPrefix(target, ".codex/skills/"), "/SKILL.md"))
	case strings.HasPrefix(target, ".codex/skills/") && strings.HasSuffix(target, ".md"):
		return "skill_doc", safeAssetSlug(strings.TrimSuffix(strings.TrimPrefix(target, ".codex/skills/"), ".md"))
	case strings.HasPrefix(target, ".codex/policies/") && strings.HasSuffix(target, ".md"):
		return "policy_doc", trimExt(filepath.Base(target))
	case strings.HasPrefix(target, ".codex/workflows/") && (strings.HasSuffix(target, ".md") || strings.HasSuffix(target, ".yaml") || strings.HasSuffix(target, ".yml")):
		return "workflow_doc", trimExt(filepath.Base(target))
	case strings.HasPrefix(target, ".codex/prompts/"):
		return "prompt_template", trimExt(filepath.Base(target))
	case strings.HasPrefix(target, ".codex/checklists/"):
		return "checklist", trimExt(filepath.Base(target))
	case target == ".codex/reports/run-report-contract.md":
		return "run_report_contract", "run-report-contract"
	case strings.HasPrefix(target, ".codex/") && strings.HasSuffix(target, ".md"):
		return "context_doc", safeAssetSlug(strings.TrimSuffix(strings.TrimPrefix(target, ".codex/"), ".md"))
	case isOpenAIConfigPath(target):
		return "context_doc", safeAssetSlug(trimExt(target))
	case strings.HasSuffix(target, ".md"):
		return "context_doc", safeAssetSlug(strings.TrimSuffix(target, ".md"))
	default:
		return "", ""
	}
}

func isOpenAIConfigPath(target string) bool {
	base := strings.ToLower(filepath.Base(filepath.ToSlash(target)))
	return base == "openai.yaml" || base == "openai.yml"
}

func stripAgentOpsHeader(content string) string {
	trimmed := strings.TrimSpace(content)
	if strings.HasPrefix(trimmed, "<!--") {
		if idx := strings.Index(trimmed, "-->"); idx >= 0 {
			return strings.TrimSpace(trimmed[idx+3:])
		}
	}
	if strings.HasPrefix(trimmed, "# agentops:") || strings.HasPrefix(trimmed, "# codex:") {
		lines := strings.Split(trimmed, "\n")
		i := 0
		for i < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[i]), "#") {
			i++
		}
		return strings.TrimSpace(strings.Join(lines[i:], "\n"))
	}
	return content
}

func parseAgentOpsHeader(content string) map[string]string {
	trimmed := strings.TrimSpace(content)
	var header string
	if strings.HasPrefix(trimmed, "<!--") {
		if idx := strings.Index(trimmed, "-->"); idx >= 0 {
			header = trimmed[:idx]
		}
	} else if strings.HasPrefix(trimmed, "# agentops:") || strings.HasPrefix(trimmed, "# codex:") {
		lines := strings.Split(trimmed, "\n")
		var headerLines []string
		for _, line := range lines {
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				break
			}
			headerLines = append(headerLines, strings.TrimPrefix(strings.TrimSpace(line), "#"))
		}
		header = strings.Join(headerLines, "\n")
	}
	if header == "" || (!strings.Contains(header, "agentops:") && !strings.Contains(header, "codex:")) {
		return nil
	}
	out := map[string]string{}
	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		if !strings.Contains(line, ":") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"`)
		if key != "" && value != "" {
			out[key] = value
		}
	}
	return out
}

func contentFormatForTarget(target string) string {
	switch {
	case strings.HasSuffix(target, ".yaml"), strings.HasSuffix(target, ".yml"):
		return "yaml"
	case strings.HasSuffix(target, ".json"):
		return "json"
	case strings.HasSuffix(target, ".md"):
		return "markdown"
	default:
		return "text"
	}
}

func trimExt(name string) string {
	return strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".yaml"), ".yml"), ".toml"), ".md")
}

func safeAssetSlug(value string) string {
	value = strings.ToLower(filepath.ToSlash(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "context"
	}
	return slug
}

func titleCase(value string) string {
	words := strings.Fields(value)
	for i, word := range words {
		if word == "" {
			continue
		}
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func MarshalPreview(items []SyncItem) json.RawMessage {
	b, _ := json.Marshal(items)
	return b
}

func (s SyncService) importRepoFileAsDraft(ctx context.Context, tx pgx.Tx, project domain.Project, target string, idx int) (SyncItem, error) {
	full, err := s.Guard.SafeTargetForRead(project.RepoPath, target)
	if err != nil {
		return SyncItem{}, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return SyncItem{}, err
	}
	typ, slug := classifyManagedFile(target)
	header := parseAgentOpsHeader(string(data))
	if header != nil {
		if header["asset_type"] != "" {
			typ = header["asset_type"]
		}
		if header["asset_slug"] != "" {
			slug = header["asset_slug"]
		}
	}
	if typ == "" || slug == "" {
		return SyncItem{}, fmt.Errorf("cannot classify managed file %s", target)
	}
	content := stripAgentOpsHeader(string(data))
	version := "repo-draft-" + time.Now().UTC().Format("20060102T150405") + fmt.Sprintf("-%02d", idx+1)
	locked := readLockedAsset(project.RepoPath, target)
	templateAssetID := templateAssetIDForImport(ctx, tx, project, target, header, locked)
	a, v, err := upsertProjectAssetDraftTx(ctx, tx, project, typ, slug, titleCase(strings.ReplaceAll(slug, "-", " ")), "Imported from repository as a project-scoped draft. Published database versions are preserved.", version, content, contentFormatForTarget(target), SHA256String(content), templateAssetID)
	if err != nil {
		return SyncItem{}, err
	}
	if err := upsertProjectAssetBindingTx(ctx, tx, project.ID, a.ID, v.ID, target, SHA256String(content)); err != nil {
		return SyncItem{}, err
	}
	return SyncItem{
		TargetPath: target, Action: "import_draft", Status: "imported_draft",
		DBChecksum: v.Checksum, RepoChecksum: SHA256Bytes(data), AssetType: a.Type, AssetSlug: a.Slug,
		AssetID: a.ID, AssetVersionID: v.ID, TemplateAssetID: templateAssetID,
		Version: v.Version, Message: "Imported project draft asset version " + v.Version + " for " + a.Type + "/" + a.Slug + ".",
	}, nil
}

func mergeSyncItem(base, update SyncItem) SyncItem {
	if update.Action != "" {
		base.Action = update.Action
	}
	if update.Status != "" {
		base.Status = update.Status
	}
	if update.DBChecksum != "" {
		base.DBChecksum = update.DBChecksum
	}
	if update.RepoChecksum != "" {
		base.RepoChecksum = update.RepoChecksum
	}
	if update.Message != "" {
		base.Message = update.Message
	}
	if update.AssetType != "" {
		base.AssetType = update.AssetType
	}
	if update.AssetSlug != "" {
		base.AssetSlug = update.AssetSlug
	}
	if update.AssetID != "" {
		base.AssetID = update.AssetID
	}
	if update.AssetVersionID != "" {
		base.AssetVersionID = update.AssetVersionID
	}
	if update.TemplateAssetID != "" {
		base.TemplateAssetID = update.TemplateAssetID
	}
	if update.Version != "" {
		base.Version = update.Version
	}
	return base
}

func resolveSyncAction(item SyncItem, req SyncRequest) string {
	for _, action := range req.Actions {
		if action.TargetPath == item.TargetPath && action.Action != "" {
			return action.Action
		}
	}
	if req.ConflictAction != "" && item.Status == "drifted" {
		return req.ConflictAction
	}
	if item.Action != "" {
		return item.Action
	}
	return "noop"
}

func (s SyncService) backupFile(project domain.Project, target string) (string, error) {
	source := filepath.Join(project.RepoPath, filepath.FromSlash(target))
	data, err := os.ReadFile(source)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	backupTarget := filepath.ToSlash(filepath.Join(".codex", "sync", "backups", time.Now().UTC().Format("20060102T150405"), target))
	dest, err := s.Guard.SafeTarget(project.RepoPath, backupTarget)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(dest, data, 0644); err != nil {
		return "", err
	}
	return backupTarget, nil
}

func DiffSummary(before, after string) string {
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")
	maxLen := len(beforeLines)
	if len(afterLines) > maxLen {
		maxLen = len(afterLines)
	}
	changed, added, removed := 0, 0, 0
	for i := 0; i < maxLen; i++ {
		switch {
		case i >= len(beforeLines):
			added++
		case i >= len(afterLines):
			removed++
		case beforeLines[i] != afterLines[i]:
			changed++
		}
	}
	return fmt.Sprintf("%d changed, %d added, %d removed lines", changed, added, removed)
}

func DiffPreview(before, after string, limit int) []string {
	if limit <= 0 {
		limit = 10
	}
	beforeLines := strings.Split(before, "\n")
	afterLines := strings.Split(after, "\n")
	maxLen := len(beforeLines)
	if len(afterLines) > maxLen {
		maxLen = len(afterLines)
	}
	var out []string
	for i := 0; i < maxLen && len(out) < limit; i++ {
		switch {
		case i >= len(beforeLines):
			out = append(out, fmt.Sprintf("+%d %s", i+1, afterLines[i]))
		case i >= len(afterLines):
			out = append(out, fmt.Sprintf("-%d %s", i+1, beforeLines[i]))
		case beforeLines[i] != afterLines[i]:
			out = append(out, fmt.Sprintf("-%d %s", i+1, beforeLines[i]))
			if len(out) < limit {
				out = append(out, fmt.Sprintf("+%d %s", i+1, afterLines[i]))
			}
		}
	}
	return out
}
