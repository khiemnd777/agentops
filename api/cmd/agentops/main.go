package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"agentops-workspace/api/internal/config"
	"agentops-workspace/api/internal/db"
	"agentops-workspace/api/internal/domain"
	"agentops-workspace/api/internal/repo"
	"agentops-workspace/api/internal/services"
)

func main() {
	log.SetFlags(0)
	if err := run(context.Background(), os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}
	runtime, err := newRuntime(ctx)
	if err != nil {
		return err
	}
	defer runtime.pool.Close()

	switch args[0] {
	case "project":
		return runtime.project(ctx, args[1:])
	case "sync":
		return runtime.syncCommand(ctx, args[1:])
	case "report":
		return runtime.report(ctx, args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

type cliRuntime struct {
	store    repo.Store
	guard    services.PathGuard
	syncSvc  services.SyncService
	importer *services.RunImporter
	pool     interface{ Close() }
}

func newRuntime(ctx context.Context) (*cliRuntime, error) {
	cfg := config.Load()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}
	migrationsDir := "migrations"
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = filepath.Join("api", "migrations")
	}
	if _, err := os.Stat(migrationsDir); err != nil {
		migrationsDir = "/app/migrations"
	}
	if err := db.Migrate(ctx, pool, migrationsDir); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	store := repo.New(pool)
	if err := (services.Seeder{Store: store}).Seed(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("seed workspace: %w", err)
	}
	guard := services.NewPathGuard()
	return &cliRuntime{
		store:    store,
		guard:    guard,
		syncSvc:  services.SyncService{Store: store, Guard: guard, MCPPublicURL: cfg.MCPPublicURL},
		importer: services.NewRunImporter(store, cfg.RunImport.MaxReportsPerScan),
		pool:     pool,
	}, nil
}

func (r *cliRuntime) project(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("project subcommand is required")
	}
	switch args[0] {
	case "manifest":
		fs := flag.NewFlagSet("agentops project manifest", flag.ContinueOnError)
		projectRef := fs.String("project", "", "project id or slug")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		project, err := r.lookupProject(ctx, *projectRef)
		if err != nil {
			return err
		}
		var manifest map[string]any
		if err := json.Unmarshal([]byte(r.syncSvc.ProjectManifest(project)), &manifest); err != nil {
			return err
		}
		return printJSON(manifest)
	default:
		return fmt.Errorf("unknown project subcommand %q", args[0])
	}
}

func (r *cliRuntime) syncCommand(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("sync subcommand is required")
	}
	fs := flag.NewFlagSet("agentops sync "+args[0], flag.ContinueOnError)
	projectRef := fs.String("project", "", "project id or slug")
	mode := fs.String("mode", "", "db-to-files, files-to-db, or compare-only")
	paths := fs.String("paths", "", "comma-separated target paths")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	project, err := r.lookupProject(ctx, *projectRef)
	if err != nil {
		return err
	}
	req := services.SyncRequest{Mode: normalizedMode(*mode), TargetPaths: splitCSV(*paths), ActorType: "agentops_local_agent", ActorName: "agentops-cli", Transport: "cli"}
	switch args[0] {
	case "preview":
		if req.Mode == "" {
			req.Mode = "REPO_TO_DB"
		}
		res, err := r.syncSvc.PreviewWithRequest(ctx, project, req)
		if err != nil {
			return err
		}
		return printJSON(res)
	case "files-to-db":
		req.Mode = "REPO_TO_DB"
		res, err := r.syncSvc.ApplyRepoToDBWithRequest(ctx, project, req)
		if err != nil {
			return err
		}
		return printJSON(res)
	case "db-to-files":
		req.Mode = "DB_TO_REPO"
		res, err := r.syncSvc.ApplyDBToRepoWithRequest(ctx, project, req)
		if err != nil {
			return err
		}
		return printJSON(res)
	case "reconcile":
		req.Mode = "COMPARE_ONLY"
		res, err := r.syncSvc.PreviewWithRequest(ctx, project, req)
		if err != nil {
			return err
		}
		return printJSON(res)
	default:
		return fmt.Errorf("unknown sync subcommand %q", args[0])
	}
}

func (r *cliRuntime) report(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("report subcommand is required")
	}
	switch args[0] {
	case "import":
		fs := flag.NewFlagSet("agentops report import", flag.ContinueOnError)
		projectRef := fs.String("project", "", "project id or slug")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		project, err := r.lookupProject(ctx, *projectRef)
		if err != nil {
			return err
		}
		summary, err := r.importer.AutoImport(ctx, project)
		if err != nil {
			return err
		}
		return printJSON(summary)
	default:
		return fmt.Errorf("unknown report subcommand %q", args[0])
	}
}

func (r *cliRuntime) lookupProject(ctx context.Context, ref string) (domain.Project, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return domain.Project{}, fmt.Errorf("--project is required")
	}
	ws, err := r.store.DefaultWorkspace(ctx)
	if err != nil {
		return domain.Project{}, err
	}
	projects, err := r.store.ListProjects(ctx, ws.ID)
	if err != nil {
		return domain.Project{}, err
	}
	for _, project := range projects {
		if project.ID == ref || project.Slug == ref {
			return project, nil
		}
	}
	return domain.Project{}, fmt.Errorf("project %q not found", ref)
}

func normalizedMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "files-to-db", "repo-to-db":
		return "REPO_TO_DB"
	case "db-to-files", "db-to-repo":
		return "DB_TO_REPO"
	case "compare", "compare-only", "reconcile":
		return "COMPARE_ONLY"
	default:
		return strings.ToUpper(value)
	}
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func printJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage:
  agentops project manifest --project <id-or-slug>
  agentops sync preview --project <id-or-slug> --mode files-to-db|db-to-files|compare-only [--paths a,b]
  agentops sync files-to-db --project <id-or-slug> [--paths a,b]
  agentops sync db-to-files --project <id-or-slug> [--paths a,b]
  agentops sync reconcile --project <id-or-slug>
  agentops report import --project <id-or-slug>`)
}
