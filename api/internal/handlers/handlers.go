package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"

	"agentops-workspace/api/internal/domain"
	"agentops-workspace/api/internal/repo"
	"agentops-workspace/api/internal/services"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type Handler struct {
	Store    repo.Store
	Guard    services.PathGuard
	Scanner  services.RepoScanner
	Sync     services.SyncService
	Importer *services.RunImporter
	Playback services.PlaybackService
}

func (h Handler) Register(api fiber.Router) {
	api.Get("/workspace/default", h.defaultWorkspace)
	api.Get("/dashboard", h.dashboard)

	api.Get("/projects", h.listProjects)
	api.Post("/projects", h.createProject)
	api.Delete("/projects/:id", h.deleteProject)
	api.Get("/projects/:id", h.getProject)
	api.Post("/projects/:id/scan", h.scanProject)
	api.Get("/projects/:id/repo-tree", h.repoTree)
	api.Get("/projects/:id/repo-file", h.repoFile)
	api.Put("/projects/:id/repo-file", h.updateRepoFile)
	api.Get("/projects/:id/assets", h.projectAssets)
	api.Get("/projects/:id/presets/:preset_id/preview", h.previewProjectPreset)
	api.Post("/projects/:id/presets/:preset_id/apply", h.applyProjectPreset)
	api.Post("/projects/:id/sync/preview", h.syncPreview)
	api.Post("/projects/:id/sync/apply", h.syncApply)
	api.Post("/projects/:id/sync", h.syncApply)
	api.Get("/projects/:id/sync-runs", h.syncRuns)

	api.Get("/assets", h.listAssets)
	api.Post("/assets", h.createAsset)
	api.Get("/assets/:id", h.getAsset)
	api.Get("/assets/:id/versions", h.assetVersions)
	api.Post("/assets/:id/publish", h.publishAsset)
	api.Post("/assets/:id/clone", h.cloneAsset)
	api.Get("/presets", h.listPresets)
	api.Post("/presets", h.createPreset)
	api.Get("/presets/:id", h.getPreset)
	api.Put("/presets/:id", h.updatePreset)
	api.Post("/presets/:id/items", h.addPresetItem)
	api.Delete("/presets/:id/items/:item_id", h.removePresetItem)

	api.Post("/composer/preview", h.composerPreview)
	api.Post("/composer/apply", h.composerApply)

	api.Get("/workflows", h.listWorkflows)
	api.Post("/workflows", h.createWorkflow)
	api.Get("/workflows/:id", h.getWorkflow)
	api.Put("/workflows/:id", h.updateWorkflow)
	api.Post("/workflows/:id/preview-graph", h.previewWorkflowGraph)

	api.Get("/projects/:id/task-runs", h.taskRuns)
	api.Get("/projects/:id/reviews", h.projectReviews)
	api.Get("/reviews", h.reviews)
	api.Get("/task-runs/:id", h.getTaskRun)
	api.Get("/task-runs/:id/graph", h.playback)
	api.Get("/task-runs/:id/events", h.events)
	api.Get("/task-runs/:id/playback", h.playback)
	api.Get("/task-runs/:id/review", h.taskRunReview)
	api.Post("/task-runs/:id/review", h.createManualReview)

	api.Get("/projects/:id/reports/:run_id/playback", h.reportPlayback)
	api.Post("/projects/:id/runs/import", h.importRuns)
	api.Get("/projects/:id/runs/imports", h.importRecords)
}

func (h Handler) defaultWorkspace(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	return respond(c, ws, err)
}

func (h Handler) dashboard(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	data, err := h.Store.Dashboard(c.Context(), ws.ID)
	for k, v := range data {
		if ptr, ok := v.(*int); ok {
			data[k] = *ptr
		}
	}
	return respond(c, data, err)
}

func (h Handler) listProjects(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	items, err := h.Store.ListProjects(c.Context(), ws.ID)
	return respond(c, items, err)
}

