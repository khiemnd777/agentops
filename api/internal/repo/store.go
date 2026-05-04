package repo

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"agentops-workspace/api/internal/domain"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	DB *pgxpool.Pool
}

func New(db *pgxpool.Pool) Store {
	return Store{DB: db}
}

func (s Store) EnsureDefaultWorkspace(ctx context.Context) (domain.Workspace, error) {
	var ws domain.Workspace
	err := s.DB.QueryRow(ctx, `
		INSERT INTO workspaces(name, slug) VALUES('Default Workspace', 'default')
		ON CONFLICT(slug) DO UPDATE SET updated_at=now()
		RETURNING id, name, slug, status, created_at, updated_at`).Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt)
	return ws, err
}

func (s Store) DefaultWorkspace(ctx context.Context) (domain.Workspace, error) {
	var ws domain.Workspace
	err := s.DB.QueryRow(ctx, `SELECT id, name, slug, status, created_at, updated_at FROM workspaces WHERE slug='default'`).Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.Status, &ws.CreatedAt, &ws.UpdatedAt)
	return ws, err
}

func (s Store) Dashboard(ctx context.Context, workspaceID string) (map[string]any, error) {
	out := map[string]any{}
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM projects WHERE workspace_id=$1 AND status='active'`, workspaceID).Scan(ptrMap(out, "project_count"))
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM project_asset_bindings WHERE sync_status IN ('drifted','missing','outdated')`).Scan(ptrMap(out, "drifted_bindings"))
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM task_run_reviews WHERE review_status IN ('warning','failed','needs_manual_review')`).Scan(ptrMap(out, "open_reviews"))
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM agentic_assets WHERE workspace_id=$1 AND scope='template' AND status='active'`, workspaceID).Scan(ptrMap(out, "asset_count"))
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM task_runs tr JOIN projects p ON p.id=tr.project_id WHERE p.workspace_id=$1 AND p.status <> 'deleted'`, workspaceID).Scan(ptrMap(out, "task_run_count"))
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM task_run_imports i JOIN projects p ON p.id=i.project_id WHERE p.workspace_id=$1 AND p.status <> 'deleted' AND i.import_status NOT IN ('imported','duplicate')`, workspaceID).Scan(ptrMap(out, "import_error_count"))
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM task_run_reviews r JOIN task_runs tr ON tr.id=r.task_run_id JOIN projects p ON p.id=tr.project_id WHERE p.workspace_id=$1 AND p.status <> 'deleted' AND r.review_status='pass'`, workspaceID).Scan(ptrMap(out, "passed_reviews"))
	_ = s.DB.QueryRow(ctx, `SELECT count(*) FROM task_run_reviews r JOIN task_runs tr ON tr.id=r.task_run_id JOIN projects p ON p.id=tr.project_id WHERE p.workspace_id=$1 AND p.status <> 'deleted'`, workspaceID).Scan(ptrMap(out, "total_reviews"))
	out["recent_reviews"] = []map[string]any{}
	if rows, err := s.DB.Query(ctx, `SELECT tr.id,tr.external_run_id,tr.title,tr.review_status,p.name,tr.imported_at FROM task_runs tr JOIN projects p ON p.id=tr.project_id WHERE p.workspace_id=$1 AND p.status <> 'deleted' ORDER BY tr.imported_at DESC NULLS LAST, tr.created_at DESC LIMIT 5`, workspaceID); err == nil {
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var id, externalID, title, reviewStatus, projectName string
			var importedAt *time.Time
			if err := rows.Scan(&id, &externalID, &title, &reviewStatus, &projectName, &importedAt); err == nil {
				items = append(items, map[string]any{"id": id, "external_run_id": externalID, "title": title, "review_status": reviewStatus, "project_name": projectName, "imported_at": importedAt})
			}
		}
		out["recent_reviews"] = items
	}
	out["recent_import_errors"] = []map[string]any{}
	if rows, err := s.DB.Query(ctx, `SELECT i.run_id,i.import_status,i.error_message,i.created_at,p.name FROM task_run_imports i JOIN projects p ON p.id=i.project_id WHERE p.workspace_id=$1 AND p.status <> 'deleted' AND i.import_status NOT IN ('imported','duplicate') ORDER BY i.created_at DESC LIMIT 5`, workspaceID); err == nil {
		defer rows.Close()
		items := []map[string]any{}
		for rows.Next() {
			var runID, status, projectName string
			var errorMessage *string
			var createdAt time.Time
			if err := rows.Scan(&runID, &status, &errorMessage, &createdAt, &projectName); err == nil {
				items = append(items, map[string]any{"run_id": runID, "import_status": status, "error_message": errorMessage, "created_at": createdAt, "project_name": projectName})
			}
		}
		out["recent_import_errors"] = items
	}
	return out, nil
}

func ptrMap(m map[string]any, key string) *int {
	var v int
	m[key] = &v
	return &v
}

const assetSelectColumns = `id, workspace_id, scope, project_id, template_asset_id, type, slug, name, description, current_version_id, status, tags, created_at, updated_at`
const assetSelectColumnsA = `a.id, a.workspace_id, a.scope, a.project_id, a.template_asset_id, a.type, a.slug, a.name, a.description, a.current_version_id, a.status, a.tags, a.created_at, a.updated_at`

type scanner interface {
	Scan(dest ...any) error
}

func scanAsset(row scanner, a *domain.Asset) error {
	return row.Scan(&a.ID, &a.WorkspaceID, &a.Scope, &a.ProjectID, &a.TemplateAssetID, &a.Type, &a.Slug, &a.Name, &a.Description, &a.CurrentVersionID, &a.Status, &a.Tags, &a.CreatedAt, &a.UpdatedAt)
}

func scanAssetVersion(row scanner, v *domain.AssetVersion) error {
	return row.Scan(&v.ID, &v.AssetID, &v.Version, &v.Content, &v.ContentFormat, &v.Checksum, &v.Metadata, &v.Status, &v.CreatedAt, &v.PublishedAt)
}

func normalizedAssetFilter(filters []domain.AssetListFilter) domain.AssetListFilter {
	filter := domain.AssetListFilter{Scope: "template"}
	if len(filters) > 0 {
		filter = filters[0]
	}
	if filter.Scope == "" {
		if filter.ProjectID != "" {
			filter.Scope = "project"
		} else {
			filter.Scope = "template"
		}
	}
	return filter
}

func (s Store) ListProjects(ctx context.Context, workspaceID string) ([]domain.Project, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, workspace_id, name, slug, repo_path, host_path_hint, default_branch, description, status, last_auto_import_at, created_at, updated_at FROM projects WHERE workspace_id=$1 AND status <> 'deleted' ORDER BY created_at DESC`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Project, 0)
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.RepoPath, &p.HostPathHint, &p.DefaultBranch, &p.Description, &p.Status, &p.LastAutoImportAt, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s Store) DeleteProject(ctx context.Context, workspaceID, id string) (bool, error) {
	tag, err := s.DB.Exec(ctx, `
		UPDATE projects
		SET status='deleted',
			slug=slug || '-deleted-' || left(id::text, 8),
			updated_at=now()
		WHERE workspace_id=$1 AND id=$2 AND status <> 'deleted'`,
		workspaceID, id,
	)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

func (s Store) CreateProject(ctx context.Context, p domain.Project) (domain.Project, error) {
	if p.DefaultBranch == "" {
		p.DefaultBranch = "main"
	}
	err := s.DB.QueryRow(ctx, `
		INSERT INTO projects(workspace_id, name, slug, repo_path, host_path_hint, default_branch, description)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		RETURNING id, workspace_id, name, slug, repo_path, host_path_hint, default_branch, description, status, last_auto_import_at, created_at, updated_at`,
		p.WorkspaceID, p.Name, p.Slug, p.RepoPath, p.HostPathHint, p.DefaultBranch, p.Description,
	).Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.RepoPath, &p.HostPathHint, &p.DefaultBranch, &p.Description, &p.Status, &p.LastAutoImportAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s Store) GetProject(ctx context.Context, id string) (domain.Project, error) {
	var p domain.Project
	err := s.DB.QueryRow(ctx, `SELECT id, workspace_id, name, slug, repo_path, host_path_hint, default_branch, description, status, last_auto_import_at, created_at, updated_at FROM projects WHERE id=$1`, id).
		Scan(&p.ID, &p.WorkspaceID, &p.Name, &p.Slug, &p.RepoPath, &p.HostPathHint, &p.DefaultBranch, &p.Description, &p.Status, &p.LastAutoImportAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (s Store) ListAssets(ctx context.Context, workspaceID, query, assetType string, filters ...domain.AssetListFilter) ([]domain.Asset, error) {
	filter := normalizedAssetFilter(filters)
	sql := `SELECT DISTINCT ` + assetSelectColumnsA + `
		FROM agentic_assets a
		LEFT JOIN agentic_asset_versions v ON v.asset_id=a.id
		WHERE a.workspace_id=$1 AND ($2='' OR a.type=$2)
		AND ($3='' OR a.name ILIKE '%'||$3||'%' OR a.slug ILIKE '%'||$3||'%' OR a.type ILIKE '%'||$3||'%' OR a.tags::text ILIKE '%'||$3||'%' OR v.content ILIKE '%'||$3||'%')
		AND (
			($4='template' AND a.scope='template')
			OR ($4='project' AND a.scope='project' AND ($5='' OR a.project_id=NULLIF($5,'')::uuid))
			OR ($4='all' AND ($5='' OR a.scope='template' OR a.project_id=NULLIF($5,'')::uuid))
		)
		ORDER BY a.scope, a.type, a.slug`
	rows, err := s.DB.Query(ctx, sql, workspaceID, assetType, query, filter.Scope, filter.ProjectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Asset
	for rows.Next() {
		var a domain.Asset
		if err := scanAsset(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s Store) UpsertAssetWithVersion(ctx context.Context, workspaceID, typ, slug, name, description, version, content, format, status, checksum string, tags []string) (domain.Asset, domain.AssetVersion, error) {
	tagsJSON, _ := json.Marshal(tags)
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.Asset{}, domain.AssetVersion{}, err
	}
	defer tx.Rollback(ctx)
	var a domain.Asset
	if err := tx.QueryRow(ctx, `
		INSERT INTO agentic_assets(workspace_id,scope,type,slug,name,description,tags)
		VALUES($1,'template',$2,$3,$4,$5,$6)
		ON CONFLICT(workspace_id,type,slug) WHERE scope='template' DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, tags=EXCLUDED.tags, updated_at=now()
		RETURNING `+assetSelectColumns,
		workspaceID, typ, slug, name, description, tagsJSON).Scan(&a.ID, &a.WorkspaceID, &a.Scope, &a.ProjectID, &a.TemplateAssetID, &a.Type, &a.Slug, &a.Name, &a.Description, &a.CurrentVersionID, &a.Status, &a.Tags, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return domain.Asset{}, domain.AssetVersion{}, err
	}
	publishedAt := any(nil)
	if status == "published" {
		publishedAt = time.Now()
	}
	var v domain.AssetVersion
	if err := tx.QueryRow(ctx, `
		INSERT INTO agentic_asset_versions(asset_id,version,content,content_format,checksum,status,published_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT(asset_id,version) DO UPDATE SET content=EXCLUDED.content, content_format=EXCLUDED.content_format, checksum=EXCLUDED.checksum, status=EXCLUDED.status, published_at=COALESCE(agentic_asset_versions.published_at, EXCLUDED.published_at)
		RETURNING id, asset_id, version, content, content_format, checksum, metadata, status, created_at, published_at`,
		a.ID, version, content, format, checksum, status, publishedAt).Scan(&v.ID, &v.AssetID, &v.Version, &v.Content, &v.ContentFormat, &v.Checksum, &v.Metadata, &v.Status, &v.CreatedAt, &v.PublishedAt); err != nil {
		return domain.Asset{}, domain.AssetVersion{}, err
	}
	if status == "published" {
		if _, err := tx.Exec(ctx, `UPDATE agentic_assets SET current_version_id=$1, updated_at=now() WHERE id=$2`, v.ID, a.ID); err != nil {
			return domain.Asset{}, domain.AssetVersion{}, err
		}
		a.CurrentVersionID = &v.ID
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.Asset{}, domain.AssetVersion{}, err
	}
	return a, v, nil
}

func (s Store) GetAsset(ctx context.Context, id string) (domain.Asset, error) {
	var a domain.Asset
	err := scanAsset(s.DB.QueryRow(ctx, `SELECT `+assetSelectColumns+` FROM agentic_assets WHERE id=$1`, id), &a)
	return a, err
}

func (s Store) ListAssetVersions(ctx context.Context, assetID string) ([]domain.AssetVersion, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, asset_id, version, content, content_format, checksum, metadata, status, created_at, published_at FROM agentic_asset_versions WHERE asset_id=$1 ORDER BY created_at DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AssetVersion
	for rows.Next() {
		var v domain.AssetVersion
		if err := rows.Scan(&v.ID, &v.AssetID, &v.Version, &v.Content, &v.ContentFormat, &v.Checksum, &v.Metadata, &v.Status, &v.CreatedAt, &v.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s Store) PublishAssetVersion(ctx context.Context, assetID, versionID string) error {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `UPDATE agentic_asset_versions SET status='published', published_at=COALESCE(published_at, now()) WHERE id=$1 AND asset_id=$2`, versionID, assetID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE agentic_assets SET current_version_id=$1, updated_at=now() WHERE id=$2`, versionID, assetID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s Store) CurrentAssetVersions(ctx context.Context, workspaceID string) ([]domain.AssetWithVersion, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT `+assetSelectColumnsA+`,
		       v.id, v.asset_id, v.version, v.content, v.content_format, v.checksum, v.metadata, v.status, v.created_at, v.published_at
		FROM agentic_assets a JOIN agentic_asset_versions v ON v.id=a.current_version_id
		WHERE a.workspace_id=$1 AND a.scope='template' AND a.status='active'
		ORDER BY a.type, a.slug`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AssetWithVersion
	for rows.Next() {
		var item domain.AssetWithVersion
		if err := rows.Scan(&item.Asset.ID, &item.Asset.WorkspaceID, &item.Asset.Scope, &item.Asset.ProjectID, &item.Asset.TemplateAssetID, &item.Asset.Type, &item.Asset.Slug, &item.Asset.Name, &item.Asset.Description, &item.Asset.CurrentVersionID, &item.Asset.Status, &item.Asset.Tags, &item.Asset.CreatedAt, &item.Asset.UpdatedAt, &item.Version.ID, &item.Version.AssetID, &item.Version.Version, &item.Version.Content, &item.Version.ContentFormat, &item.Version.Checksum, &item.Version.Metadata, &item.Version.Status, &item.Version.CreatedAt, &item.Version.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Store) ListProjectAssetBindings(ctx context.Context, projectID, query, assetType string) ([]domain.ProjectAsset, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT b.id, b.project_id, b.asset_id, b.asset_version_id, b.target_path, b.sync_policy, b.sync_status, COALESCE(b.last_db_checksum,''), COALESCE(b.last_repo_checksum,''), b.last_synced_at,
		       `+assetSelectColumnsA+`,
		       v.id, v.asset_id, v.version, v.content, v.content_format, v.checksum, v.metadata, v.status, v.created_at, v.published_at
		FROM project_asset_bindings b
		JOIN agentic_assets a ON a.id=b.asset_id
		JOIN agentic_asset_versions v ON v.id=b.asset_version_id
		WHERE b.project_id=$1
		AND ($2='' OR a.type=$2)
		AND ($3='' OR a.name ILIKE '%'||$3||'%' OR a.slug ILIKE '%'||$3||'%' OR a.type ILIKE '%'||$3||'%' OR b.target_path ILIKE '%'||$3||'%' OR v.content ILIKE '%'||$3||'%')
		ORDER BY b.target_path`, projectID, assetType, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ProjectAsset
	for rows.Next() {
		var item domain.ProjectAsset
		if err := rows.Scan(
			&item.Binding.ID, &item.Binding.ProjectID, &item.Binding.AssetID, &item.Binding.AssetVersionID, &item.Binding.TargetPath, &item.Binding.SyncPolicy, &item.Binding.SyncStatus, &item.Binding.LastDBChecksum, &item.Binding.LastRepoChecksum, &item.Binding.LastSyncedAt,
			&item.Asset.ID, &item.Asset.WorkspaceID, &item.Asset.Scope, &item.Asset.ProjectID, &item.Asset.TemplateAssetID, &item.Asset.Type, &item.Asset.Slug, &item.Asset.Name, &item.Asset.Description, &item.Asset.CurrentVersionID, &item.Asset.Status, &item.Asset.Tags, &item.Asset.CreatedAt, &item.Asset.UpdatedAt,
			&item.Version.ID, &item.Version.AssetID, &item.Version.Version, &item.Version.Content, &item.Version.ContentFormat, &item.Version.Checksum, &item.Version.Metadata, &item.Version.Status, &item.Version.CreatedAt, &item.Version.PublishedAt,
		); err != nil {
			return nil, err
		}
		item.ProjectID = item.Binding.ProjectID
		item.TargetPath = item.Binding.TargetPath
		if item.Asset.TemplateAssetID != nil {
			item.TemplateAssetID = *item.Asset.TemplateAssetID
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Store) ListProjectAssets(ctx context.Context, projectID string) ([]domain.Asset, error) {
	rows, err := s.DB.Query(ctx, `SELECT `+assetSelectColumnsA+` FROM agentic_assets a WHERE a.scope='project' AND a.project_id=$1 ORDER BY a.type, a.slug`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Asset
	for rows.Next() {
		var a domain.Asset
		if err := scanAsset(rows, &a); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s Store) ListProjectOwnedAssets(ctx context.Context, projectID string) ([]domain.Asset, error) {
	return s.ListProjectAssets(ctx, projectID)
}

func (s Store) CurrentProjectAssetVersions(ctx context.Context, projectID string) ([]domain.AssetWithVersion, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT `+assetSelectColumnsA+`,
		       v.id, v.asset_id, v.version, v.content, v.content_format, v.checksum, v.metadata, v.status, v.created_at, v.published_at
		FROM agentic_assets a JOIN agentic_asset_versions v ON v.id=a.current_version_id
		WHERE a.scope='project' AND a.project_id=$1 AND a.status='active'
		ORDER BY a.type, a.slug`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AssetWithVersion
	for rows.Next() {
		var item domain.AssetWithVersion
		if err := rows.Scan(&item.Asset.ID, &item.Asset.WorkspaceID, &item.Asset.Scope, &item.Asset.ProjectID, &item.Asset.TemplateAssetID, &item.Asset.Type, &item.Asset.Slug, &item.Asset.Name, &item.Asset.Description, &item.Asset.CurrentVersionID, &item.Asset.Status, &item.Asset.Tags, &item.Asset.CreatedAt, &item.Asset.UpdatedAt, &item.Version.ID, &item.Version.AssetID, &item.Version.Version, &item.Version.Content, &item.Version.ContentFormat, &item.Version.Checksum, &item.Version.Metadata, &item.Version.Status, &item.Version.CreatedAt, &item.Version.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Store) ListAssetPresets(ctx context.Context, workspaceID string) ([]domain.AssetPreset, error) {
	rows, err := s.DB.Query(ctx, `
		SELECT p.id, p.workspace_id, p.slug, p.name, p.description, p.status, p.current_version, p.tags, count(i.id), p.created_at, p.updated_at
		FROM asset_presets p
		LEFT JOIN asset_preset_items i ON i.preset_id=p.id
		WHERE p.workspace_id=$1 AND p.status <> 'archived'
		GROUP BY p.id
		ORDER BY p.name, p.slug`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AssetPreset
	for rows.Next() {
		var p domain.AssetPreset
		if err := rows.Scan(&p.ID, &p.WorkspaceID, &p.Slug, &p.Name, &p.Description, &p.Status, &p.CurrentVersion, &p.Tags, &p.ItemCount, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s Store) UpsertAssetPreset(ctx context.Context, workspaceID, presetID, slug, name, description, status string, currentVersion int, tags []string) (domain.AssetPresetDetail, error) {
	if status == "" {
		status = "active"
	}
	if currentVersion <= 0 {
		currentVersion = 1
	}
	tagsJSON, _ := json.Marshal(tags)
	var id string
	var err error
	if presetID == "" {
		err = s.DB.QueryRow(ctx, `
			INSERT INTO asset_presets(workspace_id,slug,name,description,status,current_version,tags)
			VALUES($1,$2,$3,$4,$5,$6,$7)
			RETURNING id`, workspaceID, slug, name, description, status, currentVersion, tagsJSON).Scan(&id)
	} else {
		err = s.DB.QueryRow(ctx, `
			UPDATE asset_presets
			SET slug=$3, name=$4, description=$5, status=$6, current_version=$7, tags=$8, updated_at=now()
			WHERE workspace_id=$1 AND (id::text=$2 OR slug=$2)
			RETURNING id`, workspaceID, presetID, slug, name, description, status, currentVersion, tagsJSON).Scan(&id)
	}
	if err != nil {
		return domain.AssetPresetDetail{}, err
	}
	return s.GetAssetPreset(ctx, workspaceID, id)
}

func (s Store) GetAssetPreset(ctx context.Context, workspaceID, presetID string) (domain.AssetPresetDetail, error) {
	var detail domain.AssetPresetDetail
	if err := s.DB.QueryRow(ctx, `
		SELECT p.id, p.workspace_id, p.slug, p.name, p.description, p.status, p.current_version, p.tags, count(i.id), p.created_at, p.updated_at
		FROM asset_presets p
		LEFT JOIN asset_preset_items i ON i.preset_id=p.id
		WHERE p.workspace_id=$1 AND (p.id::text=$2 OR p.slug=$2)
		GROUP BY p.id`, workspaceID, presetID).Scan(&detail.ID, &detail.WorkspaceID, &detail.Slug, &detail.Name, &detail.Description, &detail.Status, &detail.CurrentVersion, &detail.Tags, &detail.ItemCount, &detail.CreatedAt, &detail.UpdatedAt); err != nil {
		return detail, err
	}
	items, err := s.assetPresetItems(ctx, s.DB, detail.ID)
	if err != nil {
		return detail, err
	}
	detail.Items = items
	return detail, nil
}

func (s Store) AddAssetPresetItem(ctx context.Context, workspaceID, presetID, templateAssetID, templateAssetVersionID, targetPath string, required bool, sortOrder int) (domain.AssetPresetDetail, error) {
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return domain.AssetPresetDetail{}, err
	}
	defer tx.Rollback(ctx)
	var resolvedPresetID string
	if err := tx.QueryRow(ctx, `SELECT id::text FROM asset_presets WHERE workspace_id=$1 AND (id::text=$2 OR slug=$2)`, workspaceID, presetID).Scan(&resolvedPresetID); err != nil {
		return domain.AssetPresetDetail{}, err
	}
	var valid bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1
			FROM agentic_assets a
			JOIN agentic_asset_versions v ON v.asset_id=a.id
			WHERE a.workspace_id=$1 AND a.scope='template' AND a.id=$2 AND v.id=$3
		)`, workspaceID, templateAssetID, templateAssetVersionID).Scan(&valid); err != nil {
		return domain.AssetPresetDetail{}, err
	}
	if !valid {
		return domain.AssetPresetDetail{}, pgx.ErrNoRows
	}
	if _, err := tx.Exec(ctx, `DELETE FROM asset_preset_items WHERE preset_id=$1 AND asset_id=$2`, resolvedPresetID, templateAssetID); err != nil {
		return domain.AssetPresetDetail{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO asset_preset_items(preset_id,asset_id,asset_version_id,target_path,sort_order,required)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(preset_id,asset_version_id) DO UPDATE SET target_path=EXCLUDED.target_path, sort_order=EXCLUDED.sort_order, required=EXCLUDED.required`,
		resolvedPresetID, templateAssetID, templateAssetVersionID, targetPath, sortOrder, required); err != nil {
		return domain.AssetPresetDetail{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.AssetPresetDetail{}, err
	}
	return s.GetAssetPreset(ctx, workspaceID, resolvedPresetID)
}

func (s Store) RemoveAssetPresetItem(ctx context.Context, workspaceID, presetID, itemID string) (domain.AssetPresetDetail, error) {
	var resolvedPresetID string
	err := s.DB.QueryRow(ctx, `
		DELETE FROM asset_preset_items i
		USING asset_presets p
		WHERE p.id=i.preset_id AND p.workspace_id=$1 AND (p.id::text=$2 OR p.slug=$2) AND i.id=$3
		RETURNING p.id::text`, workspaceID, presetID, itemID).Scan(&resolvedPresetID)
	if err != nil {
		return domain.AssetPresetDetail{}, err
	}
	return s.GetAssetPreset(ctx, workspaceID, resolvedPresetID)
}

type queryer interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func (s Store) assetPresetItems(ctx context.Context, q queryer, presetID string) ([]domain.AssetPresetItem, error) {
	rows, err := q.Query(ctx, `
		SELECT i.id, i.preset_id, i.asset_id, i.asset_version_id, i.target_path, i.sort_order, i.required, i.created_at,
		       `+assetSelectColumnsA+`,
		       v.id, v.asset_id, v.version, v.content, v.content_format, v.checksum, v.metadata, v.status, v.created_at, v.published_at
		FROM asset_preset_items i
		JOIN agentic_assets a ON a.id=i.asset_id
		JOIN agentic_asset_versions v ON v.id=i.asset_version_id AND v.asset_id=a.id
		WHERE i.preset_id=$1 AND a.scope='template'
		ORDER BY i.sort_order, a.type, a.slug`, presetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AssetPresetItem
	for rows.Next() {
		var item domain.AssetPresetItem
		if err := rows.Scan(&item.ID, &item.PresetID, &item.AssetID, &item.AssetVersionID, &item.TargetPath, &item.SortOrder, &item.Required, &item.CreatedAt, &item.Asset.ID, &item.Asset.WorkspaceID, &item.Asset.Scope, &item.Asset.ProjectID, &item.Asset.TemplateAssetID, &item.Asset.Type, &item.Asset.Slug, &item.Asset.Name, &item.Asset.Description, &item.Asset.CurrentVersionID, &item.Asset.Status, &item.Asset.Tags, &item.Asset.CreatedAt, &item.Asset.UpdatedAt, &item.Version.ID, &item.Version.AssetID, &item.Version.Version, &item.Version.Content, &item.Version.ContentFormat, &item.Version.Checksum, &item.Version.Metadata, &item.Version.Status, &item.Version.CreatedAt, &item.Version.PublishedAt); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s Store) ApplyAssetPresetToProject(ctx context.Context, workspaceID, projectID, presetID string) (domain.ApplyAssetPresetResult, error) {
	result := domain.ApplyAssetPresetResult{PresetID: presetID, ProjectID: projectID}
	tx, err := s.DB.Begin(ctx)
	if err != nil {
		return result, err
	}
	defer tx.Rollback(ctx)

	var resolvedPresetID string
	if err := tx.QueryRow(ctx, `
		SELECT ap.id::text
		FROM projects pr
		JOIN asset_presets ap ON ap.workspace_id=pr.workspace_id
		WHERE pr.id=$1 AND (ap.id::text=$2 OR ap.slug=$2) AND pr.workspace_id=$3 AND pr.status='active' AND ap.status='active'`,
		projectID, presetID, workspaceID).Scan(&resolvedPresetID); err != nil {
		return result, err
	}
	result.PresetID = resolvedPresetID

	items, err := s.assetPresetItems(ctx, tx, resolvedPresetID)
	if err != nil {
		return result, err
	}
	for _, item := range items {
		applied, err := s.applyPresetItem(ctx, tx, workspaceID, projectID, item)
		if err != nil {
			return result, err
		}
		result.Items = append(result.Items, applied)
		switch applied.Status {
		case "created":
			result.Created++
		case "already_exists":
			result.Skipped++
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return result, err
	}
	return result, nil
}

func (s Store) applyPresetItem(ctx context.Context, tx pgx.Tx, workspaceID, projectID string, item domain.AssetPresetItem) (domain.ApplyAssetPresetItemResult, error) {
	applied := domain.ApplyAssetPresetItemResult{
		PresetItemID:           item.ID,
		TemplateAssetID:        item.Asset.ID,
		TemplateAssetVersionID: item.Version.ID,
		Type:                   item.Asset.Type,
		Slug:                   item.Asset.Slug,
		TargetPath:             item.TargetPath,
	}
	if item.TargetPath != "" {
		var existingAssetID string
		err := tx.QueryRow(ctx, `SELECT asset_id::text FROM project_asset_bindings WHERE project_id=$1 AND target_path=$2`, projectID, item.TargetPath).Scan(&existingAssetID)
		if err == nil {
			applied.Status = "already_exists"
			applied.ExistingAssetID = existingAssetID
			applied.Message = "project asset target path already exists"
			return applied, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return applied, err
		}
	}

	var projectAsset domain.Asset
	err := scanAsset(tx.QueryRow(ctx, `
		INSERT INTO agentic_assets(workspace_id,scope,project_id,template_asset_id,type,slug,name,description,status,tags)
		VALUES($1,'project',$2,$3,$4,$5,$6,$7,'active',$8)
		ON CONFLICT(project_id,type,slug) WHERE scope='project' DO NOTHING
		RETURNING `+assetSelectColumns,
		workspaceID, projectID, item.Asset.ID, item.Asset.Type, item.Asset.Slug, item.Asset.Name, item.Asset.Description, item.Asset.Tags), &projectAsset)
	if errors.Is(err, pgx.ErrNoRows) {
		if err := scanAsset(tx.QueryRow(ctx, `SELECT `+assetSelectColumns+` FROM agentic_assets WHERE scope='project' AND project_id=$1 AND type=$2 AND slug=$3`, projectID, item.Asset.Type, item.Asset.Slug), &projectAsset); err != nil {
			return applied, err
		}
		applied.Status = "already_exists"
		applied.ExistingAssetID = projectAsset.ID
		applied.Message = "project asset with same type and slug already exists"
		return applied, nil
	}
	if err != nil {
		return applied, err
	}

	metadata := item.Version.Metadata
	if len(metadata) == 0 {
		metadata = json.RawMessage(`{}`)
	}
	publishedAt := any(nil)
	if item.Version.Status == "published" {
		publishedAt = time.Now()
	}
	versionStatus := item.Version.Status
	if versionStatus == "" {
		versionStatus = "draft"
	}
	var projectVersion domain.AssetVersion
	if err := tx.QueryRow(ctx, `
		INSERT INTO agentic_asset_versions(asset_id,version,content,content_format,checksum,metadata,status,published_at)
		VALUES($1,$2,$3,$4,$5,$6::jsonb || jsonb_build_object('copied_from_asset_id',$7::text,'copied_from_asset_version_id',$8::text),$9,$10)
		RETURNING id, asset_id, version, content, content_format, checksum, metadata, status, created_at, published_at`,
		projectAsset.ID, item.Version.Version, item.Version.Content, item.Version.ContentFormat, item.Version.Checksum, metadata, item.Asset.ID, item.Version.ID, versionStatus, publishedAt).Scan(&projectVersion.ID, &projectVersion.AssetID, &projectVersion.Version, &projectVersion.Content, &projectVersion.ContentFormat, &projectVersion.Checksum, &projectVersion.Metadata, &projectVersion.Status, &projectVersion.CreatedAt, &projectVersion.PublishedAt); err != nil {
		return applied, err
	}
	if _, err := tx.Exec(ctx, `UPDATE agentic_assets SET current_version_id=$1, updated_at=now() WHERE id=$2`, projectVersion.ID, projectAsset.ID); err != nil {
		return applied, err
	}
	if item.TargetPath != "" {
		_, _ = tx.Exec(ctx, `
			INSERT INTO project_asset_bindings(project_id,asset_id,asset_version_id,target_path,sync_status,last_db_checksum)
			VALUES($1,$2,$3,$4,'missing',$5)
			ON CONFLICT(project_id,target_path) DO NOTHING`,
			projectID, projectAsset.ID, projectVersion.ID, item.TargetPath, projectVersion.Checksum)
	}
	applied.Status = "created"
	applied.ProjectAssetID = projectAsset.ID
	applied.ProjectAssetVersionID = projectVersion.ID
	return applied, nil
}

func (s Store) ListTaskRuns(ctx context.Context, projectID string) ([]domain.TaskRun, error) {
	rows, err := s.DB.Query(ctx, `SELECT id, project_id, workflow_template_id, external_run_id, title, input_summary, change_summary, final_summary, status, review_status, expected_workflow_id, expected_workflow_version, actual_workflow_id, actual_workflow_version, report_checksum, report_source_path, imported_at, import_status, started_at, finished_at, created_at, updated_at FROM task_runs WHERE project_id=$1 ORDER BY imported_at DESC NULLS LAST, created_at DESC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.TaskRun, 0)
	for rows.Next() {
		var tr domain.TaskRun
		if err := rows.Scan(&tr.ID, &tr.ProjectID, &tr.WorkflowTemplateID, &tr.ExternalRunID, &tr.Title, &tr.InputSummary, &tr.ChangeSummary, &tr.FinalSummary, &tr.Status, &tr.ReviewStatus, &tr.ExpectedWorkflowID, &tr.ExpectedWorkflowVersion, &tr.ActualWorkflowID, &tr.ActualWorkflowVersion, &tr.ReportChecksum, &tr.ReportSourcePath, &tr.ImportedAt, &tr.ImportStatus, &tr.StartedAt, &tr.FinishedAt, &tr.CreatedAt, &tr.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, tr)
	}
	return out, rows.Err()
}

func (s Store) GetTaskRun(ctx context.Context, id string) (domain.TaskRun, error) {
	var tr domain.TaskRun
	err := s.DB.QueryRow(ctx, `SELECT id, project_id, workflow_template_id, external_run_id, title, input_summary, change_summary, final_summary, status, review_status, expected_workflow_id, expected_workflow_version, actual_workflow_id, actual_workflow_version, report_checksum, report_source_path, imported_at, import_status, started_at, finished_at, created_at, updated_at FROM task_runs WHERE id=$1`, id).
		Scan(&tr.ID, &tr.ProjectID, &tr.WorkflowTemplateID, &tr.ExternalRunID, &tr.Title, &tr.InputSummary, &tr.ChangeSummary, &tr.FinalSummary, &tr.Status, &tr.ReviewStatus, &tr.ExpectedWorkflowID, &tr.ExpectedWorkflowVersion, &tr.ActualWorkflowID, &tr.ActualWorkflowVersion, &tr.ReportChecksum, &tr.ReportSourcePath, &tr.ImportedAt, &tr.ImportStatus, &tr.StartedAt, &tr.FinishedAt, &tr.CreatedAt, &tr.UpdatedAt)
	return tr, err
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}
