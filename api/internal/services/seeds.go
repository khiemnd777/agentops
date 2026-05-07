package services

import (
	"context"

	"agentops-workspace/api/internal/repo"
)

type Seeder struct {
	Store repo.Store
}

func (s Seeder) Seed(ctx context.Context) error {
	ws, err := s.Store.EnsureDefaultWorkspace(ctx)
	if err != nil {
		return err
	}
	defaults := []struct {
		Type, Slug, Name, Description, Format, Content string
		Tags                                           []string
	}{
		{"agents_md", "default-agents", "Default AGENTS.md", "Generated AGENTS.md with AgentOps reporting requirement.", "markdown", defaultAgentsMD(), []string{"default", "reporting"}},
		{"readme", "default-readme", "Default README", "Default project README for AgentOps-managed repositories.", "markdown", "# Project\n\nThis repository is managed by AgentOps Workspace for agentic resources.\n", []string{"default"}},
		{"skill_doc", "noah-repo-architect", "Repo Architect Skill", "Analyze repo structure and affected modules.", "markdown", "# Repo Architect\n\nInspect project structure, ownership boundaries, and affected frontend/backend modules before implementation.\n", []string{"skill", "planning"}},
		{"skill_doc", "noah-api-feature-workflow", "Backend Workflow Skill", "Backend implementation workflow.", "markdown", "# Backend Workflow\n\nApply backend changes through handler, service, repository, schema, and validation layers.\n", []string{"skill", "backend"}},
		{"skill_doc", "noah-fe-module-workflow", "Frontend Workflow Skill", "Frontend implementation workflow.", "markdown", "# Frontend Workflow\n\nApply frontend changes using existing module, route, API wrapper, mapper, schema, table, and widget patterns.\n", []string{"skill", "frontend"}},
		{"policy_doc", "noah-contract-sync", "Contract Sync Policy", "Validate FE/API contracts.", "markdown", "# Contract Sync\n\nWhen data crosses the FE/API boundary, update request shapes, response shapes, mappers, schemas, and invalidation behavior coherently.\n", []string{"policy", "contract"}},
		{"policy_doc", "noah-auth-rbac-guard", "Auth/RBAC Guard Policy", "Validate auth and permission-sensitive flows.", "markdown", "# Auth/RBAC Guard\n\nPreserve authentication, authorization, and scope rules on protected routes and privileged actions.\n", []string{"policy", "auth"}},
		{"skill_doc", "noah-regression-review", "Regression Review Skill", "Review likely regressions.", "markdown", "# Regression Review\n\nCheck feature ownership, contracts, permissions, caching, jobs, registration, and missed verification before sign-off.\n", []string{"skill", "review"}},
		{"run_report_contract", "run-report-contract", "Run Report Contract", "Contract for external agent run reports.", "markdown", runReportContract(), []string{"contract", "reporting"}},
		{"workflow_doc", "fullstack-feature-workflow", "Fullstack Feature Workflow", "Syncable YAML workflow template.", "yaml", fullstackWorkflowYAML(), []string{"workflow"}},
	}
	seeded := map[string]struct {
		AssetID   string
		VersionID string
		Type      string
		Slug      string
	}{}
	for _, d := range defaults {
		asset, version, err := s.Store.UpsertAssetWithVersion(ctx, ws.ID, d.Type, d.Slug, d.Name, d.Description, "1.0.0", d.Content, d.Format, "published", SHA256String(d.Content), d.Tags)
		if err != nil {
			return err
		}
		seeded[d.Type+"/"+d.Slug] = struct {
			AssetID   string
			VersionID string
			Type      string
			Slug      string
		}{AssetID: asset.ID, VersionID: version.ID, Type: d.Type, Slug: d.Slug}
	}
	if err := s.seedAssetPresets(ctx, ws.ID, seeded); err != nil {
		return err
	}
	return s.seedWorkflow(ctx, ws.ID)
}

