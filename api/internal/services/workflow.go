package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"agentops-workspace/api/internal/repo"

	"gopkg.in/yaml.v3"
)

type WorkflowDefinition struct {
	ID         string                 `yaml:"id" json:"id"`
	Name       string                 `yaml:"name" json:"name"`
	Version    string                 `yaml:"version" json:"version"`
	Nodes      []WorkflowNode         `yaml:"nodes" json:"nodes"`
	Compliance map[string]interface{} `yaml:"compliance" json:"compliance"`
}

type WorkflowNode struct {
	ID        string   `yaml:"id" json:"id"`
	Type      string   `yaml:"type" json:"type"`
	Ref       string   `yaml:"ref" json:"ref"`
	Label     string   `yaml:"label" json:"label"`
	Purpose   string   `yaml:"purpose" json:"purpose"`
	Required  bool     `yaml:"required" json:"required"`
	DependsOn []string `yaml:"depends_on" json:"depends_on"`
	Executor  string   `yaml:"executor" json:"executor"`
	Worker    string   `yaml:"worker" json:"worker"`
}

type AuditResult struct {
	Status     string           `json:"status"`
	Summary    string           `json:"summary"`
	Violations []map[string]any `json:"violations"`
	Warnings   []map[string]any `json:"warnings"`
}

type AuditService struct {
	Store repo.Store
}

type AssetsLock struct {
	Version int `yaml:"version" json:"version"`
	Project struct {
		ID   string `yaml:"id" json:"id"`
		Slug string `yaml:"slug" json:"slug"`
	} `yaml:"project" json:"project"`
	Assets []LockedAsset `yaml:"assets" json:"assets"`
}

type LockedAsset struct {
	AssetID    string `yaml:"asset_id" json:"asset_id"`
	Type       string `yaml:"type" json:"type"`
	Slug       string `yaml:"slug" json:"slug"`
	Version    string `yaml:"version" json:"version"`
	TargetPath string `yaml:"target_path" json:"target_path"`
	Checksum   string `yaml:"checksum" json:"checksum"`
	Generated  bool   `yaml:"generated" json:"generated"`
}

func (s AuditService) FindWorkflow(ctx context.Context, workspaceID, slug, version string) (string, WorkflowDefinition, string, error) {
	var id, yamlText string
	err := s.Store.DB.QueryRow(ctx, `SELECT id, definition_yaml FROM workflow_templates WHERE workspace_id=$1 AND slug=$2 AND version=$3`, workspaceID, slug, version).Scan(&id, &yamlText)
	if err != nil {
		return "", WorkflowDefinition{}, "", err
	}
	def, err := ParseWorkflowDefinition(yamlText)
	if err != nil {
		return "", WorkflowDefinition{}, "", err
	}
	return id, def, yamlText, nil
}

func (s AuditService) Audit(report RunReport, def WorkflowDefinition) AuditResult {
	actualByKey := map[string]ReportNode{}
	order := map[string]int{}
	for i, node := range report.Nodes {
		actualByKey[node.NodeKey] = node
		order[node.NodeKey] = i
	}
	assetVersions := map[string]string{}
	for _, asset := range report.AssetsUsed {
		assetVersions[asset.Slug] = asset.Version
	}
	expectedKeys := map[string]bool{}
	var violations, warnings []map[string]any
	hasReporting := false
	for _, node := range def.Nodes {
		expectedKeys[node.ID] = true
		if node.Type == "reporting" {
			hasReporting = true
		}
		actual, ok := actualByKey[node.ID]
		if !ok {
			if node.Required {
				violations = append(violations, violation("missing_required_step", node.ID, node.Ref, "high", "Required workflow step was not executed."))
			} else {
				warnings = append(warnings, warning("missing_optional_step", node.ID, "Optional workflow step was not executed."))
			}
			continue
		}
		if node.Required && slices.Contains([]string{"failed", "skipped", "blocked"}, actual.Status) {
			violations = append(violations, violation("required_step_not_completed", node.ID, node.Ref, "high", "Required workflow step did not complete."))
		}
		if node.Required && len(actual.OutputSummary) == 0 {
			warnings = append(warnings, warning("missing_required_output_summary", node.ID, "Required workflow step has no output summary."))
		}
		if node.Required && node.Ref != "" && node.Type != "task_start" {
			if _, ok := assetVersions[node.Ref]; !ok {
				warnings = append(warnings, map[string]any{"type": "required_asset_not_reported", "node_key": node.ID, "expected_ref": node.Ref, "severity": "medium", "message": "Required workflow asset was not listed in assets_used."})
			}
		}
		if node.Required && node.Worker != "" && actual.Executor != node.Worker {
			violations = append(violations, violation("required_worker_mismatch", node.ID, node.Worker, "medium", "Required worker/subagent was not used for this step."))
		}
		for _, dep := range node.DependsOn {
			if depOrder, depOK := order[dep]; depOK && depOrder > order[node.ID] {
				violations = append(violations, violation("dependency_order_violation", node.ID, node.Ref, "medium", fmt.Sprintf("Step ran before dependency %s.", dep)))
			}
		}
	}
	for _, actual := range report.Nodes {
		if !expectedKeys[actual.NodeKey] {
			warnings = append(warnings, map[string]any{"type": "unexpected_node", "node_key": actual.NodeKey, "severity": "medium", "message": "Unexpected node appeared in actual report."})
		}
	}
	if report.FinalSummary == "" {
		violations = append(violations, violation("missing_final_summary", "agentops_report", "run-report-contract", "medium", "Report is missing final_summary."))
	}
	if len(report.ChangeSummary) == 0 {
		violations = append(violations, violation("missing_change_summary", "agentops_report", "run-report-contract", "medium", "Report is missing change_summary."))
	}
	if hasReporting {
		if actual, ok := actualByKey["agentops_report"]; !ok || actual.Status != "completed" {
			violations = append(violations, violation("required_reporting_node_missing", "agentops_report", "run-report-contract", "high", "Required reporting node was not completed."))
		}
	}
	status := "pass"
	summary := "All required workflow nodes were executed in correct dependency order."
	if len(violations) > 0 {
		status = "failed"
		summary = "Workflow audit found required compliance violations."
	} else if len(warnings) > 0 {
		status = "warning"
		summary = "Workflow audit passed with warnings."
	}
	return AuditResult{Status: status, Summary: summary, Violations: violations, Warnings: warnings}
}

