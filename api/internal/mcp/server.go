package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"agentops-workspace/api/internal/config"
	"agentops-workspace/api/internal/domain"
	"agentops-workspace/api/internal/repo"
	"agentops-workspace/api/internal/services"

	"github.com/gofiber/fiber/v2"
)

type Server struct {
	Config   config.Config
	Store    repo.Store
	Sync     services.SyncService
	Importer *services.RunImporter
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string    `json:"jsonrpc"`
	ID      any       `json:"id,omitempty"`
	Result  any       `json:"result,omitempty"`
	Error   *rpcError `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

func (s Server) Register(app *fiber.App) {
	app.Get("/mcp/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"ok": true, "server": "agentops-mcp"})
	})
	app.Post("/mcp", s.handle)
}

func (s Server) handle(c *fiber.Ctx) error {
	if !s.authorized(c) {
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
	}
	var req rpcRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return c.Status(http.StatusBadRequest).JSON(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
	}
	res := rpcResponse{JSONRPC: "2.0", ID: req.ID}
	result, err := s.dispatch(c.Context(), req)
	if err != nil {
		res.Error = &rpcError{Code: -32603, Message: err.Error()}
	} else {
		res.Result = result
	}
	return c.JSON(res)
}

func (s Server) authorized(c *fiber.Ctx) bool {
	if s.Config.MCPToken == "" {
		return true
	}
	auth := strings.TrimSpace(c.Get("Authorization"))
	if strings.EqualFold(auth, "Bearer "+s.Config.MCPToken) {
		return true
	}
	return c.Get("X-AgentOps-MCP-Token") == s.Config.MCPToken
}

func (s Server) dispatch(ctx context.Context, req rpcRequest) (any, error) {
	switch req.Method {
	case "initialize":
		return map[string]any{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]any{
				"tools": map[string]any{},
			},
			"serverInfo": map[string]string{
				"name":    "agentops",
				"version": "0.1.0",
			},
		}, nil
	case "tools/list":
		return map[string]any{"tools": tools()}, nil
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid tools/call params: %w", err)
		}
		return s.callTool(ctx, params)
	default:
		return nil, fmt.Errorf("unsupported MCP method %q", req.Method)
	}
}

func (s Server) callTool(ctx context.Context, params toolCallParams) (any, error) {
	switch params.Name {
	case "agentops_get_project_manifest":
		var args projectArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, err
		}
		project, err := s.lookupProject(ctx, args)
		if err != nil {
			return nil, err
		}
		var manifest map[string]any
		if err := json.Unmarshal([]byte(s.Sync.ProjectManifest(project)), &manifest); err != nil {
			return nil, err
		}
		return toolJSON(map[string]any{
			"project":        project,
			"filesystem":     "codex",
			"manifest_path":  ".codex/project.json",
			"lock_path":      ".codex/sync/lock.json",
			"manifest":       manifest,
			"managed_roots":  []string{"AGENTS.md", "README.md", ".codex/**"},
			"report_pattern": ".codex/reports/runs/{run_id}/run.report.json",
		})
	case "agentops_get_sync_status":
		var args projectArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, err
		}
		project, err := s.lookupProject(ctx, args)
		if err != nil {
			return nil, err
		}
		rows, err := s.Store.DB.Query(ctx, `SELECT id,direction,status,summary,actor_type,actor_name,transport,started_at,finished_at FROM sync_runs WHERE project_id=$1 ORDER BY started_at DESC LIMIT 10`, project.ID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		runs := []map[string]any{}
		for rows.Next() {
			var id, direction, status, actorType, actorName, transport string
			var summary *string
			var startedAt, finishedAt any
			if err := rows.Scan(&id, &direction, &status, &summary, &actorType, &actorName, &transport, &startedAt, &finishedAt); err != nil {
				return nil, err
			}
			runs = append(runs, map[string]any{"id": id, "direction": direction, "status": status, "summary": summary, "actor_type": actorType, "actor_name": actorName, "transport": transport, "started_at": startedAt, "finished_at": finishedAt})
		}
		return toolJSON(map[string]any{"project_id": project.ID, "sync_runs": runs})
	case "agentops_preview_files_to_db":
		var args syncArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, err
		}
		project, err := s.lookupProject(ctx, args.projectArgs)
		if err != nil {
			return nil, err
		}
		res, err := s.Sync.PreviewWithRequest(ctx, project, services.SyncRequest{Mode: "REPO_TO_DB", TargetPaths: args.Paths, ActorType: "project_repo_agent", ActorName: args.ActorName, Transport: "mcp"})
		if err != nil {
			return nil, err
		}
		return toolJSON(res)
	case "agentops_sync_files_to_db":
		var args syncArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, err
		}
		project, err := s.lookupProject(ctx, args.projectArgs)
		if err != nil {
			return nil, err
		}
		res, err := s.Sync.ApplyRepoToDBWithRequest(ctx, project, services.SyncRequest{Mode: "REPO_TO_DB", TargetPaths: args.Paths, ActorType: "project_repo_agent", ActorName: args.ActorName, Transport: "mcp"})
		if err != nil {
			return nil, err
		}
		return toolJSON(res)
	case "agentops_submit_run_report":
		var args submitReportArgs
		if err := decodeArgs(params.Arguments, &args); err != nil {
			return nil, err
		}
		project, err := s.lookupProject(ctx, args.projectArgs)
		if err != nil {
			return nil, err
		}
		data, err := args.reportBytes()
		if err != nil {
			return nil, err
		}
		sourcePath := args.SourcePath
		if sourcePath == "" {
			sourcePath = "mcp://agentops/reports/run.report.json"
		}
		status, err := s.Importer.ImportBytes(ctx, project, sourcePath, data)
		if err != nil {
			return nil, err
		}
		return toolJSON(map[string]any{"project_id": project.ID, "status": status, "source_path": sourcePath})
	default:
		return nil, fmt.Errorf("unknown tool %q", params.Name)
	}
}

type projectArgs struct {
	ProjectID   string `json:"project_id"`
	ProjectSlug string `json:"project_slug"`
	Project     string `json:"project"`
	RepoPath    string `json:"repo_path"`
}

type syncArgs struct {
	projectArgs
	Paths     []string `json:"paths"`
	ActorName string   `json:"actor_name"`
}

type submitReportArgs struct {
	projectArgs
	SourcePath string          `json:"source_path"`
	RunReport  json.RawMessage `json:"run_report"`
}

func (a submitReportArgs) reportBytes() ([]byte, error) {
	if len(a.RunReport) == 0 || string(a.RunReport) == "null" {
		return nil, errors.New("run_report is required")
	}
	var asString string
	if err := json.Unmarshal(a.RunReport, &asString); err == nil {
		return []byte(asString), nil
	}
	return a.RunReport, nil
}

func (s Server) lookupProject(ctx context.Context, args projectArgs) (domain.Project, error) {
	ref := firstNonEmpty(args.ProjectID, args.ProjectSlug, args.Project)
	ws, err := s.Store.DefaultWorkspace(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	projects, err := s.Store.ListProjects(ctx, ws.ID)
	if err != nil {
		return domain.Project{}, err
	}
	for _, project := range projects {
		if ref != "" && (project.ID == ref || project.Slug == ref) {
			return project, nil
		}
		if args.RepoPath != "" && project.RepoPath == args.RepoPath {
			return project, nil
		}
	}
	return domain.Project{}, fmt.Errorf("project not found")
}

func decodeArgs(data json.RawMessage, out any) error {
	if len(data) == 0 || string(data) == "null" {
		data = []byte("{}")
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("invalid tool arguments: %w", err)
	}
	return nil
}

func toolJSON(value any) (map[string]any, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"content": []map[string]string{{
			"type": "text",
			"text": string(data),
		}},
	}, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
