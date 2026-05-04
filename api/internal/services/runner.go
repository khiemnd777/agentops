package services

import "context"

// AgentRunner is the future integration boundary for Codex CLI, Claude Code,
// Gemini CLI, and custom shell adapters. MVP intentionally uses file-based
// reports only and never executes coding agents directly.
type AgentRunner interface {
	StartRun(ctx context.Context, request RunnerRunRequest) (RunnerRun, error)
	SubmitEvent(ctx context.Context, runID string, event RunnerEvent) error
	SubmitNodeUpdate(ctx context.Context, runID string, update RunnerNodeUpdate) error
	FinishRun(ctx context.Context, runID string, result RunnerResult) error
	SubmitReport(ctx context.Context, runID string, report RunReport) error
}

type RunnerRunRequest struct {
	ProjectID  string
	WorkflowID string
	Task       string
}

type RunnerRun struct {
	ID string
}

type RunnerEvent struct {
	NodeKey string
	Type    string
	Summary string
}

type RunnerNodeUpdate struct {
	NodeKey string
	Status  string
	Summary string
}

type RunnerResult struct {
	Status       string
	FinalSummary string
}
