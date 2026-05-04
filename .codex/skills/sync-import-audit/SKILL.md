---
name: sync-import-audit
description: Use for repository scanner, DB-to-repo sync, repo-to-DB import, run report importer, validator, workflow audit, and playback import prep.
metadata:
  short-description: Sync, import, and audit
---

# Sync, Import, And Audit

Use this skill for repository scanner, DB-to-repo sync, repo-to-DB import, run report importer, validator, and workflow audit changes.

## Sync Rules

- DB asset versions are canonical.
- Repo files are generated projections.
- Repo edits can be imported only as draft versions.
- Compare DB checksum against repo checksum for drift.
- Writes are allowed only for managed paths.

## Import Rules

- Auto-import must be idempotent.
- Use per-project locking.
- Reject unsafe reports containing secret-like content.
- Deduplicate by project, run id, and checksum.
- Updated reports create revisions and trigger re-audit.

## Audit Rules

Compare expected workflow templates to actual report data. Detect missing required nodes, failed/skipped required nodes, dependency order issues, unexpected nodes, missing final summaries, and asset version mismatches where possible.
