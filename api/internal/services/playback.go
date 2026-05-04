package services

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"agentops-workspace/api/internal/domain"
	"agentops-workspace/api/internal/repo"
)

type PlaybackService struct {
	Store repo.Store
	Audit AuditService
}

type PlaybackResponse struct {
	TaskRunID     string                `json:"task_run_id"`
	ExternalRunID string                `json:"external_run_id"`
	Workflow      map[string]string     `json:"workflow"`
	ReviewStatus  string                `json:"review_status"`
	ImportSummary *domain.ImportSummary `json:"import_summary,omitempty"`
	Nodes         []map[string]any      `json:"nodes"`
	Edges         []map[string]any      `json:"edges"`
	Events        []map[string]any      `json:"events"`
	Review        map[string]any        `json:"review"`
}

func (s PlaybackService) Build(ctx context.Context, taskRun domain.TaskRun, importSummary *domain.ImportSummary) (PlaybackResponse, error) {
	var def WorkflowDefinition
	if taskRun.ExpectedWorkflowID != "" {
		if _, d, _, err := s.Audit.FindWorkflow(ctx, s.projectWorkspaceID(ctx, taskRun.ProjectID), taskRun.ExpectedWorkflowID, taskRun.ExpectedWorkflowVersion); err == nil {
			def = d
		}
	}
	actual, err := s.loadSteps(ctx, taskRun.ID)
	if err != nil {
		return PlaybackResponse{}, err
	}
	events, err := s.loadEvents(ctx, taskRun.ID)
	if err != nil {
		return PlaybackResponse{}, err
	}
	review, _ := s.loadReview(ctx, taskRun.ID)
	nodes, edges := mergeGraph(def, actual)
	return PlaybackResponse{
		TaskRunID: taskRun.ID, ExternalRunID: taskRun.ExternalRunID,
		Workflow:     map[string]string{"id": taskRun.ExpectedWorkflowID, "version": taskRun.ExpectedWorkflowVersion},
		ReviewStatus: taskRun.ReviewStatus, ImportSummary: importSummary,
		Nodes: nodes, Edges: edges, Events: events, Review: review,
	}, nil
}

func (s PlaybackService) projectWorkspaceID(ctx context.Context, projectID string) string {
	var workspaceID string
	_ = s.Store.DB.QueryRow(ctx, `SELECT workspace_id FROM projects WHERE id=$1`, projectID).Scan(&workspaceID)
	return workspaceID
}

func (s PlaybackService) loadSteps(ctx context.Context, taskRunID string) (map[string]map[string]any, error) {
	rows, err := s.Store.DB.Query(ctx, `SELECT node_key,node_type,title,expected_ref,actual_ref,asset_version,executor,status,purpose,input_summary,action_summary,output_summary,started_at,finished_at FROM task_run_steps WHERE task_run_id=$1 ORDER BY created_at`, taskRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]map[string]any{}
	for rows.Next() {
		var nodeKey, nodeType, title, status, purpose, inputSummary string
		var expectedRef, actualRef, version, executor *string
		var action, output json.RawMessage
		var started, finished *time.Time
		if err := rows.Scan(&nodeKey, &nodeType, &title, &expectedRef, &actualRef, &version, &executor, &status, &purpose, &inputSummary, &action, &output, &started, &finished); err != nil {
			return nil, err
		}
		out[nodeKey] = map[string]any{
			"id": nodeKey, "label": title, "type": nodeType, "ref": strPtr(expectedRef), "actual_ref": strPtr(actualRef),
			"version": strPtr(version), "executor": strPtr(executor), "status": status, "purpose": purpose, "input_summary": inputSummary,
			"action_summary": rawJSON(action), "output_summary": rawJSON(output), "started_at": started, "finished_at": finished,
		}
	}
	return out, rows.Err()
}

