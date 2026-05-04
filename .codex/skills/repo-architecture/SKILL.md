---
name: repo-architecture
description: Use before changing cross-cutting behavior or when ownership is unclear across backend, frontend, database, sync, import, audit, or playback.
metadata:
  short-description: Repo architecture routing
---

# Repo Architecture

Use this skill before changing cross-cutting behavior or when the affected area is unclear.

## Map The Change

- `api/internal/handlers`: HTTP routing and request/response mapping.
- `api/internal/services`: business logic for assets, sync, scanner, importer, workflow, playback.
- `api/internal/repo`: SQL persistence boundary.
- `api/internal/domain`: shared domain types.
- `api/migrations`: schema changes.
- `fe/src/api`: frontend API client.
- `fe/src/pages`: top-level dashboard pages.
- `fe/src/components`: reusable UI surfaces.
- `fe/src/store`: playback/UI state.
- `fe/src/types`: shared frontend types.

## Expected Approach

1. Identify whether the task is backend, frontend, database, or cross-stack.
2. Inspect the smallest set of files needed before editing.
3. Preserve handler/service/repository separation on the backend.
4. Preserve API client/page/component/type separation on the frontend.
5. Update fixtures or tests when behavior changes.
