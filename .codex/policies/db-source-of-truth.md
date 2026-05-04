# Policy: Database Source Of Truth

PostgreSQL is canonical for AgentOps assets, asset versions, chunks, workflow templates, task runs, reviews, imports, and sync state.

## Requirements

- Repo files are projections generated from DB state.
- Repo-to-DB import creates draft versions only.
- Published asset versions must not be overwritten by repo content.
- Drift detection must compare database checksums with repository checksums.
- Sync lock files describe generated state; they do not become canonical state.
