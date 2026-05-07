# Policy: Mounted Repository Path Safety

Backend code must never write arbitrary paths.

## Requirements

- Validate repository paths by canonicalizing them, requiring existing readable directories, and using the API process filesystem visibility as the deployment boundary.
- Reject path traversal.
- Do not follow symlinks outside the project repo path.
- Writes are limited to managed Codex-native paths: `AGENTS.md`, `README.md`, and `.codex/**`.
- Do not write to `.git/**`, `.env`, `src/**`, package manifests, Go module files, or production config unless a future explicit policy allows it.
