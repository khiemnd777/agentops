package services

import "testing"

func TestWorkflowAuditMissingRequiredAndUnexpectedNode(t *testing.T) {
	def, err := ParseWorkflowDefinition(`id: wf
name: Workflow
version: 1.0.0
nodes:
  - id: task_start
    type: task_start
    label: Task Start
    required: true
  - id: regression_review
    type: review
    ref: noah-regression-review
    label: Regression Review
    required: true
    depends_on: [task_start]
  - id: agentops_report
    type: reporting
    ref: run-report-contract
    label: AgentOps Report
    required: true
    depends_on: [regression_review]
`)
	if err != nil {
		t.Fatal(err)
	}
	result := (AuditService{}).Audit(RunReport{
		FinalSummary:  "done",
		ChangeSummary: []string{"changed"},
		Nodes: []ReportNode{
			{NodeKey: "task_start", Title: "Task Start", Type: "task_start", Status: "completed", OutputSummary: []string{"ok"}},
			{NodeKey: "extra_node", Title: "Extra", Type: "skill", Status: "completed"},
		},
	}, def)
	if result.Status != "failed" {
		t.Fatalf("expected failed audit, got %s", result.Status)
	}
	if len(result.Violations) == 0 {
		t.Fatal("expected missing required violations")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("expected unexpected node warning")
	}
}

func TestWorkflowGraphPreviewRejectsUnknownDependency(t *testing.T) {
	_, err := WorkflowGraphPreview(`id: wf
name: Workflow
version: 1.0.0
nodes:
  - id: a
    type: skill
    label: A
    depends_on: [missing]
`)
	if err == nil {
		t.Fatal("expected invalid workflow dependency to fail")
	}
}
