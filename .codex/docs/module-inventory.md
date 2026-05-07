# Module Inventory

This inventory maps product modules to source files so coding agents can find the right code quickly. It intentionally excludes generated output and dependency folders.

## Root And Local Ops

| File | Description |
| --- | --- |
| `AGENTS.md` | Root operating guide for coding agents working on this repo. |
| `.codex/README.md` | Index for repo-local Codex configuration and guidance. |
| `.codex/docs/tech-stack-inventory.md` | Runtime, stack, commands, container, and config inventory. |
| `.codex/docs/module-inventory.md` | This file: source module map. |
| `.codex/skills/` | Repo-local Codex skills. |
| `.env.example` | Example local environment values. |
| `docker-compose.yml` | Local stack: PostgreSQL/pgvector, backend, frontend. |
| `README.md` | Human-facing setup and MVP usage notes. |

## Backend Entry And Wiring

| File | Description |
| --- | --- |
| `api/cmd/server/main.go` | Backend process entrypoint; loads config, connects DB, runs migrations/seeds, starts Fiber. |
| `api/cmd/agentops/main.go` | AgentOps-local CLI entrypoint for project manifest, sync preview/apply, reconcile, and report import. |
| `api/internal/app/app.go` | Fiber app construction, middleware, `/health`, `/api` group, handler registration. |
| `api/internal/config/config.go` | Environment-backed config, auth flag, run import settings. |
| `api/internal/db/db.go` | PostgreSQL connection pool and migration runner. |
| `api/Dockerfile` | Backend container image. |
| `api/go.mod` | Backend module and dependency list. |

## Backend Domain And Persistence

| File | Description |
| --- | --- |
| `api/internal/domain/types.go` | Shared backend domain structs: workspace, project, assets, versions, bindings, workflows, task runs, repo tree, import summary. |
| `api/internal/repo/store.go` | SQL repository methods for workspace, dashboard, projects, assets, versions, workflows, task runs, reviews, imports, and sync records. |
| `api/migrations/001_schema.sql` | Full MVP schema including workspaces, projects, assets, chunks, bindings, sync, workflows, task runs, events, reviews, imports, and pgvector-ready embedding column. |

## Backend HTTP API

| File | Description |
| --- | --- |
| `api/internal/handlers/handlers.go` | All API route handlers. Parses requests, calls services/store, returns JSON. |

Primary routes registered here:

- Workspace/dashboard: `GET /api/workspace/default`, `GET /api/dashboard`.
- Projects: `GET/POST /api/projects`, `GET /api/projects/:id`, scan, repo tree, sync preview/apply.
- Assets: list/create/get/versions/publish/clone.
- Composer: preview/apply project asset profile.
- Workflows: list/create/get/update/preview graph.
- Task runs/reviews/playback: project task runs, project reviews, global reviews, graph/events/playback, manual review.
- Reports/imports: report playback by run id, manual import, import records.

## Backend Services

| File | Description |
| --- | --- |
| `api/internal/services/checksum.go` | SHA-256 helpers for strings, bytes, and files. |
| `api/internal/services/path_guard.go` | Repo path canonicalization and safe generated-file target validation for project repos. |
| `api/internal/services/scanner.go` | Repo tree viewer, managed-file detection, scan preview helpers. |
| `api/internal/services/sync.go` | DB-to-repo, repo-to-DB, compare-only sync planning/apply, Codex-native managed file rendering, `.codex` manifest/lock generation, drift/diff/conflict actions. |
| `api/internal/services/seeds.go` | Idempotent default workspace assets and workflow seed content. |
| `api/internal/services/run_report.go` | Run report DTOs, JSON validation, timestamp validation, secret-like content detection. |
| `api/internal/services/importer.go` | File-based run report scanner/importer, per-project in-memory lock, idempotency, transactional task run import, audit trigger. |
| `api/internal/services/workflow.go` | Workflow YAML parser, expected graph preview, workflow audit engine, assets lock parsing, review persistence. |
| `api/internal/services/playback.go` | Playback response builder; merges expected workflow graph with actual imported run data. |
| `api/internal/services/runner.go` | Future `AgentRunner` interface only; no MVP execution implementation. |

## Backend Tests

| File | Description |
| --- | --- |
| `api/internal/services/path_guard_test.go` | Path safety tests for repo validation, symlink containment, and managed targets. |
| `api/internal/services/scanner_test.go` | Repo tree, managed file classification, header stripping, diff summary tests. |
| `api/internal/services/run_report_test.go` | Valid fixture acceptance and unsafe secret rejection. |
| `api/internal/services/workflow_test.go` | Workflow audit and graph preview validation. |
| `api/internal/services/importer_transaction_test.go` | Transaction rollback behavior for failed imports. |

## Frontend App Shell

| File | Description |
| --- | --- |
| `fe/src/main.tsx` | React entrypoint and global imports. |
| `fe/src/App.tsx` | App shell, hash routing, project-centric sidebar, workspace tools navigation. |
| `fe/src/styles.css` | Global layout, dashboard, editor, repo tree, sync, and workflow playback styling. |
| `fe/src/api/client.ts` | Typed fetch helper and API base URL handling. |
| `fe/src/types/index.ts` | Frontend DTOs for projects, assets, versions, task runs, playback, workflows. |
| `fe/vite.config.ts` | Vite/Vitest config. |
| `fe/Dockerfile` | Frontend container image running Vite dev server. |
| `fe/package.json` | Frontend scripts and dependencies. |

## Frontend Pages

