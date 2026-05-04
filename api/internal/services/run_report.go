package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

type RunReport struct {
	SchemaVersion string                 `json:"schema_version"`
	RunID         string                 `json:"run_id"`
	ProjectSlug   string                 `json:"project_slug"`
	Task          ReportTask             `json:"task"`
	Workflow      ReportWorkflow         `json:"workflow"`
	Status        string                 `json:"status"`
	StartedAt     string                 `json:"started_at"`
	FinishedAt    string                 `json:"finished_at"`
	ChangeSummary []string               `json:"change_summary"`
	FinalSummary  string                 `json:"final_summary"`
	Nodes         []ReportNode           `json:"nodes"`
	Events        []ReportEvent          `json:"events"`
	AssetsUsed    []ReportAsset          `json:"assets_used"`
	WorkerSummary []map[string]any       `json:"worker_summary"`
	ReviewHints   map[string]any         `json:"review_hints"`
	Raw           map[string]interface{} `json:"-"`
}

type ReportTask struct {
	Title        string `json:"title"`
	InputSummary string `json:"input_summary"`
}

type ReportWorkflow struct {
	ExpectedID      string `json:"expected_id"`
	ExpectedVersion string `json:"expected_version"`
	ActualID        string `json:"actual_id"`
	ActualVersion   string `json:"actual_version"`
}

type ReportNode struct {
	NodeKey       string   `json:"node_key"`
	Title         string   `json:"title"`
	Type          string   `json:"type"`
	Ref           string   `json:"ref"`
	Version       string   `json:"version"`
	Executor      string   `json:"executor"`
	Status        string   `json:"status"`
	Purpose       string   `json:"purpose"`
	InputSummary  string   `json:"input_summary"`
	ActionSummary []string `json:"action_summary"`
	OutputSummary []string `json:"output_summary"`
	StartedAt     string   `json:"started_at"`
	FinishedAt    string   `json:"finished_at"`
}

type ReportEvent struct {
	NodeKey   string         `json:"node_key"`
	EventType string         `json:"event_type"`
	EventTime string         `json:"event_time"`
	Summary   string         `json:"summary"`
	Metadata  map[string]any `json:"metadata"`
}

type ReportAsset struct {
	Type      string `json:"type"`
	Slug      string `json:"slug"`
	Version   string `json:"version"`
	UsageRole string `json:"usage_role"`
}

type ValidationError struct {
	Status string   `json:"status"`
	Errors []string `json:"errors"`
}

func (e ValidationError) Error() string {
	return strings.Join(e.Errors, "; ")
}

type RunReportValidator struct{}

var (
	validEvents = []string{"entered", "started", "progress", "completed", "failed", "skipped", "blocked", "warning"}
	validStatus = []string{"pending", "running", "completed", "failed", "skipped", "blocked", "needs_review"}
	secretRegex = regexp.MustCompile(`(?i)(-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----|api[_-]?key\s*[:=]\s*['"]?[A-Za-z0-9_\-]{16,}|access[_-]?token\s*[:=]|refresh[_-]?token\s*[:=]|password\s*[:=]|sk_live_[A-Za-z0-9]{16,}|eyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,})`)
)

func (RunReportValidator) ParseAndValidate(data []byte, expectedProjectSlug string) (RunReport, error) {
	if secretRegex.Match(data) {
		return RunReport{}, ValidationError{Status: "unsafe_secret_found", Errors: []string{"report contains secret-like content"}}
	}
	var report RunReport
	if err := json.Unmarshal(data, &report); err != nil {
		return RunReport{}, ValidationError{Status: "invalid_schema", Errors: []string{err.Error()}}
	}
	var raw map[string]any
	_ = json.Unmarshal(data, &raw)
	report.Raw = raw
	var errs []string
	if report.SchemaVersion == "" {
		errs = append(errs, "schema_version is required")
	}
	if report.RunID == "" {
		errs = append(errs, "run_id is required")
	}
	if report.ProjectSlug != expectedProjectSlug {
		errs = append(errs, "project_slug does not match project")
	}
	if report.Workflow.ExpectedID == "" || report.Workflow.ExpectedVersion == "" {
		errs = append(errs, "workflow expected_id/version is required")
	}
	if len(report.Nodes) == 0 {
		errs = append(errs, "nodes array is required")
	}
	if report.StartedAt != "" {
		validateTime(report.StartedAt, "started_at", &errs)
	}
	if report.FinishedAt != "" {
		validateTime(report.FinishedAt, "finished_at", &errs)
	}
	for i, n := range report.Nodes {
		prefix := fmt.Sprintf("nodes[%d]", i)
		if n.NodeKey == "" || n.Title == "" || n.Type == "" || n.Status == "" {
			errs = append(errs, prefix+" requires node_key, title, type, status")
		}
		if !slices.Contains(validStatus, n.Status) {
			errs = append(errs, prefix+" has invalid status")
		}
		if n.StartedAt != "" {
			validateTime(n.StartedAt, prefix+".started_at", &errs)
		}
		if n.FinishedAt != "" {
			validateTime(n.FinishedAt, prefix+".finished_at", &errs)
		}
	}
	for i, e := range report.Events {
		prefix := fmt.Sprintf("events[%d]", i)
		if e.NodeKey == "" || e.EventType == "" || e.EventTime == "" {
			errs = append(errs, prefix+" requires node_key, event_type, event_time")
		}
		if !slices.Contains(validEvents, e.EventType) {
			errs = append(errs, prefix+" has invalid event_type")
		}
		validateTime(e.EventTime, prefix+".event_time", &errs)
	}
	for i, a := range report.AssetsUsed {
		if a.Type == "" || a.Slug == "" || a.UsageRole == "" {
			errs = append(errs, fmt.Sprintf("assets_used[%d] requires type, slug, usage_role", i))
		}
	}
	if len(errs) > 0 {
		return RunReport{}, ValidationError{Status: "invalid_schema", Errors: errs}
	}
	return report, nil
}

func validateTime(value, field string, errs *[]string) {
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		*errs = append(*errs, field+" must be RFC3339")
	}
}

func parseTimePtr(value string) *time.Time {
	if value == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &t
}

func validationStatus(err error) string {
	var ve ValidationError
	if errors.As(err, &ve) {
		return ve.Status
	}
	return "failed"
}
