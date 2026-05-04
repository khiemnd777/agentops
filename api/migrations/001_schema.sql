CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version TEXT PRIMARY KEY,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workspaces (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id),
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  repo_path TEXT NOT NULL,
  host_path_hint TEXT NULL,
  default_branch TEXT NOT NULL DEFAULT 'main',
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  last_auto_import_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, slug)
);

CREATE TABLE IF NOT EXISTS agentic_assets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id),
  scope TEXT NOT NULL DEFAULT 'template',
  project_id UUID NULL REFERENCES projects(id),
  template_asset_id UUID NULL REFERENCES agentic_assets(id),
  type TEXT NOT NULL,
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  current_version_id UUID NULL,
  status TEXT NOT NULL DEFAULT 'active',
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT agentic_assets_scope_check CHECK (scope IN ('template', 'project')),
  CONSTRAINT agentic_assets_scope_project_check CHECK (
    (scope = 'template' AND project_id IS NULL)
    OR (scope = 'project' AND project_id IS NOT NULL)
  ),
  CONSTRAINT agentic_assets_template_source_check CHECK (template_asset_id IS NULL OR scope = 'project')
);

ALTER TABLE agentic_assets ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT 'template';
ALTER TABLE agentic_assets ADD COLUMN IF NOT EXISTS project_id UUID NULL REFERENCES projects(id);
ALTER TABLE agentic_assets ADD COLUMN IF NOT EXISTS template_asset_id UUID NULL REFERENCES agentic_assets(id);
ALTER TABLE agentic_assets DROP CONSTRAINT IF EXISTS agentic_assets_workspace_id_type_slug_key;
ALTER TABLE agentic_assets DROP CONSTRAINT IF EXISTS agentic_assets_scope_check;
ALTER TABLE agentic_assets ADD CONSTRAINT agentic_assets_scope_check CHECK (scope IN ('template', 'project'));
ALTER TABLE agentic_assets DROP CONSTRAINT IF EXISTS agentic_assets_scope_project_check;
ALTER TABLE agentic_assets ADD CONSTRAINT agentic_assets_scope_project_check CHECK (
  (scope = 'template' AND project_id IS NULL)
  OR (scope = 'project' AND project_id IS NOT NULL)
);
ALTER TABLE agentic_assets DROP CONSTRAINT IF EXISTS agentic_assets_template_source_check;
ALTER TABLE agentic_assets ADD CONSTRAINT agentic_assets_template_source_check CHECK (template_asset_id IS NULL OR scope = 'project');

CREATE TABLE IF NOT EXISTS agentic_asset_versions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_id UUID NOT NULL REFERENCES agentic_assets(id),
  version TEXT NOT NULL,
  content TEXT NOT NULL,
  content_format TEXT NOT NULL DEFAULT 'markdown',
  checksum TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  published_at TIMESTAMPTZ NULL,
  UNIQUE(asset_id, version)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_agentic_assets_template_slug ON agentic_assets(workspace_id, type, slug) WHERE scope = 'template';
CREATE UNIQUE INDEX IF NOT EXISTS idx_agentic_assets_project_slug ON agentic_assets(project_id, type, slug) WHERE scope = 'project';

CREATE TABLE IF NOT EXISTS agentic_asset_chunks (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  asset_version_id UUID NOT NULL REFERENCES agentic_asset_versions(id),
  chunk_index INT NOT NULL,
  content TEXT NOT NULL,
  checksum TEXT NOT NULL,
  embedding vector(1536) NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS asset_presets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id),
  slug TEXT NOT NULL,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  current_version INT NOT NULL DEFAULT 1,
  tags JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, slug)
);

ALTER TABLE asset_presets ADD COLUMN IF NOT EXISTS current_version INT NOT NULL DEFAULT 1;

CREATE TABLE IF NOT EXISTS asset_preset_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  preset_id UUID NOT NULL REFERENCES asset_presets(id),
  asset_id UUID NOT NULL REFERENCES agentic_assets(id),
  asset_version_id UUID NOT NULL REFERENCES agentic_asset_versions(id),
  target_path TEXT NOT NULL DEFAULT '',
  sort_order INT NOT NULL DEFAULT 0,
  required BOOLEAN NOT NULL DEFAULT true,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(preset_id, asset_version_id)
);

CREATE TABLE IF NOT EXISTS project_asset_bindings (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id),
  asset_id UUID NOT NULL REFERENCES agentic_assets(id),
  asset_version_id UUID NOT NULL REFERENCES agentic_asset_versions(id),
  target_path TEXT NOT NULL,
  sync_policy TEXT NOT NULL DEFAULT 'protect_drift',
  sync_status TEXT NOT NULL DEFAULT 'synced',
  last_db_checksum TEXT,
  last_repo_checksum TEXT,
  last_synced_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(project_id, target_path)
);

