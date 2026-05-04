---
name: frontend-ui
description: Use for React, Vite, MUI, React Flow, Zustand, and frontend DTO/API changes under fe/.
metadata:
  short-description: Frontend UI changes
---

# Frontend UI

Use this skill for React/Vite/MUI changes under `fe/`.

## Boundaries

- `src/api/client.ts`: HTTP calls and API shapes.
- `src/types`: frontend DTO/type definitions.
- `src/pages`: route-level pages.
- `src/components`: reusable views and controls.
- `src/store`: playback and UI state.

## UI Principles

- Build dashboard surfaces, not marketing pages.
- Use dense, scannable operational layouts.
- Keep cards for repeated items, modals, or framed tools.
- Use MUI controls and React Flow patterns already present in the app.
- Do not show raw logs or full diffs in task review UI.

## Verification

Run:

```sh
cd fe && npm run build
cd fe && npm test -- --run
```
