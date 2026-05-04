---
name: workflow-playback
description: Use for playback APIs and React Flow review UI, including graph nodes, edges, events, controls, compliance badges, and timelines.
metadata:
  short-description: Workflow playback
---

# Workflow Playback

Use this skill for playback APIs and React Flow review UI.

## Backend Expectations

- Playback responses must include nodes, edges, events, review summary, violations, and warnings.
- Missing expected nodes must appear as missing/skipped.
- Unexpected actual nodes must appear with an explicit unexpected marker.
- Keep response payloads summary-level; no raw logs or full diffs.

## Frontend Expectations

- Default view is Flow View.
- Use DAG layout when positions are absent.
- Nodes must show label, type, purpose, status, summary, and compliance badges.
- Right panel must show purpose, input, action summary, output summary, compliance, and mini timeline.
- Playback controls must support play, pause, restart, stepping, speed, and final-state mode.