| File | Description |
| --- | --- |
| `fe/src/pages/DashboardPage.tsx` | Workspace overview metrics, recent reviews, import errors. |
| `fe/src/pages/ProjectsPage.tsx` | Project list, initialization wizard, project-centric detail sections, overview, sync panel, contextual workflows. |
| `fe/src/pages/AssetsPage.tsx` | Workspace asset library, search/filter, create draft, Monaco editor, markdown preview, publish/clone/version history. |
| `fe/src/pages/WorkflowsPage.tsx` | Workflow YAML designer and React Flow graph preview. |
| `fe/src/pages/ReviewsPage.tsx` | Global review center and task-run playback route. |

## Frontend Components

| File | Description |
| --- | --- |
| `fe/src/components/ProjectComposer.tsx` | Project asset profile composer and generated file preview/apply entry. |
| `fe/src/components/RepoTree.tsx` | Project repository tree viewer showing directories plus managed files. |
| `fe/src/components/TaskRunsPanel.tsx` | Project task runs/reviews list with auto-import status and playback links. |
| `fe/src/components/WorkflowPlayback.tsx` | Animated workflow playback review: React Flow graph, controls, timeline/compliance tabs, node detail panel. |
| `fe/src/store/playback.ts` | Zustand playback state: active node, playing, speed, final-state mode. |

## Frontend Tests

| File | Description |
| --- | --- |
| `fe/src/components/RepoTree.test.tsx` | Repo tree render smoke test. |
| `fe/src/components/WorkflowPlayback.test.tsx` | Playback graph/control/detail smoke test. |
| `fe/src/pages/DashboardPage.test.tsx` | Dashboard metrics/recent review smoke test. |
| `fe/src/pages/ProjectsPage.test.tsx` | Project initialization wizard smoke test. |
| `fe/src/testSetup.ts` | Vitest/testing-library setup. |

## Fixtures

| File | Description |
| --- | --- |
| `fixtures/run_reports/fullstack-feature-success.report.json` | Valid successful run report fixture. |
| `fixtures/run_reports/fullstack-feature-warning.report.json` | Valid warning/non-pass review fixture. |
| `fixtures/run_reports/invalid-schema.report.json` | Invalid schema fixture for validator/importer tests. |
| `fixtures/run_reports/unsafe-secret.report.json` | Unsafe secret-like content fixture for rejection tests. |

## Product Module To File Map

| Product Module | Main Files |
| --- | --- |
| `workspace` | `api/internal/repo/store.go`, `api/internal/services/seeds.go`, `api/internal/handlers/handlers.go`, `api/migrations/001_schema.sql` |
| `project` | `api/internal/repo/store.go`, `api/internal/handlers/handlers.go`, `fe/src/pages/ProjectsPage.tsx`, `fe/src/App.tsx` |
| `repository` / `repo_locator` | `api/internal/services/path_guard.go`, `api/internal/services/scanner.go`, `api/internal/services/sync.go` |
| `repo_tree` | `api/internal/services/scanner.go`, `fe/src/components/RepoTree.tsx` |
| `asset` / `asset_version` / `asset_chunk` | `api/internal/repo/store.go`, `api/migrations/001_schema.sql`, `fe/src/pages/AssetsPage.tsx` |
| `composer` | `api/internal/handlers/handlers.go`, `api/internal/services/sync.go`, `fe/src/components/ProjectComposer.tsx` |
| `scanner` | `api/internal/services/scanner.go`, `api/internal/handlers/handlers.go` |
| `sync` | `api/internal/services/sync.go`, `api/internal/services/path_guard.go`, `fe/src/pages/ProjectsPage.tsx` |
| `workflow` | `api/internal/services/workflow.go`, `api/internal/repo/store.go`, `fe/src/pages/WorkflowsPage.tsx` |
| `task_run` | `api/internal/repo/store.go`, `api/internal/services/importer.go`, `fe/src/components/TaskRunsPanel.tsx` |
| `run_importer` | `api/internal/services/importer.go`, `api/internal/services/run_report.go` |
| `run_report_validator` | `api/internal/services/run_report.go`, `api/internal/services/run_report_test.go` |
| `workflow_audit` | `api/internal/services/workflow.go`, `api/internal/services/workflow_test.go` |
| `workflow_playback` | `api/internal/services/playback.go`, `fe/src/components/WorkflowPlayback.tsx`, `fe/src/store/playback.ts` |
| `dashboard` | `api/internal/repo/store.go`, `api/internal/handlers/handlers.go`, `fe/src/pages/DashboardPage.tsx` |
| `review` | `api/internal/repo/store.go`, `api/internal/services/workflow.go`, `fe/src/pages/ReviewsPage.tsx`, `fe/src/components/TaskRunsPanel.tsx` |

## Common Change Starting Points

- Add/modify API route: start at `api/internal/handlers/handlers.go`, then service/repo.
- Change DB schema: start at `api/migrations/001_schema.sql`, then `api/internal/repo/store.go`, then tests.
- Change repo path safety: start at `api/internal/services/path_guard.go` and `api/internal/services/path_guard_test.go`.
- Change sync behavior: start at `api/internal/services/sync.go` and `fe/src/pages/ProjectsPage.tsx`.
- Change run report validation: start at `api/internal/services/run_report.go` and fixtures.
- Change import transaction behavior: start at `api/internal/services/importer.go` and `api/internal/services/importer_transaction_test.go`.
- Change workflow audit: start at `api/internal/services/workflow.go`.
- Change playback graph: start at `api/internal/services/playback.go` and `fe/src/components/WorkflowPlayback.tsx`.
- Change project-centric navigation/layout: start at `fe/src/App.tsx`, `fe/src/pages/ProjectsPage.tsx`, and `fe/src/styles.css`.
