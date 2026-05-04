package services

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	agentdb "agentops-workspace/api/internal/db"
	"agentops-workspace/api/internal/domain"
	"agentops-workspace/api/internal/repo"
)

func TestRunImporterRollsBackPartialTaskRunOnFailure(t *testing.T) {
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
	store := repo.New(pool)
	if err := (Seeder{Store: store}).Seed(ctx); err != nil {
		t.Fatal(err)
	}
	ws, err := store.DefaultWorkspace(ctx)
	if err != nil {
		t.Fatal(err)
	}
	repoPath := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(filepath.Join(repoPath, ".agentops", "runs", "run_tx_failure"), 0755); err != nil {
		t.Fatal(err)
	}
	slug := "tx-failure-" + time.Now().UTC().Format("150405000")
	project := domain.Project{WorkspaceID: ws.ID, Name: "Tx Failure", Slug: slug, RepoPath: repoPath, DefaultBranch: "main"}
	project, err = store.CreateProject(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	report := `{
  "schema_version":"1.0",
  "run_id":"run_tx_failure",
  "project_slug":"` + slug + `",
  "task":{"title":"Tx failure","input_summary":"Force rollback."},
  "workflow":{"expected_id":"fullstack-feature-workflow","expected_version":"1.0.0","actual_id":"fullstack-feature-workflow","actual_version":"1.0.0"},
  "status":"completed",
  "change_summary":["changed"],
  "final_summary":"done",
  "nodes":[{"node_key":"task_start","title":"Task Start","type":"task_start","status":"completed"}],
  "events":[],
  "assets_used":[]
}`
	reportPath := filepath.Join(repoPath, ".agentops", "runs", "run_tx_failure", "run.report.json")
	if err := os.WriteFile(reportPath, []byte(report), 0644); err != nil {
		t.Fatal(err)
	}
	importer := NewRunImporter(store, 100)
	importer.failAfterTaskRunForTest = true
	if _, err := importer.ImportFile(ctx, project, reportPath); !errors.Is(err, errForcedImporterFailure) {
		t.Fatalf("expected forced failure, got %v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM task_runs WHERE project_id=$1 AND external_run_id='run_tx_failure'`, project.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("expected rollback to remove partial task_run, found %d", count)
	}
}