COMMENT ON COLUMN agentic_assets.scope IS 'template assets are shared workspace assets; project assets are per-project copies that may share type/slug with templates.';
COMMENT ON COLUMN agentic_assets.template_asset_id IS 'For project assets copied from a preset, points at the source template asset. Existing bindings remain valid; new preset copies should bind/sync project-scoped assets.';
COMMENT ON TABLE asset_presets IS 'Shared asset presets group pinned template asset versions for copying into project assets.';
COMMENT ON TABLE asset_preset_items IS 'Preset membership is version-pinned. Applying a preset copies the pinned content and must not overwrite an existing project asset with the same type/slug.';

CREATE TABLE IF NOT EXISTS sync_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id),
  direction TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'running',
  summary TEXT NOT NULL DEFAULT '',
  started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  finished_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS sync_run_items (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  sync_run_id UUID NOT NULL REFERENCES sync_runs(id),
  binding_id UUID NULL REFERENCES project_asset_bindings(id),
  target_path TEXT NOT NULL,
  action TEXT NOT NULL,
  status TEXT NOT NULL,
  before_checksum TEXT,
  after_checksum TEXT,
  message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workflow_templates (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  workspace_id UUID NOT NULL REFERENCES workspaces(id),
  name TEXT NOT NULL,
  slug TEXT NOT NULL,
  version TEXT NOT NULL,
  definition_yaml TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'draft',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(workspace_id, slug, version)
);

CREATE TABLE IF NOT EXISTS task_runs (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id),
  workflow_template_id UUID NULL REFERENCES workflow_templates(id),
  external_run_id TEXT NOT NULL,
  title TEXT NOT NULL,
  input_summary TEXT NOT NULL DEFAULT '',
  change_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
  final_summary TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'completed',
  review_status TEXT NOT NULL DEFAULT 'needs_manual_review',
  expected_workflow_id TEXT,
  expected_workflow_version TEXT,
  actual_workflow_id TEXT,
  actual_workflow_version TEXT,
  report_checksum TEXT NOT NULL,
  report_source_path TEXT NOT NULL,
  imported_at TIMESTAMPTZ,
  import_status TEXT NOT NULL DEFAULT 'imported',
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(project_id, external_run_id)
);

CREATE TABLE IF NOT EXISTS task_run_steps (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_run_id UUID NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
  node_key TEXT NOT NULL,
  node_type TEXT NOT NULL,
  title TEXT NOT NULL,
  expected_ref TEXT,
  actual_ref TEXT,
  asset_version TEXT NULL,
  executor TEXT NULL,
  status TEXT NOT NULL,
  purpose TEXT NOT NULL DEFAULT '',
  input_summary TEXT NOT NULL DEFAULT '',
  action_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
  output_summary JSONB NOT NULL DEFAULT '[]'::jsonb,
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS task_run_node_events (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_run_id UUID NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
  node_key TEXT NOT NULL,
  event_type TEXT NOT NULL,
  event_time TIMESTAMPTZ NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS task_run_assets (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_run_id UUID NOT NULL REFERENCES task_runs(id) ON DELETE CASCADE,
  asset_id UUID NULL REFERENCES agentic_assets(id),
  asset_version_id UUID NULL REFERENCES agentic_asset_versions(id),
  asset_type TEXT NOT NULL,
  asset_slug TEXT NOT NULL,
  asset_version TEXT NULL,
  usage_role TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS task_run_reviews (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  task_run_id UUID NOT NULL REFERENCES task_runs(id),
  review_status TEXT NOT NULL,
  reviewer_type TEXT NOT NULL DEFAULT 'system',
  summary TEXT NOT NULL DEFAULT '',
  expected_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  actual_json JSONB NOT NULL DEFAULT '{}'::jsonb,
  violations_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  warnings_json JSONB NOT NULL DEFAULT '[]'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS task_run_imports (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  project_id UUID NOT NULL REFERENCES projects(id),
  task_run_id UUID NULL REFERENCES task_runs(id),
  run_id TEXT NOT NULL,
  source_path TEXT NOT NULL,
  source_checksum TEXT NOT NULL,
  import_revision INT NOT NULL DEFAULT 1,
  import_status TEXT NOT NULL,
  error_message TEXT NULL,
  imported_at TIMESTAMPTZ NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(project_id, run_id, source_checksum)
);

CREATE INDEX IF NOT EXISTS idx_assets_search ON agentic_assets USING gin (to_tsvector('simple', name || ' ' || slug || ' ' || type));
CREATE INDEX IF NOT EXISTS idx_asset_versions_content ON agentic_asset_versions USING gin (to_tsvector('simple', content));
CREATE INDEX IF NOT EXISTS idx_task_runs_project ON task_runs(project_id, imported_at DESC);
