---
name: backend-api
description: Use for Go/Fiber API changes under api/, including handlers, services, repositories, domain types, structured errors, and backend tests.
metadata:
  short-description: Backend API changes
---

# Backend API

Use this skill for Go/Fiber API changes under `api/`.

## Boundaries

- Handlers parse HTTP input, call services, and return JSON.
- Services own product behavior and validation.
- Repositories own SQL and database mapping.
- Domain types should stay reusable and transport-neutral where practical.

## Rules

- Keep all workspace/project scoping explicit.
- Return structured API errors instead of raw internal errors.
- Keep mounted repo path validation in service/path guard code.
- Use transactions for multi-table writes, especially imports and sync state changes.
- Do not introduce direct coding-agent execution.

## Verification

Run:

```sh
cd api && GOCACHE="$PWD/.gocache" go test ./...
```