func (s Seeder) seedAssetPresets(ctx context.Context, workspaceID string, assets map[string]struct {
	AssetID   string
	VersionID string
	Type      string
	Slug      string
}) error {
	presets := []struct {
		Slug        string
		Name        string
		Description string
		Keys        []string
	}{
		{
			Slug:        "noah",
			Name:        "Noah",
			Description: "Default Noah agent instructions, workflow, policies, skills, README, and run-report contract.",
			Keys:        []string{"agents_md/default-agents", "readme/default-readme", "skill_doc/noah-api-feature-workflow", "skill_doc/noah-fe-module-workflow", "skill_doc/noah-regression-review", "policy_doc/noah-contract-sync", "policy_doc/noah-auth-rbac-guard", "workflow_doc/fullstack-feature-workflow", "run_report_contract/run-report-contract"},
		},
		{
			Slug:        "reporting-minimal",
			Name:        "Reporting Minimal",
			Description: "Only the top-level agent instruction file and run-report contract.",
			Keys:        []string{"agents_md/default-agents", "run_report_contract/run-report-contract"},
		},
		{
			Slug:        "agentops-full",
			Name:        "AgentOps Full",
			Description: "All default template assets with materializable target paths.",
			Keys:        []string{"agents_md/default-agents", "readme/default-readme", "skill_doc/noah-repo-architect", "skill_doc/noah-api-feature-workflow", "skill_doc/noah-fe-module-workflow", "skill_doc/noah-regression-review", "policy_doc/noah-contract-sync", "policy_doc/noah-auth-rbac-guard", "workflow_doc/fullstack-feature-workflow", "run_report_contract/run-report-contract"},
		},
	}
	for _, preset := range presets {
		var presetID string
		if err := s.Store.DB.QueryRow(ctx, `
			INSERT INTO asset_presets(workspace_id,slug,name,description,status,current_version,tags)
			VALUES($1,$2,$3,$4,'active',1,'["default"]'::jsonb)
			ON CONFLICT(workspace_id,slug) DO UPDATE SET name=EXCLUDED.name, description=EXCLUDED.description, status='active', current_version=EXCLUDED.current_version, updated_at=now()
			RETURNING id`, workspaceID, preset.Slug, preset.Name, preset.Description).Scan(&presetID); err != nil {
			return err
		}
		for idx, key := range preset.Keys {
			asset, ok := assets[key]
			if !ok {
				continue
			}
			target := TargetPathForAsset(asset.Type, asset.Slug)
			if _, err := s.Store.DB.Exec(ctx, `
				INSERT INTO asset_preset_items(preset_id,asset_id,asset_version_id,target_path,sort_order,required)
				VALUES($1,$2,$3,$4,$5,true)
				ON CONFLICT(preset_id,asset_version_id) DO UPDATE SET target_path=EXCLUDED.target_path, sort_order=EXCLUDED.sort_order, required=EXCLUDED.required`,
				presetID, asset.AssetID, asset.VersionID, target, idx+1); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s Seeder) seedWorkflow(ctx context.Context, workspaceID string) error {
	yaml := fullstackWorkflowYAML()
	_, err := s.Store.DB.Exec(ctx, `
		INSERT INTO workflow_templates(workspace_id,name,slug,version,definition_yaml,status)
		VALUES($1,'Fullstack Feature Workflow','fullstack-feature-workflow','1.0.0',$2,'published')
		ON CONFLICT(workspace_id,slug,version) DO UPDATE SET definition_yaml=EXCLUDED.definition_yaml, status='published', updated_at=now()`,
		workspaceID, yaml)
	return err
}

func defaultAgentsMD() string {
	return `# Agent Instructions

This project uses AgentOps as the control plane for Codex-native agent assets.

Use only these managed paths:

- ` + "`AGENTS.md`" + `
- ` + "`.codex/**`" + `

Do not create or use:

- ` + "`.agentops/**`" + `
- ` + "`.agents/**`" + `
- ` + "`docs/agentic/**`" + `

Follow the project-specific skills, policies, workflow templates, checklists, and reporting contract materialized under ` + "`.codex/`" + `.

## AgentOps Sync

When you change files under ` + "`.codex/**`" + ` or ` + "`AGENTS.md`" + `, sync them back to AgentOps DB by using the AgentOps MCP server.

Preferred flow:

1. Read ` + "`.codex/project.json`" + ` to identify the AgentOps project.
2. Read ` + "`.codex/project.json`" + ` field ` + "`mcp.endpoint`" + ` for the AgentOps MCP endpoint.
3. Use the token from the environment variable named by ` + "`.codex/project.json`" + ` field ` + "`mcp.token_env`" + `; never hardcode the token into repo files.
4. Inspect ` + "`.codex/sync/lock.json`" + ` before changing managed files.
5. Make file changes locally.
6. Call MCP tool ` + "`agentops_preview_files_to_db`" + `.
7. If the preview is safe, call MCP tool ` + "`agentops_sync_files_to_db`" + `.
8. Write a run report to ` + "`.codex/reports/runs/{run_id}/run.report.json`" + `.

Never overwrite published AgentOps versions directly. Repo file changes must be imported as draft versions or import records.

## AgentOps Reporting Requirement

At the end of every task, you MUST generate an AgentOps run report.

Output path:

` + "`.codex/reports/runs/{run_id}/run.report.json`" + `

The report must follow the schema defined in:

` + "`.codex/reports/run-report-contract.md`" + `

The report must include:
- task title
- workflow template id and version
- executed nodes
- skills used
- workers/subagents used
- node summaries
- node events
- change summary
- final summary
- skipped checks, warnings, or violations if any

Do not include raw secrets, private keys, credentials, or full logs.
Do not include full file diffs unless explicitly requested.
`
}

func runReportContract() string {
	return `# AgentOps Run Report Contract

External agents such as Codex must create a structured report after every task:

` + "`.codex/reports/runs/{run_id}/run.report.json`" + `

The JSON report must include summary-level structured data only. Write exactly one JSON object with these top-level fields:

- ` + "`schema_version`" + `: string, currently "1.0"
- ` + "`run_id`" + `: stable external run id, unique within the project
- ` + "`project_slug`" + `: must match ` + "`.codex/project.json`" + `
- ` + "`task`" + `: object with ` + "`title`" + ` and ` + "`input_summary`" + `
- ` + "`workflow`" + `: object with ` + "`expected_id`" + `, ` + "`expected_version`" + `, ` + "`actual_id`" + `, and ` + "`actual_version`" + `
- ` + "`status`" + `: one of completed, failed, blocked, needs_review
- ` + "`started_at`" + ` and ` + "`finished_at`" + `: RFC3339 timestamps when available
- ` + "`change_summary`" + `: array of concise user-facing changes
- ` + "`final_summary`" + `: concise final result
- ` + "`nodes`" + `: array of executed workflow nodes
- ` + "`events`" + `: array of node-level timeline events
- ` + "`assets_used`" + `: array of skills, policies, workflow docs, contracts, prompts, checklists, and subagents used
- ` + "`worker_summary`" + `: array of workers/subagents used and their result summaries
- ` + "`warnings`" + ` and ` + "`violations`" + `: arrays when applicable
- ` + "`review_hints`" + `: object with ` + "`raw_logs_available`" + `, ` + "`file_diff_available`" + `, and ` + "`requires_manual_review`" + `

Each node object must include:

- ` + "`node_key`" + `
- ` + "`title`" + `
- ` + "`type`" + `
- ` + "`ref`" + ` when tied to a skill, policy, workflow, subagent, or contract
- ` + "`version`" + ` when known
- ` + "`executor`" + ` when a worker/subagent performed the node
- ` + "`status`" + `: pending, running, completed, failed, skipped, blocked, or needs_review
- ` + "`purpose`" + `
- ` + "`input_summary`" + `
- ` + "`action_summary`" + ` array
- ` + "`output_summary`" + ` array
- ` + "`started_at`" + ` and ` + "`finished_at`" + ` when available

Each event object must include:

- ` + "`node_key`" + `
- ` + "`event_type`" + `: entered, started, progress, completed, failed, skipped, blocked, or warning
- ` + "`event_time`" + ` as RFC3339
- ` + "`summary`" + `
- ` + "`metadata`" + ` object when useful

Each ` + "`assets_used`" + ` object must include:

- ` + "`type`" + `
- ` + "`slug`" + `
- ` + "`version`" + ` when known
- ` + "`usage_role`" + `

Do not include secrets, tokens, private keys, credentials, full raw logs, or full file diffs.
`
}

func fullstackWorkflowYAML() string {
	return `id: fullstack-feature-workflow
name: Fullstack Feature Workflow
version: 1.0.0
nodes:
  - id: task_start
    type: task_start
    label: Task Start
    purpose: Mark the beginning of the task workflow.
    required: true
  - id: repo_architect
    type: skill
    ref: noah-repo-architect
    label: Repo Architect
    purpose: Analyze repo structure and determine affected modules.
    required: true
    depends_on: [task_start]
    display: { group: planning, icon: architecture }
  - id: backend_workflow
    type: skill
    ref: noah-api-feature-workflow
    label: Backend Workflow
    purpose: Apply backend feature changes.
    required: false
    depends_on: [repo_architect]
    condition: task.affects_backend == true
  - id: frontend_workflow
    type: skill
    ref: noah-fe-module-workflow
    label: Frontend Workflow
    purpose: Apply frontend feature changes.
    required: false
    depends_on: [repo_architect]
    condition: task.affects_frontend == true
  - id: contract_sync
    type: policy_check
    ref: noah-contract-sync
    label: Contract Sync
    purpose: Validate backend/frontend contract consistency.
    required: true
    depends_on: [backend_workflow, frontend_workflow]
  - id: auth_rbac_guard
    type: policy_check
    ref: noah-auth-rbac-guard
    label: Auth/RBAC Guard
    purpose: Validate authentication and authorization safety.
    required: true
    depends_on: [contract_sync]
  - id: regression_review
    type: review
    ref: noah-regression-review
    label: Regression Review
    purpose: Check for regressions and missed impact areas.
    required: true
    depends_on: [auth_rbac_guard]
  - id: agentops_report
    type: reporting
    ref: run-report-contract
    label: AgentOps Report
    purpose: Generate structured AgentOps run report for ingestion.
    required: true
    depends_on: [regression_review]
compliance:
  require_all_required_nodes: true
  allow_extra_nodes: false
  require_dependency_order: true
  require_asset_version_lock: true
output_contract:
  required: [execution_plan, change_summary, validation_summary, final_summary]
`
}