func (h Handler) createProject(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	var req struct {
		domain.Project
		CreateRepoPath bool `json:"create_repo_path"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" || req.Slug == "" || req.RepoPath == "" {
		return fiber.NewError(http.StatusBadRequest, "name, slug, and repo_path are required")
	}
	safeRepo, err := h.Guard.ValidateRepoPathWithCreate(req.RepoPath, req.CreateRepoPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return structuredError(c, http.StatusBadRequest, "repo_path_not_found", "repository path does not exist")
		}
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	req.WorkspaceID = ws.ID
	req.RepoPath = safeRepo
	project, err := h.Store.CreateProject(c.Context(), req.Project)
	return respond(c, project, err)
}

func (h Handler) getProject(c *fiber.Ctx) error {
	p, err := h.Store.GetProject(c.Context(), c.Params("id"))
	return respond(c, p, err)
}

func (h Handler) deleteProject(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	deleted, err := h.Store.DeleteProject(c.Context(), ws.ID, c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	if !deleted {
		return fiber.NewError(http.StatusNotFound, "not found")
	}
	return respond(c, fiber.Map{"ok": true}, nil)
}

func (h Handler) scanProject(c *fiber.Ctx) error {
	p, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	result, err := h.Scanner.Detect(p.RepoPath)
	result.ProjectID = p.ID
	return respond(c, result, err)
}

func (h Handler) repoTree(c *fiber.Ctx) error {
	p, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	root, err := h.Scanner.Tree(p.RepoPath, c.QueryInt("max_depth", 5), c.QueryBool("show_managed_files", true))
	return respond(c, fiber.Map{"project_id": p.ID, "repo_path": p.RepoPath, "root": root}, err)
}

func (h Handler) repoFile(c *fiber.Ctx) error {
	p, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	target := strings.TrimSpace(c.Query("path"))
	if target == "" {
		return fiber.NewError(http.StatusBadRequest, "path is required")
	}
	full, err := h.Guard.SafeTargetForRead(p.RepoPath, target)
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	info, err := os.Stat(full)
	if err != nil {
		return respond(c, nil, err)
	}
	if info.IsDir() {
		return fiber.NewError(http.StatusBadRequest, "path is a directory")
	}
	if info.Size() > 512*1024 {
		return fiber.NewError(http.StatusBadRequest, "file is too large to preview")
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return respond(c, nil, err)
	}
	cleanTarget := strings.TrimPrefix(strings.ReplaceAll(target, "\\", "/"), "./")
	return respond(c, fiber.Map{
		"path":        cleanTarget,
		"name":        info.Name(),
		"size_bytes":  info.Size(),
		"content":     string(data),
		"modified_at": info.ModTime(),
		"can_write":   h.Guard.CanWriteTarget(cleanTarget),
	}, nil)
}

func (h Handler) updateRepoFile(c *fiber.Ctx) error {
	p, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	var req struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	target := strings.TrimSpace(req.Path)
	if target == "" {
		return fiber.NewError(http.StatusBadRequest, "path is required")
	}
	if len(req.Content) > 512*1024 {
		return fiber.NewError(http.StatusBadRequest, "file is too large to save")
	}
	full, err := h.Guard.SafeTarget(p.RepoPath, target)
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	if info, err := os.Stat(full); err == nil && info.IsDir() {
		return fiber.NewError(http.StatusBadRequest, "path is a directory")
	}
	if err := os.WriteFile(full, []byte(req.Content), 0644); err != nil {
		return respond(c, nil, err)
	}
	info, err := os.Stat(full)
	if err != nil {
		return respond(c, nil, err)
	}
	cleanTarget := strings.TrimPrefix(strings.ReplaceAll(target, "\\", "/"), "./")
	return respond(c, fiber.Map{
		"path":        cleanTarget,
		"name":        info.Name(),
		"size_bytes":  info.Size(),
		"content":     req.Content,
		"modified_at": info.ModTime(),
		"can_write":   h.Guard.CanWriteTarget(cleanTarget),
	}, nil)
}

func (h Handler) syncPreview(c *fiber.Ctx) error {
	p, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	var req services.SyncRequest
	_ = c.BodyParser(&req)
	res, err := h.Sync.PreviewWithRequest(c.Context(), p, req)
	return respond(c, res, err)
}

func (h Handler) syncApply(c *fiber.Ctx) error {
	p, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	var req services.SyncRequest
	_ = c.BodyParser(&req)
	req.ActorType = defaultString(req.ActorType, "operator")
	req.Transport = defaultString(req.Transport, "http_api")
	switch strings.ToUpper(req.Mode) {
	case "REPO_TO_DB":
		res, err := h.Sync.ApplyRepoToDBWithRequest(c.Context(), p, req)
		return respond(c, res, err)
	case "COMPARE_ONLY":
		res, err := h.Sync.PreviewWithRequest(c.Context(), p, req)
		return respond(c, res, err)
	default:
		res, err := h.Sync.ApplyDBToRepoWithRequest(c.Context(), p, req)
		return respond(c, res, err)
	}
}

func (h Handler) syncRuns(c *fiber.Ctx) error {
	rows, err := h.Store.DB.Query(c.Context(), `SELECT id,direction,status,summary,actor_type,actor_name,transport,started_at,finished_at FROM sync_runs WHERE project_id=$1 ORDER BY started_at DESC LIMIT 25`, c.Params("id"))
	return rowsToMaps(c, rows, err, []string{"id", "direction", "status", "summary", "actor_type", "actor_name", "transport", "started_at", "finished_at"})
}

func (h Handler) listAssets(c *fiber.Ctx) error {
	scope := strings.ToLower(strings.TrimSpace(c.Query("scope", "template")))
	if scope != "" && scope != "template" {
		return structuredError(c, http.StatusBadRequest, "invalid_scope", "assets supports scope=template; use /api/projects/:id/assets for project assets")
	}
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	items, err := h.Store.ListAssets(c.Context(), ws.ID, c.Query("query"), c.Query("type"))
	return respond(c, items, err)
}

type assetRequest struct {
	Type          string   `json:"type"`
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Version       string   `json:"version"`
	Content       string   `json:"content"`
	ContentFormat string   `json:"content_format"`
	Status        string   `json:"status"`
	Tags          []string `json:"tags"`
}

func (h Handler) createAsset(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	var req assetRequest
	if err := c.BodyParser(&req); err != nil {
		return structuredError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	req.Type = strings.TrimSpace(req.Type)
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)
	if req.Type == "" || req.Slug == "" || req.Name == "" {
		return structuredError(c, http.StatusBadRequest, "invalid_request", "type, slug, and name are required")
	}
	if req.Version == "" {
		req.Version = "0.1.0"
	}
	if req.ContentFormat == "" {
		req.ContentFormat = "markdown"
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	a, v, err := h.Store.UpsertAssetWithVersion(c.Context(), ws.ID, req.Type, req.Slug, req.Name, req.Description, req.Version, req.Content, req.ContentFormat, req.Status, services.SHA256String(req.Content), req.Tags)
	return respond(c, fiber.Map{"asset": a, "version": v}, err)
}

func (h Handler) listPresets(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	presets, err := h.Store.ListAssetPresets(c.Context(), ws.ID)
	return respond(c, presets, err)
}

func (h Handler) getPreset(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	detail, err := h.Store.GetAssetPreset(c.Context(), ws.ID, c.Params("id"))
	return respond(c, presetDetailResponse(detail), err)
}

type presetRequest struct {
	Slug           string   `json:"slug"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Status         string   `json:"status"`
	CurrentVersion int      `json:"current_version"`
	Tags           []string `json:"tags"`
}

func (h Handler) createPreset(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	var req presetRequest
	if err := c.BodyParser(&req); err != nil {
		return structuredError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)
	if req.Slug == "" || req.Name == "" {
		return structuredError(c, http.StatusBadRequest, "invalid_request", "slug and name are required")
	}
	detail, err := h.Store.UpsertAssetPreset(c.Context(), ws.ID, "", req.Slug, req.Name, req.Description, req.Status, req.CurrentVersion, req.Tags)
	return respond(c, presetDetailResponse(detail), err)
}

func (h Handler) updatePreset(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	var req presetRequest
	if err := c.BodyParser(&req); err != nil {
		return structuredError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	req.Slug = strings.TrimSpace(req.Slug)
	req.Name = strings.TrimSpace(req.Name)
	if req.Slug == "" || req.Name == "" {
		return structuredError(c, http.StatusBadRequest, "invalid_request", "slug and name are required")
	}
	detail, err := h.Store.UpsertAssetPreset(c.Context(), ws.ID, c.Params("id"), req.Slug, req.Name, req.Description, req.Status, req.CurrentVersion, req.Tags)
	return respond(c, presetDetailResponse(detail), err)
}

func (h Handler) addPresetItem(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	var req struct {
		TemplateAssetID        string `json:"template_asset_id"`
		TemplateAssetVersionID string `json:"template_asset_version_id"`
		TargetPath             string `json:"target_path"`
		Required               bool   `json:"required"`
		SortOrder              int    `json:"sort_order"`
	}
	if err := c.BodyParser(&req); err != nil {
		return structuredError(c, http.StatusBadRequest, "invalid_request", err.Error())
	}
	if req.TemplateAssetID == "" || req.TemplateAssetVersionID == "" {
		return structuredError(c, http.StatusBadRequest, "invalid_request", "template_asset_id and template_asset_version_id are required")
	}
	detail, err := h.Store.AddAssetPresetItem(c.Context(), ws.ID, c.Params("id"), req.TemplateAssetID, req.TemplateAssetVersionID, strings.TrimSpace(req.TargetPath), req.Required, req.SortOrder)
	return respond(c, presetDetailResponse(detail), err)
}

func (h Handler) removePresetItem(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	detail, err := h.Store.RemoveAssetPresetItem(c.Context(), ws.ID, c.Params("id"), c.Params("item_id"))
	return respond(c, presetDetailResponse(detail), err)
}

func (h Handler) projectAssets(c *fiber.Ctx) error {
	project, err := h.defaultWorkspaceProject(c)
	if err != nil {
		return respond(c, nil, err)
	}
	items, err := h.Store.ListProjectAssets(c.Context(), project.ID)
	return respond(c, items, err)
}

func (h Handler) previewProjectPreset(c *fiber.Ctx) error {
	project, err := h.defaultWorkspaceProject(c)
	if err != nil {
		return respond(c, nil, err)
	}
	detail, err := h.Store.GetAssetPreset(c.Context(), project.WorkspaceID, c.Params("preset_id"))
	if err != nil {
		return respond(c, nil, err)
	}
	items, err := h.projectPresetPreviewItems(c.Context(), project, detail)
	return respond(c, fiber.Map{"project_id": project.ID, "preset_id": detail.ID, "preset": presetDetailResponse(detail), "items": items}, err)
}

func (h Handler) applyProjectPreset(c *fiber.Ctx) error {
	project, err := h.defaultWorkspaceProject(c)
	if err != nil {
		return respond(c, nil, err)
	}
	result, err := h.Store.ApplyAssetPresetToProject(c.Context(), project.WorkspaceID, project.ID, c.Params("preset_id"))
	return respond(c, result, err)
}

func (h Handler) getAsset(c *fiber.Ctx) error {
	a, err := h.Store.GetAsset(c.Context(), c.Params("id"))
	return respond(c, a, err)
}

func (h Handler) assetVersions(c *fiber.Ctx) error {
	v, err := h.Store.ListAssetVersions(c.Context(), c.Params("id"))
	return respond(c, v, err)
}

func (h Handler) publishAsset(c *fiber.Ctx) error {
	var req struct {
		VersionID string `json:"version_id"`
	}
	_ = c.BodyParser(&req)
	if req.VersionID == "" {
		return fiber.NewError(http.StatusBadRequest, "version_id is required")
	}
	return respond(c, fiber.Map{"ok": true}, h.Store.PublishAssetVersion(c.Context(), c.Params("id"), req.VersionID))
}

func (h Handler) cloneAsset(c *fiber.Ctx) error {
	var req struct {
		Slug    string `json:"slug"`
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	_ = c.BodyParser(&req)
	source, err := h.Store.GetAsset(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	versions, err := h.Store.ListAssetVersions(c.Context(), source.ID)
	if err != nil {
		return respond(c, nil, err)
	}
	if len(versions) == 0 {
		return fiber.NewError(http.StatusBadRequest, "asset has no versions to clone")
	}
	if req.Slug == "" {
		req.Slug = source.Slug + "-copy"
	}
	if req.Name == "" {
		req.Name = source.Name + " Copy"
	}
	if req.Version == "" {
		req.Version = "0.1.0"
	}
	a, v, err := h.Store.UpsertAssetWithVersion(c.Context(), source.WorkspaceID, source.Type, req.Slug, req.Name, source.Description, req.Version, versions[0].Content, versions[0].ContentFormat, "draft", services.SHA256String(versions[0].Content), []string{"clone"})
	return respond(c, fiber.Map{"asset": a, "version": v}, err)
}

func (h Handler) composerPreview(c *fiber.Ctx) error {
	var req struct {
		ProjectID   string   `json:"project_id"`
		AssetIDs    []string `json:"asset_ids"`
		AssetTypes  []string `json:"asset_types"`
		TargetPaths []string `json:"target_paths"`
	}
	_ = c.BodyParser(&req)
	if req.ProjectID == "" {
		return structuredError(c, http.StatusBadRequest, "project_required", "project_id is required; apply a preset to copy templates into project assets first")
	}
	assets, err := h.Store.CurrentProjectAssetVersions(c.Context(), req.ProjectID)
	if err != nil {
		return respond(c, nil, err)
	}
	var files []fiber.Map
	for _, av := range assets {
		if len(req.AssetIDs) > 0 && !contains(req.AssetIDs, av.Asset.ID) {
			continue
		}
		if len(req.AssetTypes) > 0 && !contains(req.AssetTypes, av.Asset.Type) {
			continue
		}
		target := services.TargetPathForAsset(av.Asset.Type, av.Asset.Slug)
		if target == "" {
			continue
		}
		if len(req.TargetPaths) > 0 && !contains(req.TargetPaths, target) {
			continue
		}
		rendered := services.RenderManagedFile(av.Asset, av.Version)
		files = append(files, fiber.Map{"asset": av.Asset, "version": av.Version.Version, "target_path": target, "checksum": services.SHA256String(rendered), "content_preview": rendered})
	}
	return c.JSON(fiber.Map{"files": files})
}

func (h Handler) composerApply(c *fiber.Ctx) error {
	var req struct {
		ProjectID   string   `json:"project_id"`
		AssetIDs    []string `json:"asset_ids"`
		AssetTypes  []string `json:"asset_types"`
		TargetPaths []string `json:"target_paths"`
	}
	_ = c.BodyParser(&req)
	if req.ProjectID == "" {
		return h.composerPreview(c)
	}
	project, err := h.Store.GetProject(c.Context(), req.ProjectID)
	if err != nil {
		return respond(c, nil, err)
	}
	res, err := h.Sync.ApplyDBToRepoWithRequest(c.Context(), project, services.SyncRequest{Mode: "DB_TO_REPO", AssetIDs: req.AssetIDs, AssetTypes: req.AssetTypes, TargetPaths: req.TargetPaths})
	return respond(c, res, err)
}

func (h Handler) listWorkflows(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	rows, err := h.Store.DB.Query(c.Context(), `SELECT id,name,slug,version,status,created_at,updated_at FROM workflow_templates WHERE workspace_id=$1 ORDER BY slug, version`, ws.ID)
	return rowsToMaps(c, rows, err, []string{"id", "name", "slug", "version", "status", "created_at", "updated_at"})
}

func (h Handler) createWorkflow(c *fiber.Ctx) error {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return respond(c, nil, err)
	}
	var req struct {
		Name           string `json:"name"`
		Slug           string `json:"slug"`
		Version        string `json:"version"`
		DefinitionYAML string `json:"definition_yaml"`
		Status         string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	def, err := services.ParseWorkflowDefinition(req.DefinitionYAML)
	if err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	if req.Name == "" {
		req.Name = def.Name
	}
	if req.Slug == "" {
		req.Slug = def.ID
	}
	if req.Version == "" {
		req.Version = def.Version
	}
	if req.Status == "" {
		req.Status = "draft"
	}
	var id string
	err = h.Store.DB.QueryRow(c.Context(), `
		INSERT INTO workflow_templates(workspace_id,name,slug,version,definition_yaml,status)
		VALUES($1,$2,$3,$4,$5,$6)
		ON CONFLICT(workspace_id,slug,version) DO UPDATE SET name=EXCLUDED.name, definition_yaml=EXCLUDED.definition_yaml, status=EXCLUDED.status, updated_at=now()
		RETURNING id`, ws.ID, req.Name, req.Slug, req.Version, req.DefinitionYAML, req.Status).Scan(&id)
	return respond(c, fiber.Map{"id": id, "name": req.Name, "slug": req.Slug, "version": req.Version, "status": req.Status}, err)
}
func (h Handler) updateWorkflow(c *fiber.Ctx) error {
	var req struct {
		Name           string `json:"name"`
		DefinitionYAML string `json:"definition_yaml"`
		Status         string `json:"status"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	if req.DefinitionYAML != "" {
		if _, err := services.ParseWorkflowDefinition(req.DefinitionYAML); err != nil {
			return fiber.NewError(http.StatusBadRequest, err.Error())
		}
	}
	_, err := h.Store.DB.Exec(c.Context(), `UPDATE workflow_templates SET name=COALESCE(NULLIF($2,''),name), definition_yaml=COALESCE(NULLIF($3,''),definition_yaml), status=COALESCE(NULLIF($4,''),status), updated_at=now() WHERE id=$1`, c.Params("id"), req.Name, req.DefinitionYAML, req.Status)
	return respond(c, fiber.Map{"ok": true}, err)
}
func (h Handler) getWorkflow(c *fiber.Ctx) error {
	var item map[string]any
	rows, err := h.Store.DB.Query(c.Context(), `SELECT id,workspace_id,name,slug,version,definition_yaml,status,created_at,updated_at FROM workflow_templates WHERE id=$1`, c.Params("id"))
	items, err := collectRowsWithErr(rows, err, []string{"id", "workspace_id", "name", "slug", "version", "definition_yaml", "status", "created_at", "updated_at"})
	if err == nil && len(items) > 0 {
		item = items[0]
	}
	if item == nil && err == nil {
		err = pgx.ErrNoRows
	}
	return respond(c, item, err)
}
func (h Handler) previewWorkflowGraph(c *fiber.Ctx) error {
	var req struct {
		DefinitionYAML string `json:"definition_yaml"`
	}
	_ = c.BodyParser(&req)
	if req.DefinitionYAML == "" {
		var yamlText string
		if err := h.Store.DB.QueryRow(c.Context(), `SELECT definition_yaml FROM workflow_templates WHERE id=$1`, c.Params("id")).Scan(&yamlText); err != nil {
			return respond(c, nil, err)
		}
		req.DefinitionYAML = yamlText
	}
	graph, err := services.WorkflowGraphPreview(req.DefinitionYAML)
	return respond(c, graph, err)
}

func (h Handler) taskRuns(c *fiber.Ctx) error {
	project, summary, err := h.maybeAutoImport(c.Context(), c.Params("id"), c.QueryBool("auto_import", true))
	if err != nil {
		return respond(c, nil, err)
	}
	items, err := h.Store.ListTaskRuns(c.Context(), project.ID)
	return respond(c, fiber.Map{"auto_import": summary, "items": items}, err)
}

func (h Handler) projectReviews(c *fiber.Ctx) error {
	project, summary, err := h.maybeAutoImport(c.Context(), c.Params("id"), c.QueryBool("auto_import", true))
	if err != nil {
		return respond(c, nil, err)
	}
	rows, err := h.Store.DB.Query(c.Context(), `SELECT tr.id,tr.external_run_id,tr.title,tr.review_status,r.summary,r.created_at FROM task_runs tr LEFT JOIN LATERAL (SELECT summary,created_at FROM task_run_reviews WHERE task_run_id=tr.id ORDER BY created_at DESC LIMIT 1) r ON true WHERE tr.project_id=$1 ORDER BY tr.imported_at DESC`, project.ID)
	if err != nil {
		return respond(c, nil, err)
	}
	items, err := collectRows(rows, []string{"task_run_id", "external_run_id", "title", "review_status", "summary", "created_at"})
	return respond(c, fiber.Map{"auto_import": summary, "items": items}, err)
}

func (h Handler) reviews(c *fiber.Ctx) error {
	rows, err := h.Store.DB.Query(c.Context(), `SELECT tr.id,tr.external_run_id,tr.title,tr.review_status,p.name AS project_name,r.summary,r.created_at FROM task_runs tr JOIN projects p ON p.id=tr.project_id LEFT JOIN LATERAL (SELECT summary,created_at FROM task_run_reviews WHERE task_run_id=tr.id ORDER BY created_at DESC LIMIT 1) r ON true WHERE p.status <> 'deleted' ORDER BY tr.imported_at DESC LIMIT 100`)
	return rowsToMaps(c, rows, err, []string{"task_run_id", "external_run_id", "title", "review_status", "project_name", "summary", "created_at"})
}

func (h Handler) getTaskRun(c *fiber.Ctx) error {
	tr, err := h.Store.GetTaskRun(c.Context(), c.Params("id"))
	return respond(c, tr, err)
}

func (h Handler) playback(c *fiber.Ctx) error {
	tr, err := h.Store.GetTaskRun(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	var summary *domain.ImportSummary
	if c.QueryBool("auto_import", true) {
		project, err := h.Store.GetProject(c.Context(), tr.ProjectID)
		if err == nil {
			s, _ := h.Importer.AutoImport(c.Context(), project)
			summary = &s
			tr, _ = h.Store.GetTaskRun(c.Context(), c.Params("id"))
		}
	}
	res, err := h.Playback.Build(c.Context(), tr, summary)
	return respond(c, res, err)
}

func (h Handler) events(c *fiber.Ctx) error {
	rows, err := h.Store.DB.Query(c.Context(), `SELECT node_key,event_type,event_time,summary,metadata FROM task_run_node_events WHERE task_run_id=$1 ORDER BY event_time`, c.Params("id"))
	return rowsToMaps(c, rows, err, []string{"node_key", "event_type", "event_time", "summary", "metadata"})
}

func (h Handler) taskRunReview(c *fiber.Ctx) error {
	rows, err := h.Store.DB.Query(c.Context(), `SELECT id,review_status,summary,violations_json,warnings_json,created_at FROM task_run_reviews WHERE task_run_id=$1 ORDER BY created_at DESC`, c.Params("id"))
	return rowsToMaps(c, rows, err, []string{"id", "review_status", "summary", "violations", "warnings", "created_at"})
}

func (h Handler) createManualReview(c *fiber.Ctx) error {
	var req struct {
		ReviewStatus string           `json:"review_status"`
		Summary      string           `json:"summary"`
		Violations   []map[string]any `json:"violations"`
		Warnings     []map[string]any `json:"warnings"`
	}
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(http.StatusBadRequest, err.Error())
	}
	if req.ReviewStatus == "" {
		req.ReviewStatus = "needs_manual_review"
	}
	violations, _ := json.Marshal(req.Violations)
	warnings, _ := json.Marshal(req.Warnings)
	_, err := h.Store.DB.Exec(c.Context(), `INSERT INTO task_run_reviews(task_run_id,review_status,reviewer_type,summary,violations_json,warnings_json) VALUES($1,$2,'manual',$3,$4,$5)`, c.Params("id"), req.ReviewStatus, req.Summary, violations, warnings)
	if err == nil {
		_, err = h.Store.DB.Exec(c.Context(), `UPDATE task_runs SET review_status=$1, updated_at=now() WHERE id=$2`, req.ReviewStatus, c.Params("id"))
	}
	return respond(c, fiber.Map{"ok": true}, err)
}

func (h Handler) reportPlayback(c *fiber.Ctx) error {
	project, summary, err := h.maybeAutoImport(c.Context(), c.Params("id"), c.QueryBool("auto_import", true))
	if err != nil {
		return respond(c, nil, err)
	}
	var taskRunID string
	err = h.Store.DB.QueryRow(c.Context(), `SELECT id FROM task_runs WHERE project_id=$1 AND external_run_id=$2`, project.ID, c.Params("run_id")).Scan(&taskRunID)
	if err != nil {
		return respond(c, nil, err)
	}
	tr, err := h.Store.GetTaskRun(c.Context(), taskRunID)
	if err != nil {
		return respond(c, nil, err)
	}
	res, err := h.Playback.Build(c.Context(), tr, &summary)
	return respond(c, res, err)
}

func (h Handler) importRuns(c *fiber.Ctx) error {
	project, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return respond(c, nil, err)
	}
	summary, err := h.Importer.AutoImport(c.Context(), project)
	return respond(c, fiber.Map{"auto_import": summary}, err)
}

func (h Handler) importRecords(c *fiber.Ctx) error {
	rows, err := h.Store.DB.Query(c.Context(), `SELECT id,run_id,source_path,source_checksum,import_revision,import_status,error_message,imported_at,created_at FROM task_run_imports WHERE project_id=$1 ORDER BY created_at DESC LIMIT 100`, c.Params("id"))
	return rowsToMaps(c, rows, err, []string{"id", "run_id", "source_path", "source_checksum", "import_revision", "import_status", "error_message", "imported_at", "created_at"})
}

func (h Handler) maybeAutoImport(ctx context.Context, projectID string, enabled bool) (domain.Project, domain.ImportSummary, error) {
	project, err := h.Store.GetProject(ctx, projectID)
	if err != nil {
		return project, domain.ImportSummary{}, err
	}
	if !enabled {
		return project, domain.ImportSummary{}, nil
	}
	summary, err := h.Importer.AutoImport(ctx, project)
	return project, summary, err
}

type assetPresetDef struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	AssetTypes  []string `json:"asset_types"`
}

type presetAssetVersion struct {
	ID            string `json:"id"`
	AssetID       string `json:"asset_id"`
	Version       string `json:"version"`
	Content       string `json:"-"`
	ContentFormat string `json:"content_format"`
	Checksum      string `json:"checksum"`
	Status        string `json:"status"`
}

type presetTemplateItem struct {
	PresetItemID  string             `json:"preset_item_id"`
	TargetPath    string             `json:"target_path"`
	TemplateAsset domain.Asset       `json:"template_asset"`
	PinnedVersion presetAssetVersion `json:"pinned_version"`
}

type projectPresetItem struct {
	PresetItemID   string             `json:"preset_item_id"`
	TargetPath     string             `json:"target_path"`
	Action         string             `json:"action"`
	Status         string             `json:"status"`
	Conflict       string             `json:"conflict,omitempty"`
	ProjectAssetID string             `json:"project_asset_id,omitempty"`
	Checksum       string             `json:"checksum"`
	TemplateAsset  domain.Asset       `json:"template_asset"`
	PinnedVersion  presetAssetVersion `json:"pinned_version"`
	ContentPreview string             `json:"content_preview,omitempty"`
}

var assetPresetCatalog = []assetPresetDef{
	{
		ID:          "agentops-core",
		Name:        "AgentOps Core",
		Description: "Recommended agent instructions, workflow, policies, skills, README, and run-report contract.",
		AssetTypes:  []string{"agents_md", "readme", "skill_doc", "policy_doc", "workflow_doc", "run_report_contract"},
	},
	{
		ID:          "reporting-minimal",
		Name:        "Reporting Minimal",
		Description: "Only the top-level agent instruction file and run-report contract.",
		AssetTypes:  []string{"agents_md", "run_report_contract"},
	},
	{
		ID:          "agentops-full",
		Name:        "AgentOps Full",
		Description: "All published template assets with a materializable target path.",
		AssetTypes:  []string{"agents_md", "readme", "skill_doc", "subagent_doc", "policy_doc", "workflow_doc", "prompt_template", "checklist", "run_report_contract"},
	},
}

func assetPresetByID(id string) (assetPresetDef, bool) {
	for _, preset := range assetPresetCatalog {
		if preset.ID == id {
			return preset, true
		}
	}
	return assetPresetDef{}, false
}

func presetResponse(preset assetPresetDef) fiber.Map {
	return fiber.Map{"id": preset.ID, "name": preset.Name, "description": preset.Description, "asset_types": preset.AssetTypes}
}

func presetDetailResponse(detail domain.AssetPresetDetail) fiber.Map {
	items := make([]fiber.Map, 0, len(detail.Items))
	for _, item := range detail.Items {
		items = append(items, fiber.Map{
			"id":                     item.ID,
			"preset_id":              item.PresetID,
			"template_asset":         item.Asset,
			"template_asset_version": item.Version,
			"target_path":            item.TargetPath,
			"required":               item.Required,
			"sort_order":             item.SortOrder,
		})
	}
	return fiber.Map{
		"id":              detail.ID,
		"workspace_id":    detail.WorkspaceID,
		"slug":            detail.Slug,
		"name":            detail.Name,
		"description":     detail.Description,
		"status":          detail.Status,
		"current_version": detail.CurrentVersion,
		"tags":            detail.Tags,
		"item_count":      detail.ItemCount,
		"items":           items,
		"created_at":      detail.CreatedAt,
		"updated_at":      detail.UpdatedAt,
	}
}

func (h Handler) defaultWorkspaceProject(c *fiber.Ctx) (domain.Project, error) {
	ws, err := h.Store.DefaultWorkspace(c.Context())
	if err != nil {
		return domain.Project{}, err
	}
	project, err := h.Store.GetProject(c.Context(), c.Params("id"))
	if err != nil {
		return domain.Project{}, err
	}
	if project.WorkspaceID != ws.ID {
		return domain.Project{}, pgx.ErrNoRows
	}
	return project, nil
}

func (h Handler) projectPresetPreviewItems(ctx context.Context, project domain.Project, detail domain.AssetPresetDetail) ([]fiber.Map, error) {
	projectAssets, err := h.Store.ListProjectAssets(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	existing := map[string]string{}
	for _, asset := range projectAssets {
		existing[asset.Type+"/"+asset.Slug] = asset.ID
	}
	bindings, err := h.Store.ListProjectAssetBindings(ctx, project.ID, "", "")
	if err != nil {
		return nil, err
	}
	for _, binding := range bindings {
		if binding.TargetPath != "" {
			existing["target:"+binding.TargetPath] = binding.Asset.ID
		}
	}
	items := make([]fiber.Map, 0, len(detail.Items))
	for _, item := range detail.Items {
		status := "missing"
		action := "copy"
		alreadyExists := false
		projectAssetID := ""
		if id := existing[item.Asset.Type+"/"+item.Asset.Slug]; id != "" {
			status = "already_exists"
			action = "already_exists"
			alreadyExists = true
			projectAssetID = id
		}
		if item.TargetPath != "" {
			if id := existing["target:"+item.TargetPath]; id != "" {
				status = "already_exists"
				action = "already_exists"
				alreadyExists = true
				projectAssetID = id
			}
		}
		items = append(items, fiber.Map{
			"preset_item_id":         item.ID,
			"template_asset":         item.Asset,
			"template_asset_version": item.Version,
			"project_asset_id":       projectAssetID,
			"target_path":            item.TargetPath,
			"required":               item.Required,
			"sort_order":             item.SortOrder,
			"status":                 status,
			"action":                 action,
			"already_exists":         alreadyExists,
		})
	}
	return items, nil
}

func (h Handler) projectPreset(c *fiber.Ctx) (domain.Project, assetPresetDef, error) {
	project, err := h.defaultWorkspaceProject(c)
	if err != nil {
		return domain.Project{}, assetPresetDef{}, err
	}
	preset, ok := assetPresetByID(c.Params("preset_id"))
	if !ok {
		return domain.Project{}, assetPresetDef{}, pgx.ErrNoRows
	}
	return project, preset, nil
}

func (h Handler) resolvePresetItems(ctx context.Context, workspaceID string, preset assetPresetDef) ([]presetTemplateItem, error) {
	versions, err := h.Store.CurrentAssetVersions(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	items := []presetTemplateItem{}
	for _, av := range versions {
		if !contains(preset.AssetTypes, av.Asset.Type) {
			continue
		}
		target := services.TargetPathForAsset(av.Asset.Type, av.Asset.Slug)
		if target == "" {
			continue
		}
		items = append(items, presetTemplateItem{
			PresetItemID:  av.Asset.Type + "/" + av.Asset.Slug,
			TargetPath:    target,
			TemplateAsset: av.Asset,
			PinnedVersion: presetAssetVersion{
				ID:            av.Version.ID,
				AssetID:       av.Version.AssetID,
				Version:       av.Version.Version,
				Content:       av.Version.Content,
				ContentFormat: av.Version.ContentFormat,
				Checksum:      av.Version.Checksum,
				Status:        av.Version.Status,
			},
		})
	}
	return items, nil
}

func (h Handler) projectPresetPlan(ctx context.Context, project domain.Project, preset assetPresetDef) ([]projectPresetItem, error) {
	templateItems, err := h.resolvePresetItems(ctx, project.WorkspaceID, preset)
	if err != nil {
		return nil, err
	}
	projectAssets, err := h.Store.ListProjectAssets(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	existingTargets := map[string]string{}
	for _, item := range projectAssets {
		existingTargets[item.Type+"/"+item.Slug] = item.ID
	}
	out := make([]projectPresetItem, 0, len(templateItems))
	for _, item := range templateItems {
		version := domain.AssetVersion{
			ID:            item.PinnedVersion.ID,
			AssetID:       item.PinnedVersion.AssetID,
			Version:       item.PinnedVersion.Version,
			Content:       item.PinnedVersion.Content,
			ContentFormat: item.PinnedVersion.ContentFormat,
			Checksum:      item.PinnedVersion.Checksum,
			Status:        item.PinnedVersion.Status,
		}
		rendered := services.RenderManagedFile(item.TemplateAsset, version)
		planned := projectPresetItem{
			PresetItemID:   item.PresetItemID,
			TargetPath:     item.TargetPath,
			Action:         "copy",
			Status:         "ready",
			Checksum:       services.SHA256String(rendered),
			TemplateAsset:  item.TemplateAsset,
			PinnedVersion:  item.PinnedVersion,
			ContentPreview: rendered,
		}
		if bindingID, ok := existingTargets[item.TemplateAsset.Type+"/"+item.TemplateAsset.Slug]; ok {
			planned.Action = "skip"
			planned.Status = "skipped"
			planned.Conflict = "already_exists"
			planned.ProjectAssetID = bindingID
		}
		out = append(out, planned)
	}
	return out, nil
}

func presetPlanSummary(items []projectPresetItem) fiber.Map {
	summary := fiber.Map{"total": len(items), "copy": 0, "created": 0, "skipped": 0, "already_exists": 0}
	for _, item := range items {
		if item.Action == "copy" {
			summary["copy"] = summary["copy"].(int) + 1
		}
		if item.Status == "created" {
			summary["created"] = summary["created"].(int) + 1
		}
		if item.Status == "skipped" {
			summary["skipped"] = summary["skipped"].(int) + 1
		}
		if item.Conflict == "already_exists" {
			summary["already_exists"] = summary["already_exists"].(int) + 1
		}
	}
	return summary
}

func structuredError(c *fiber.Ctx, status int, code, message string) error {
	return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": message}})
}

func respond(c *fiber.Ctx, data any, err error) error {
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(http.StatusNotFound, "not found")
		}
		if errors.Is(err, os.ErrNotExist) {
			return fiber.NewError(http.StatusNotFound, err.Error())
		}
		return fiber.NewError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(data)
}

func rowsToMaps(c *fiber.Ctx, rows pgx.Rows, err error, columns []string) error {
	if err != nil {
		return respond(c, nil, err)
	}
	items, err := collectRows(rows, columns)
	return respond(c, items, err)
}

func collectRowsWithErr(rows pgx.Rows, err error, columns []string) ([]map[string]any, error) {
	if err != nil {
		return nil, err
	}
	return collectRows(rows, columns)
}

func collectRows(rows pgx.Rows, columns []string) ([]map[string]any, error) {
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		values := make([]any, len(columns))
		ptrs := make([]any, len(columns))
		for i := range values {
			ptrs[i] = &values[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		item := map[string]any{}
		for i, col := range columns {
			item[col] = values[i]
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
