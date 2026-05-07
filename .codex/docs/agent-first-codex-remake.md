# Agent-First Codex Remake

This document defines the target remake direction for AgentOps Workspace. It replaces the previous product-specific project filesystem with Codex-native conventions.

## Goal

AgentOps Workspace becomes a Codex-native control plane:

- PostgreSQL remains canonical for published assets, workflow templates, task runs, reviews, imports, and sync state.
- Project repository files are projections and editing surfaces for agents.
- AgentOps exposes shared sync/import services to both repo-local automation and future MCP tools.
- Generated project files use `AGENTS.md` and `.codex/**`; do not introduce `.agentops/**`, `.agents/**`, or other product-specific agent folders.

## Actors

### AgentOps Agent

Runs from this repository or its deployed environment.

- Syncs DB to project repo files.
- Syncs project repo files back to DB.
- Handles admin, batch, reconcile, and recovery workflows.
- Uses the same backend sync/import service that MCP tools will use.

### Project Repo Agent

Runs inside a project repository.

- Reads and edits files in `AGENTS.md` and `.codex/**`.
- Connects to AgentOps MCP.
- Syncs project repo files to DB through MCP.
- Never writes directly to AgentOps DB.

## Filesystem Contract

Project repositories should use:

```text
AGENTS.md
.codex/
  project.json
  sync/
    lock.json
    state.json
    backups/
  assets/
    context/
  agents/
    <agent>.toml
  skills/
    <skill-name>/
      SKILL.md
  policies/
    <policy>.md
  workflows/
    <workflow>.workflow.yaml
  prompts/
    <prompt>.md
  checklists/
    <checklist>.md
  reports/
    run-report-contract.md
    runs/
      <run_id>/
        run.report.json
```

## Sync Rules

- `DB -> files` materializes published/current DB state into Codex-native paths.
- `files -> DB` creates draft asset versions or import records only.
- Published DB versions are not overwritten by repo content.
- `.codex/sync/lock.json` records generated state and checksums; it is not canonical state.
- Conflict detection compares DB checksums, repo checksums, and lock checksums.
- Backups from overwrite actions live under `.codex/sync/backups/**`.

## MCP Surface

Initial MCP tools should wrap the shared service layer:

- `get_project_manifest`
- `preview_db_to_files`
- `sync_db_to_files`
- `preview_files_to_db`
- `sync_files_to_db`
- `create_draft_asset_version`
- `submit_run_report`
- `get_sync_status`

MCP V1 is exposed by the API process:

```text
GET  /mcp/health
POST /mcp
```

If `AGENTOPS_MCP_TOKEN` is set, callers must send either:

```text
Authorization: Bearer <token>
```

or:

```text
X-AgentOps-MCP-Token: <token>
```

Project manifests include a non-secret MCP connection hint:

```json
{
  "mcp": {
    "server_name": "agentops",
    "endpoint": "${AGENTOPS_PUBLIC_BASE_URL}/mcp",
    "auth": "bearer_token_env",
    "token_env": "AGENTOPS_MCP_TOKEN"
  }
}
```

Set `AGENTOPS_PUBLIC_BASE_URL` once to control public API URLs. Override `AGENTOPS_MCP_PUBLIC_URL` only when MCP is served from a different URL. Never materialize the token itself into project repositories.

Implemented V1 tools:

- `agentops_get_project_manifest`
- `agentops_get_sync_status`
- `agentops_preview_files_to_db`
- `agentops_sync_files_to_db`
- `agentops_submit_run_report`

## AgentOps CLI Surface

AgentOps-local agents should use the CLI instead of MCP for local maintenance:

```sh
cd api && go run ./cmd/agentops project manifest --project <slug-or-id>
cd api && go run ./cmd/agentops sync preview --project <slug-or-id> --mode files-to-db
cd api && go run ./cmd/agentops sync files-to-db --project <slug-or-id>
cd api && go run ./cmd/agentops sync db-to-files --project <slug-or-id>
cd api && go run ./cmd/agentops sync reconcile --project <slug-or-id>
cd api && go run ./cmd/agentops report import --project <slug-or-id>
```

The CLI is an adapter over the shared sync/import services; it must not duplicate business rules.

When running through Docker, use the CLI binary baked into the API image:

```sh
make cli ARGS="sync preview --project <slug-or-id> --mode files-to-db"
make cli ARGS="sync files-to-db --project <slug-or-id>"
make cli ARGS="sync db-to-files --project <slug-or-id>"
make cli ARGS="report import --project <slug-or-id>"
```

## Migration Map

- `.agentops/project.yaml` -> `.codex/project.json`
- `.agentops/assets.lock.yaml` -> `.codex/sync/lock.json`
- `.agentops/runs/**` -> `.codex/reports/runs/**`
- `docs/agentic/skills/*.md` -> `.codex/skills/<skill>/SKILL.md`
- `docs/agentic/subagents/*.md` -> `.codex/agents/<agent>.toml`
- `docs/agentic/policies/*.md` -> `.codex/policies/*.md`
- `docs/agentic/workflows/*.workflow.yaml` -> `.codex/workflows/*.workflow.yaml`
- `docs/agentic/prompts/*.md` -> `.codex/prompts/*.md`
- `docs/agentic/checklists/*.md` -> `.codex/checklists/*.md`
- `docs/agentic/contracts/run-report-contract.md` -> `.codex/reports/run-report-contract.md`

Legacy paths may be read during migration if explicitly supported, but new writes must target Codex-native paths only.