func (s AuditService) AuditWithLock(report RunReport, def WorkflowDefinition, lock AssetsLock) AuditResult {
	result := s.Audit(report, def)
	lockBySlug := map[string]LockedAsset{}
	for _, asset := range lock.Assets {
		lockBySlug[asset.Slug] = asset
	}
	for _, used := range report.AssetsUsed {
		locked, ok := lockBySlug[used.Slug]
		if !ok {
			result.Warnings = append(result.Warnings, map[string]any{"type": "asset_not_in_lock", "asset_slug": used.Slug, "severity": "medium", "message": "Report used an asset not present in .codex/sync/lock.json."})
			continue
		}
		if used.Version != "" && locked.Version != "" && used.Version != locked.Version {
			result.Violations = append(result.Violations, map[string]any{"type": "asset_version_mismatch", "asset_slug": used.Slug, "expected_version": locked.Version, "actual_version": used.Version, "severity": "high", "message": "Reported asset version does not match the project lock file."})
		}
	}
	if len(result.Violations) > 0 {
		result.Status = "failed"
		result.Summary = "Workflow audit found required compliance violations."
	} else if len(result.Warnings) > 0 && result.Status == "pass" {
		result.Status = "warning"
		result.Summary = "Workflow audit passed with warnings."
	}
	return result
}

func LoadAssetsLock(repoPath string) (AssetsLock, error) {
	var lock AssetsLock
	data, err := os.ReadFile(filepath.Join(repoPath, ".codex", "sync", "lock.json"))
	if err != nil {
		return lock, err
	}
	if err := json.Unmarshal(data, &lock); err != nil {
		return lock, err
	}
	return lock, nil
}

func ParseWorkflowDefinition(yamlText string) (WorkflowDefinition, error) {
	var def WorkflowDefinition
	if err := yaml.Unmarshal([]byte(yamlText), &def); err != nil {
		return WorkflowDefinition{}, err
	}
	if def.ID == "" || def.Version == "" || len(def.Nodes) == 0 {
		return WorkflowDefinition{}, fmt.Errorf("workflow YAML requires id, version, and nodes")
	}
	ids := map[string]bool{}
	for _, node := range def.Nodes {
		if node.ID == "" || node.Type == "" || node.Label == "" {
			return WorkflowDefinition{}, fmt.Errorf("workflow node requires id, type, and label")
		}
		if ids[node.ID] {
			return WorkflowDefinition{}, fmt.Errorf("duplicate workflow node id %q", node.ID)
		}
		ids[node.ID] = true
	}
	for _, node := range def.Nodes {
		for _, dep := range node.DependsOn {
			if !ids[dep] {
				return WorkflowDefinition{}, fmt.Errorf("node %q depends on unknown node %q", node.ID, dep)
			}
		}
	}
	return def, nil
}

func WorkflowGraphPreview(yamlText string) (map[string]any, error) {
	def, err := ParseWorkflowDefinition(yamlText)
	if err != nil {
		return nil, err
	}
	nodes := make([]map[string]any, 0, len(def.Nodes))
	edges := []map[string]any{}
	for i, node := range def.Nodes {
		nodes = append(nodes, map[string]any{
			"id": node.ID, "label": node.Label, "type": node.Type, "ref": node.Ref, "required": node.Required,
			"purpose": node.Purpose, "position": map[string]int{"x": (i % 3) * 320, "y": (i / 3) * 170},
		})
		for _, dep := range node.DependsOn {
			edges = append(edges, map[string]any{"id": "e_" + dep + "_" + node.ID, "source": dep, "target": node.ID})
		}
	}
	return map[string]any{"workflow": def, "nodes": nodes, "edges": edges}, nil
}

func violation(t, nodeKey, expectedRef, severity, message string) map[string]any {
	return map[string]any{"type": t, "node_key": nodeKey, "expected_ref": expectedRef, "severity": severity, "message": message}
}

func warning(t, nodeKey, message string) map[string]any {
	return map[string]any{"type": t, "node_key": nodeKey, "severity": "low", "message": message}
}

func jsonBytes(v any) []byte {
	b, _ := json.Marshal(v)
	if string(b) == "null" {
		return []byte("[]")
	}
	return b
}

func (s AuditService) StoreReview(ctx context.Context, taskRunID string, result AuditResult, expected, actual any) error {
	_, err := s.Store.DB.Exec(ctx, `
		INSERT INTO task_run_reviews(task_run_id,review_status,summary,expected_json,actual_json,violations_json,warnings_json)
		VALUES($1,$2,$3,$4,$5,$6,$7)`,
		taskRunID, result.Status, result.Summary, jsonBytes(expected), jsonBytes(actual), jsonBytes(result.Violations), jsonBytes(result.Warnings))
	if err != nil {
		return err
	}
	_, err = s.Store.DB.Exec(ctx, `UPDATE task_runs SET review_status=$1, updated_at=now() WHERE id=$2`, result.Status, taskRunID)
	return err
}
