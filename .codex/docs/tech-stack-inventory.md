# Tech Stack Inventory

This inventory is for coding agents working on this repository. It describes the actual local implementation stack, not the AgentOps product assets synced into managed projects.

## Application Shape

- Product: AgentOps Workspace MVP.
- Deployment shape: local/self-hosted browser app.
- Frontend URL: `http://localhost:3000`.
- Backend URL: `http://localhost:8080`.
- Database: PostgreSQL with pgvector-ready schema.
- Source-of-truth rule: PostgreSQL is canonical; mounted repo files are generated projections.
- MVP boundary: no direct Codex/agent execution. Future execution stays behind the `AgentRunner` interface.

## Repository Layout

- `api/`: Go backend.
- `fe/`: React frontend.
- `fixtures/run_reports/`: sample run reports for importer, validator, audit, playback.
- `sample-repos/`: local mounted repo examples for sync/import testing.
- `.codex/`: repo-local Codex configuration, custom agents, skills, policies, workflows, checklists, and supporting docs.
- `docker-compose.yml`: local stack definition.
- `.env.example`: documented local environment defaults.

## Backend Stack

- Language: Go `1.23`.
- HTTP framework: Fiber `github.com/gofiber/fiber/v2`.
- Database driver/pool: `github.com/jackc/pgx/v5`.
- YAML parser: `gopkg.in/yaml.v3`.
- UUID helper: `github.com/google/uuid`.
- Main package: `api/cmd/server/main.go`.
- App wiring: `api/internal/app/app.go`.
- Config: `api/internal/config/config.go`.
- Migrations: `api/migrations/001_schema.sql`.

## Backend Commands

```sh
cd api && GOCACHE="$PWD/.gocache" go test ./...
```

When running through Docker:

```sh
docker compose up -d --build api
curl http://localhost:8080/health
```

## Backend Environment

- `DATABASE_URL`: PostgreSQL connection string.
- `API_ADDR`: backend listen address, default `:8080`.
- `AUTH_ENABLED`: local auth flag, default `false`.
- Run import settings are loaded in `api/internal/config/config.go`.

## Frontend Stack

- Language: TypeScript.
- Framework: React `18`.
- Build/dev server: Vite `5`.
- UI: MUI `6`, MUI icons.
- Workflow graph: `@xyflow/react` / React Flow.
- Graph layout: `dagre`.
- Editor: `@monaco-editor/react`.
- Markdown preview: `marked`.
- State: Zustand.
- Tests: Vitest, Testing Library, jsdom.
- Entry: `fe/src/main.tsx`.
- App shell/routing: `fe/src/App.tsx`.

## Frontend Commands

```sh
cd fe && npm run build
cd fe && npm test -- --run
```

Local dev server:

```sh
cd fe && npm run dev
```

When using Docker, source is copied into the image. Rebuild after frontend edits:

```sh
docker compose up -d --build frontend
```

## Frontend Environment

- `VITE_API_BASE_URL`: backend API base URL, default `http://localhost:8080`.

## Docker Stack

`docker-compose.yml` defines:

- `postgres`: `pgvector/pgvector:pg16`, exposed on `5432`.
- `api`: Go backend, exposed on `8080`.
- `frontend`: Vite dev server, exposed on `3000`.

Mounted repo volume:

- Host default: `./sample-repos`.
- Container path: `/workspace/repos`.
- Override with `AGENTOPS_REPO_ROOT`.
- Project repo paths are created explicitly by users and must be visible to the API process; the Docker mount is not a project discovery source.

## Verification Matrix

- Backend service/API behavior: `cd api && GOCACHE="$PWD/.gocache" go test ./...`.
- Frontend compile/type safety: `cd fe && npm run build`.
- Frontend component smoke tests: `cd fe && npm test -- --run`.
- Runtime smoke: `docker compose up -d --build`, `curl http://localhost:8080/health`, open `http://localhost:3000`.
- Docs-only changes: verify file presence and links; runtime tests optional.

## Generated Or Dependency Paths To Ignore

- `fe/node_modules/`
- `fe/dist/`
- `fe/tsconfig.tsbuildinfo`
- `api/.gocache/`
- Docker/PostgreSQL volumes
