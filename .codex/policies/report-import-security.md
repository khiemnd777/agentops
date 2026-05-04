# Policy: Report Import Security

Run reports are untrusted input.

## Requirements

- Validate schema before importing.
- Reject secret-like content before inserting task run details.
- Keep import transactional.
- Deduplicate imports by project, run id, and checksum.
- Preserve prior valid imports if a new report is invalid.
- Store summary-level structured data only.

## Unsafe Content Examples

- Private keys.
- API keys.
- JWT-like tokens.
- `.env` style secrets.
- Password, access token, and refresh token fields.
