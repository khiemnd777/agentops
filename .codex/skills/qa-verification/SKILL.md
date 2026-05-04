---
name: qa-verification
description: Use before final handoff, especially after cross-stack, persistence, sync, import, playback, frontend, or runtime changes.
metadata:
  short-description: Verification planning
---

# QA Verification

Use this skill before final handoff, especially after cross-stack or data-flow changes.

## Test Selection

- Backend service/API behavior: `cd api && GOCACHE="$PWD/.gocache" go test ./...`
- Frontend compile/runtime type safety: `cd fe && npm run build`
- Frontend smoke/component tests: `cd fe && npm test -- --run`
- Full local smoke: Docker compose plus health checks.

## Required Coverage Areas

- Path safety.
- Repo tree scanning.
- Sync preview/drift detection.
- Run report validation and unsafe secret detection.
- Idempotent and transactional import.
- Workflow audit.
- Playback graph rendering and controls.

If verification is skipped or blocked, document the exact residual risk.