func (s PlaybackService) loadEvents(ctx context.Context, taskRunID string) ([]map[string]any, error) {
	rows, err := s.Store.DB.Query(ctx, `SELECT node_key,event_type,event_time,summary,metadata FROM task_run_node_events WHERE task_run_id=$1 ORDER BY event_time, created_at`, taskRunID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []map[string]any
	for rows.Next() {
		var nodeKey, eventType, summary string
		var eventTime time.Time
		var metadata json.RawMessage
		if err := rows.Scan(&nodeKey, &eventType, &eventTime, &summary, &metadata); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{"node_key": nodeKey, "event_type": eventType, "event_time": eventTime, "summary": summary, "metadata": rawJSON(metadata)})
	}
	return out, rows.Err()
}

func (s PlaybackService) loadReview(ctx context.Context, taskRunID string) (map[string]any, error) {
	var status, summary string
	var violations, warnings json.RawMessage
	err := s.Store.DB.QueryRow(ctx, `SELECT review_status,summary,violations_json,warnings_json FROM task_run_reviews WHERE task_run_id=$1 ORDER BY created_at DESC LIMIT 1`, taskRunID).Scan(&status, &summary, &violations, &warnings)
	if err != nil {
		return map[string]any{"summary": "No review available.", "violations": []any{}, "warnings": []any{}}, err
	}
	return map[string]any{"status": status, "summary": summary, "violations": rawJSON(violations), "warnings": rawJSON(warnings)}, nil
}

func mergeGraph(def WorkflowDefinition, actual map[string]map[string]any) ([]map[string]any, []map[string]any) {
	var nodes []map[string]any
	var edges []map[string]any
	seen := map[string]bool{}
	for idx, expected := range def.Nodes {
		status := "pending"
		short := ""
		actualNode, executed := actual[expected.ID]
		if executed {
			status = actualNode["status"].(string)
			short = firstStringSlice(actualNode["output_summary"])
		} else if expected.Required {
			status = "skipped"
		}
		node := map[string]any{
			"id": expected.ID, "label": fallback(expected.Label, expected.ID), "type": expected.Type, "ref": expected.Ref,
			"status": status, "required": expected.Required, "purpose": expected.Purpose, "short_summary": short,
			"position":   position(idx),
			"compliance": map[string]any{"required": expected.Required, "executed": executed, "version_match": true, "dependency_order_ok": true, "status": complianceStatus(expected.Required, executed, status)},
		}
		for k, v := range actualNode {
			node[k] = v
		}
		nodes = append(nodes, node)
		seen[expected.ID] = true
		for _, dep := range expected.DependsOn {
			edges = append(edges, map[string]any{"id": "e_" + dep + "_" + expected.ID, "source": dep, "target": expected.ID, "status": status})
		}
	}
	for key, actualNode := range actual {
		if seen[key] {
			continue
		}
		actualNode["required"] = false
		actualNode["unexpected"] = true
		actualNode["position"] = position(len(nodes))
		actualNode["compliance"] = map[string]any{"required": false, "executed": true, "status": "warning", "unexpected": true}
		nodes = append(nodes, actualNode)
	}
	if len(edges) == 0 && len(nodes) > 1 {
		for i := 1; i < len(nodes); i++ {
			edges = append(edges, map[string]any{"id": "e_auto_" + nodes[i-1]["id"].(string) + "_" + nodes[i]["id"].(string), "source": nodes[i-1]["id"], "target": nodes[i]["id"], "status": nodes[i]["status"]})
		}
	}
	return nodes, edges
}

func position(i int) map[string]float64 {
	col := i % 3
	row := int(math.Floor(float64(i / 3)))
	return map[string]float64{"x": float64(col * 340), "y": float64(row * 190)}
}

func complianceStatus(required, executed bool, status string) string {
	if required && (!executed || status == "failed" || status == "skipped" || status == "blocked") {
		return "failed"
	}
	if !executed {
		return "warning"
	}
	return "pass"
}

func fallback(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func strPtr(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func rawJSON(raw json.RawMessage) any {
	var out any
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	return out
}

func firstStringSlice(v any) string {
	arr, ok := v.([]any)
	if ok && len(arr) > 0 {
		if s, ok := arr[0].(string); ok {
			return s
		}
	}
	return ""
}
