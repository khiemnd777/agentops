# AgentOps Workspace

AgentOps Workspace is a local/self-hosted control plane for agentic documentation assets, workflow templates, file sync, imported agent run reports, workflow audit, and animated workflow playback review.

This MVP does not execute Codex or any other coding agent. PostgreSQL is the source of truth; repository files are generated projections for external agents to consume.

## Local Development

1. Copy `.env.example` to `.env` if you want to override defaults.
2. Optionally set the repo mount:
   - default host path: `./sample-repos`
   - custom host path: `AGENTOPS_REPO_ROOT=/Users/khiem/projects`
   - container-visible path: `/workspace/repos`
   - local host repo paths under `/Users` are also mounted at the same path by default, so you can enter paths like `/Users/khiemnguyen/Works/demo` directly in the UI.
   - if your repos live outside `/Users`, set `AGENTOPS_HOST_REPO_ROOT` to a shared parent path and enter repo paths under that root.
3. Start the stack:

```bash
docker compose up --build
```

Or with a custom repo root:

```bash
AGENTOPS_REPO_ROOT=/Users/khiem/projects docker compose up --build
```

4. Open:
   - Frontend: `http://localhost:${FRONTEND_HOST_PORT}`
   - Backend health: `http://localhost:${API_HOST_PORT}/health`

The API runs migrations and seeds default assets on boot.

## MVP Flow

1. Create a project explicitly with a repo path visible to the API process, for example `/Users/khiemnguyen/Works/demo` for a local macOS repo, or `/workspace/repos/my-repo` when using the sample repo mount.
2. Scan the existing project repo to detect agent-related files and folders.
3. Compose or select seeded assets.
4. Preview and apply DB-to-repo sync.
5. Place a report at `.agentops/runs/{run_id}/run.report.json`.
6. Open Task Runs, Reviews, or Playback. The backend auto-imports new reports, validates them, audits workflow compliance, and returns playback data.

## Implemented Control Plane Capabilities

- Versioned AgentOps assets with draft/published status and keyword search.
- Project composer preview and apply for materialized agentic resources.
- `DB_TO_REPO`, `REPO_TO_DB`, and `COMPARE_ONLY` sync modes.
- Repo-to-DB imports always create draft versions and never overwrite published versions.
- Drift, missing, orphaned, ignored, and conflicted sync item statuses.
- Workflow YAML create/update and graph preview.
- File-based run report auto-import on task list, review, and playback endpoints.
- Workflow audit with missing required nodes, dependency order, reporting node, unexpected node, required asset, worker, and summary checks.
- Animated React Flow playback with dagre layout, controls, timeline, compliance view, node detail, and manual review status capture.
- Future runner integration boundary via `AgentRunner`; no real agent execution is implemented in this MVP.

## Generated Files

AgentOps writes only approved generated paths:

- `AGENTS.md`
- `README.md`
- `.agentops/**`
- `docs/agentic/**`

All writes are path-checked against the project repo path, symlink containment, and the generated-path allowlist.

## Test Fixtures

Report fixtures live under `fixtures/run_reports/`:

- `fullstack-feature-success.report.json`
- `fullstack-feature-warning.report.json`
- `invalid-schema.report.json`
- `unsafe-secret.report.json`

Copy one into a project repo under `.agentops/runs/{run_id}/run.report.json` to test auto-import.
