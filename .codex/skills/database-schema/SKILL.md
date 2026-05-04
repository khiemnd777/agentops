---
name: database-schema
description: Use when touching migrations, persistence, asset versioning, task run import, reviews, sync state, or DB source-of-truth behavior.
metadata:
  short-description: Database schema changes
---

# Database Schema

Use this skill when touching migrations, persistence, asset versioning, task run import, reviews, or sync state.

## Rules

- PostgreSQL is the canonical source of truth.
- Keep `workspace_id` and `project_id` relationships explicit.
- Do not hard-delete assets or versions in MVP; use status fields.
- Keep pgvector-ready columns/schema intact even when embeddings are stubbed.
- Preserve uniqueness constraints that make sync/import idempotent.

## Import Transactions

Run report import must be transactional. A failed import must not leave partial task run rows, steps, events, assets, or reviews.

## Verification

Run backend tests and include targeted tests for migrations/repository behavior when schema semantics change.
