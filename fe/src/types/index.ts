export type Project = {
  id: string;
  workspace_id?: string;
  name: string;
  slug: string;
  repo_path: string;
  host_path_hint?: string | null;
  default_branch: string;
  description: string;
  status: string;
  last_auto_import_at?: string | null;
  created_at?: string;
  updated_at?: string;
};

export type CreateProjectRequest = {
  name: string;
  slug: string;
  repo_path: string;
  description: string;
  default_branch: string;
  create_repo_path: boolean;
};

export type PickFolderResponse = {
  path: string;
};

export type SyncItem = {
  target_path: string;
  action: string;
  status: string;
  db_checksum?: string;
  repo_checksum?: string;
  message?: string;
  rendered_preview?: string;
  asset_type?: string;
  asset_slug?: string;
  diff_summary?: string;
  backup_path?: string;
  allowed_actions?: string[];
};

export type SyncResponse = {
  mode: string;
  items: SyncItem[];
};

export type SyncRun = {
  id: string;
  direction: string;
  status: string;
  summary: string;
  actor_type?: string;
  actor_name?: string;
  transport?: string;
  started_at: string;
  finished_at?: string | null;
};

export type ProjectScanCandidate = {
  path: string;
  kind: string;
  reason: string;
  size_bytes: number;
  matched_keywords?: string[];
};

export type ProjectScanResponse = {
  project_id: string;
  repo_path: string;
  private_path?: string;
  global_path?: string;
  git_repo: boolean;
  readable: boolean;
  scanned_files: number;
  candidates: ProjectScanCandidate[];
  private?: ProjectScanSection;
  global?: ProjectScanSection;
  ignored_dirs: string[];
  truncated: boolean;
};

export type ProjectScanSection = {
  path: string;
  exists: boolean;
  readable: boolean;
  scanned_files: number;
  candidates: ProjectScanCandidate[];
  ignored_dirs: string[];
  truncated: boolean;
};

export type Asset = {
  id: string;
  type: string;
  slug: string;
  name: string;
  description: string;
  current_version_id?: string;
  status: string;
  tags: string[];
};

export type AssetVersion = {
  id: string;
  asset_id: string;
  version: string;
  content: string;
  content_format: string;
  checksum: string;
  status: string;
};

export type AssetPreset = {
  id: string;
  slug: string;
  name: string;
  description: string;
  status: string;
  current_version?: number;
  item_count?: number;
  tags?: string[];
};

export type AssetPresetItem = {
  id?: string;
  template_asset: Asset;
  template_asset_version: AssetVersion;
  target_path: string;
  required: boolean;
  sort_order: number;
};

export type AssetPresetDetail = AssetPreset & {
  items: AssetPresetItem[];
};

export type ApplyPresetItem = {
  template_asset?: Asset;
  template_asset_version?: AssetVersion;
  project_asset?: Asset;
  target_path: string;
  required?: boolean;
  sort_order?: number;
  status?: string;
  action?: string;
  already_exists?: boolean;
  message?: string;
};

export type ApplyPresetResponse = {
  project_id: string;
  preset_id: string;
  items: ApplyPresetItem[];
};

export type TaskRun = {
  id: string;
  external_run_id: string;
  title: string;
  status: string;
  review_status: string;
  expected_workflow_id: string;
  expected_workflow_version: string;
  started_at?: string;
  finished_at?: string;
};

export type PlaybackNode = {
  id: string;
  label: string;
  type: string;
  ref?: string;
  status: string;
  required?: boolean;
  unexpected?: boolean;
  purpose?: string;
  short_summary?: string;
  action_summary?: string[];
  output_summary?: string[];
  input_summary?: string;
  executor?: string;
  version?: string;
  position?: { x: number; y: number };
  compliance?: Record<string, unknown>;
};

export type PlaybackResponse = {
  task_run_id: string;
  external_run_id: string;
  workflow: { id: string; version: string };
  review_status: string;
  import_summary?: Record<string, number | boolean>;
  nodes: PlaybackNode[];
  edges: Array<{ id: string; source: string; target: string; status: string }>;
  events: Array<{ node_key: string; event_type: string; event_time: string; summary: string }>;
  review: { summary: string; violations: unknown[]; warnings: unknown[] };
};

export type WorkflowTemplate = {
  id: string;
  name: string;
  slug: string;
  version: string;
  definition_yaml: string;
  status: string;
};
