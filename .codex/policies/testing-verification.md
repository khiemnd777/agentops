# Policy: Testing And Verification

Every change needs verification proportional to risk.

## Defaults

- Backend behavior: run Go tests.
- Frontend behavior: run build and Vitest.
- Docker/runtime changes: run Docker compose smoke checks.
- Docs-only changes: inspect file list and links; tests are optional.

## Report Blockers

If verification cannot run, final handoff must say which command was skipped, why, and what risk remains.
