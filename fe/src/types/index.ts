export type Project = {
  id: string;
  name: string;
  slug: string;
  repo_path: string;
  default_branch: string;
  description: string;
  status: string;
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
  git_repo: boolean;
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
