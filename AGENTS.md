# AgentOps Workspace Repo Agent Guide

This file is for coding agents working on this repository itself. It is not a product asset template and must not be synced into user project repositories.

## Repo Goal

AgentOps Workspace is the operational brain for coding agents across project repositories. It centralizes agent knowledge, workflows, policies, run reports, audits, reviews, playback, and synchronization so external coding agents can work consistently, transparently, and safely across repos.

AgentOps Workspace is a local/self-hosted web app with:

- Backend: Go, Fiber, PostgreSQL/pgvector-ready schema in `api/`.
- Frontend: React, TypeScript, Vite, MUI, React Flow, Zustand, Monaco in `fe/`.
- Fixtures: run report examples in `fixtures/run_reports/`.
- Local sample repos: `sample-repos/`.
- Repo-local Codex configuration, guidance, custom agents, and skills: `.codex/`.

## Fast Orientation

Read these first when entering the repo:

- `.codex/docs/tech-stack-inventory.md`: runtime, libraries, commands, containers, config.
- `.codex/docs/module-inventory.md`: module map with files and descriptions.

## Product Boundary

The MVP does not execute Codex or other coding agents directly. Keep runner work behind the future `AgentRunner` interface only. The implemented task review flow is file-based: external agents write `.codex/reports/runs/{run_id}/run.report.json`, then this app imports, validates, audits, and renders playback.

## Source Of Truth

PostgreSQL is canonical for AgentOps assets, versions, chunks, workflow templates, task runs, reviews, and sync state. Physical files in mounted repositories are generated/materialized projections. Repo file imports may create draft versions only; never overwrite published database versions from repo content.

## AgentOps Local Agent

For AgentOps-local sync tasks, prefer the `agentops` CLI. From this repository, run it with:

- `cd api && go run ./cmd/agentops project manifest --project <slug-or-id>`
- `cd api && go run ./cmd/agentops sync preview --project <slug-or-id> --mode files-to-db`
- `cd api && go run ./cmd/agentops sync files-to-db --project <slug-or-id>`
- `cd api && go run ./cmd/agentops sync db-to-files --project <slug-or-id>`
- `cd api && go run ./cmd/agentops report import --project <slug-or-id>`

Do not call MCP for local AgentOps maintenance unless explicitly testing MCP behavior. MCP is primarily for project repo agents.

## Required Local Skillset

Before non-trivial changes, read the relevant repo-local skill:

- `.codex/skills/repo-architecture/SKILL.md`
- `.codex/skills/backend-api/SKILL.md`
- `.codex/skills/frontend-ui/SKILL.md`
- `.codex/skills/database-schema/SKILL.md`
- `.codex/skills/sync-import-audit/SKILL.md`
- `.codex/skills/workflow-playback/SKILL.md`
- `.codex/skills/qa-verification/SKILL.md`

Use custom agent definitions in `.codex/agents/` when splitting or delegating work:

- `.codex/agents/backend-api-agent.toml`
- `.codex/agents/frontend-ui-agent.toml`
- `.codex/agents/database-agent.toml`
- `.codex/agents/import-audit-agent.toml`
- `.codex/agents/devops-agent.toml`

## Repo Policies

Follow `.codex/policies/` for:

- DB source-of-truth behavior.
- Safe mounted-repo file writes.
- MVP no-agent-execution boundary.
- Report import security.
- Verification expectations.

## Collaboration Defaults

- Do not start long-running local dev servers such as `npm run dev` unless the user explicitly asks for it.
- Do not implement code changes by default. First produce a plan and wait for the user's approval before editing files, unless the user explicitly asks for immediate implementation.
- For UI-related requests, provide a text mockup for review and wait for the user's approval before implementation.

## Standard Change Workflow

For substantial changes, follow `.codex/workflows/repo-change-workflow.md`.

Before final response, run through `.codex/checklists/repo-change-review.md`.

## Verification Defaults

Use the narrowest meaningful verification first:

- Backend: `cd api && GOCACHE="$PWD/.gocache" go test ./...`
- Frontend build: `cd fe && npm run build`
- Frontend tests: `cd fe && npm test -- --run`
- Docker smoke: `docker compose up -d --build`, then check `http://localhost:8080/health` and `http://localhost:3000`

If a command cannot be run, state the reason and the risk left behind.
