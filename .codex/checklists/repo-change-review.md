# Checklist: Repo Change Review

Before final handoff, check:

- The change respects the MVP no-agent-execution boundary.
- Database source-of-truth semantics are preserved.
- Mounted repo writes remain path-safe.
- Run report import remains safe, idempotent, and transactional.
- Playback data still represents missing and unexpected workflow nodes.
- Frontend UI does not rely on raw logs or full diffs.
- Tests or verification commands match the changed surface.
- No unrelated user changes were reverted.
