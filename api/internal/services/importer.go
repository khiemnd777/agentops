package services

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agentops-workspace/api/internal/domain"
	"agentops-workspace/api/internal/repo"
)

var errForcedImporterFailure = errors.New("forced importer failure")

type RunImporter struct {
	Store      repo.Store
	Validator  RunReportValidator
	Audit      AuditService
	MaxReports int

	mu    sync.Mutex
	locks map[string]bool

	failAfterTaskRunForTest bool
}

func NewRunImporter(store repo.Store, maxReports int) *RunImporter {
	return &RunImporter{Store: store, Validator: RunReportValidator{}, Audit: AuditService{Store: store}, MaxReports: maxReports, locks: map[string]bool{}}
}

func (i *RunImporter) AutoImport(ctx context.Context, project domain.Project) (domain.ImportSummary, error) {
	if !i.tryLock(project.ID) {
		return domain.ImportSummary{Locked: true}, nil
	}
	defer i.unlock(project.ID)
	pattern := filepath.Join(project.RepoPath, ".agentops", "runs", "*", "run.report.json")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return domain.ImportSummary{}, err
	}
	if i.MaxReports > 0 && len(files) > i.MaxReports {
		files = files[:i.MaxReports]
	}
	summary := domain.ImportSummary{Scanned: len(files)}
	for _, file := range files {
		status, err := i.ImportFile(ctx, project, file)
		if err != nil {
			summary.Invalid++
			continue
		}
		switch status {
		case "imported":
			summary.Imported++
		case "updated":
			summary.Updated++
		case "duplicate":
			summary.Unchanged++
		default:
			summary.Invalid++
		}
	}
	_, _ = i.Store.DB.Exec(ctx, `UPDATE projects SET last_auto_import_at=now() WHERE id=$1`, project.ID)
	return summary, nil
}

func (i *RunImporter) ImportFile(ctx context.Context, project domain.Project, file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "failed", err
	}
	checksum := SHA256Bytes(data)
	report, err := i.Validator.ParseAndValidate(data, project.Slug)
	if err != nil {
		runID := report.RunID
		if runID == "" {
			runID = filepath.Base(filepath.Dir(file))
		}
		_, _ = i.Store.DB.Exec(ctx, `INSERT INTO task_run_imports(project_id,run_id,source_path,source_checksum,import_status,error_message,created_at) VALUES($1,$2,$3,$4,$5,$6,now()) ON CONFLICT(project_id,run_id,source_checksum) DO NOTHING`, project.ID, runID, file, checksum, validationStatus(err), err.Error())
		return validationStatus(err), err
	}
	var exists bool
	_ = i.Store.DB.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM task_run_imports WHERE project_id=$1 AND run_id=$2 AND source_checksum=$3 AND import_status='imported')`, project.ID, report.RunID, checksum).Scan(&exists)
	if exists {
		return "duplicate", nil
	}
	var previousChecksum string
	_ = i.Store.DB.QueryRow(ctx, `SELECT report_checksum FROM task_runs WHERE project_id=$1 AND external_run_id=$2`, project.ID, report.RunID).Scan(&previousChecksum)
	status := "imported"
	if previousChecksum != "" && previousChecksum != checksum {
		status = "updated"
	}
	tx, err := i.Store.DB.Begin(ctx)
	if err != nil {
		return "failed", err
	}
	defer tx.Rollback(ctx)
	var workflowTemplateID *string
	var workflowDef WorkflowDefinition
	if id, def, _, err := i.Audit.FindWorkflow(ctx, project.WorkspaceID, report.Workflow.ExpectedID, report.Workflow.ExpectedVersion); err == nil {
		workflowTemplateID = &id
		workflowDef = def
	}
	var taskRunID string
	changeSummary, _ := json.Marshal(report.ChangeSummary)
	started := parseTimePtr(report.StartedAt)
	finished := parseTimePtr(report.FinishedAt)
	if err := tx.QueryRow(ctx, `
		INSERT INTO task_runs(project_id,workflow_template_id,external_run_id,title,input_summary,change_summary,final_summary,status,expected_workflow_id,expected_workflow_version,actual_workflow_id,actual_workflow_version,report_checksum,report_source_path,imported_at,import_status,started_at,finished_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,now(),'imported',$15,$16)
		ON CONFLICT(project_id,external_run_id) DO UPDATE SET workflow_template_id=EXCLUDED.workflow_template_id,title=EXCLUDED.title,input_summary=EXCLUDED.input_summary,change_summary=EXCLUDED.change_summary,final_summary=EXCLUDED.final_summary,status=EXCLUDED.status,expected_workflow_id=EXCLUDED.expected_workflow_id,expected_workflow_version=EXCLUDED.expected_workflow_version,actual_workflow_id=EXCLUDED.actual_workflow_id,actual_workflow_version=EXCLUDED.actual_workflow_version,report_checksum=EXCLUDED.report_checksum,report_source_path=EXCLUDED.report_source_path,imported_at=now(),import_status='imported',started_at=EXCLUDED.started_at,finished_at=EXCLUDED.finished_at,updated_at=now()
		RETURNING id`,
		project.ID, workflowTemplateID, report.RunID, report.Task.Title, report.Task.InputSummary, changeSummary, report.FinalSummary, report.Status, report.Workflow.ExpectedID, report.Workflow.ExpectedVersion, report.Workflow.ActualID, report.Workflow.ActualVersion, checksum, file, started, finished).Scan(&taskRunID); err != nil {
		return "failed", err
	}
	if i.failAfterTaskRunForTest {
		return "failed", errForcedImporterFailure
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_run_steps WHERE task_run_id=$1`, taskRunID); err != nil {
		return "failed", err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_run_node_events WHERE task_run_id=$1`, taskRunID); err != nil {
		return "failed", err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM task_run_assets WHERE task_run_id=$1`, taskRunID); err != nil {
		return "failed", err
	}
	for _, node := range report.Nodes {
		actionSummary, _ := json.Marshal(node.ActionSummary)
		outputSummary, _ := json.Marshal(node.OutputSummary)
		if _, err := tx.Exec(ctx, `INSERT INTO task_run_steps(task_run_id,node_key,node_type,title,expected_ref,actual_ref,asset_version,executor,status,purpose,input_summary,action_summary,output_summary,started_at,finished_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			taskRunID, node.NodeKey, node.Type, node.Title, node.Ref, node.Ref, nullEmpty(node.Version), nullEmpty(node.Executor), node.Status, node.Purpose, node.InputSummary, actionSummary, outputSummary, parseTimePtr(node.StartedAt), parseTimePtr(node.FinishedAt)); err != nil {
			return "failed", err
		}
	}
	for _, event := range report.Events {
		metadata, _ := json.Marshal(event.Metadata)
		if event.Metadata == nil {
			metadata = []byte("{}")
		}
		if _, err := tx.Exec(ctx, `INSERT INTO task_run_node_events(task_run_id,node_key,event_type,event_time,summary,metadata) VALUES($1,$2,$3,$4,$5,$6)`, taskRunID, event.NodeKey, event.EventType, parseTimePtr(event.EventTime), event.Summary, metadata); err != nil {
			return "failed", err
		}
	}
	for _, asset := range report.AssetsUsed {
		if _, err := tx.Exec(ctx, `INSERT INTO task_run_assets(task_run_id,asset_type,asset_slug,asset_version,usage_role) VALUES($1,$2,$3,$4,$5)`, taskRunID, asset.Type, asset.Slug, nullEmpty(asset.Version), asset.UsageRole); err != nil {
			return "failed", err
		}
	}
	var revision int
	_ = tx.QueryRow(ctx, `SELECT COALESCE(MAX(import_revision),0)+1 FROM task_run_imports WHERE project_id=$1 AND run_id=$2`, project.ID, report.RunID).Scan(&revision)
	if _, err := tx.Exec(ctx, `INSERT INTO task_run_imports(project_id,task_run_id,run_id,source_path,source_checksum,import_revision,import_status,imported_at) VALUES($1,$2,$3,$4,$5,$6,'imported',now()) ON CONFLICT(project_id,run_id,source_checksum) DO NOTHING`, project.ID, taskRunID, report.RunID, file, checksum, revision); err != nil {
		return "failed", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "failed", err
	}
	if workflowDef.ID != "" {
		result := i.Audit.Audit(report, workflowDef)
		if lock, err := LoadAssetsLock(project.RepoPath); err == nil {
			result = i.Audit.AuditWithLock(report, workflowDef, lock)
		}
		_ = i.Audit.StoreReview(ctx, taskRunID, result, workflowDef, report.Raw)
	}
	return status, nil
}

func (i *RunImporter) tryLock(projectID string) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.locks[projectID] {
		return false
	}
	i.locks[projectID] = true
	return true
}

func (i *RunImporter) unlock(projectID string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	delete(i.locks, projectID)
}

func nullEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

var _ = time.RFC3339
