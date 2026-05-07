package domain

import (
	"encoding/json"
	"time"
)

type Workspace struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Project struct {
	ID               string     `json:"id"`
	WorkspaceID      string     `json:"workspace_id"`
	Name             string     `json:"name"`
	Slug             string     `json:"slug"`
	RepoPath         string     `json:"repo_path"`
	HostPathHint     *string    `json:"host_path_hint"`
	DefaultBranch    string     `json:"default_branch"`
	Description      string     `json:"description"`
	Status           string     `json:"status"`
	LastAutoImportAt *time.Time `json:"last_auto_import_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type RepoScanResult struct {
	ProjectID    string              `json:"project_id,omitempty"`
	RepoPath     string              `json:"repo_path"`
	PrivatePath  string              `json:"private_path,omitempty"`
	GlobalPath   string              `json:"global_path,omitempty"`
	GitRepo      bool                `json:"git_repo"`
	Readable     bool                `json:"readable"`
	ScannedFiles int                 `json:"scanned_files"`
	Candidates   []RepoScanCandidate `json:"candidates"`
	Private      *RepoScanSection    `json:"private,omitempty"`
	Global       *RepoScanSection    `json:"global,omitempty"`
	IgnoredDirs  []string            `json:"ignored_dirs"`
	Truncated    bool                `json:"truncated"`
}

type RepoScanSection struct {
	Path         string              `json:"path"`
	Exists       bool                `json:"exists"`
	Readable     bool                `json:"readable"`
	ScannedFiles int                 `json:"scanned_files"`
	Candidates   []RepoScanCandidate `json:"candidates"`
	IgnoredDirs  []string            `json:"ignored_dirs"`
	Truncated    bool                `json:"truncated"`
}

type RepoScanCandidate struct {
	Path            string   `json:"path"`
	Kind            string   `json:"kind"`
	Reason          string   `json:"reason"`
	SizeBytes       int64    `json:"size_bytes"`
	MatchedKeywords []string `json:"matched_keywords,omitempty"`
}

type Asset struct {
	ID               string          `json:"id"`
	WorkspaceID      string          `json:"workspace_id"`
	Scope            string          `json:"scope"`
	ProjectID        *string         `json:"project_id,omitempty"`
	TemplateAssetID  *string         `json:"template_asset_id,omitempty"`
	Type             string          `json:"type"`
	Slug             string          `json:"slug"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	CurrentVersionID *string         `json:"current_version_id"`
	Status           string          `json:"status"`
	Tags             json.RawMessage `json:"tags"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type AssetListFilter struct {
	Scope     string `json:"scope,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

type AssetVersion struct {
	ID            string          `json:"id"`
	AssetID       string          `json:"asset_id"`
	Version       string          `json:"version"`
	Content       string          `json:"content"`
	ContentFormat string          `json:"content_format"`
	Checksum      string          `json:"checksum"`
	Metadata      json.RawMessage `json:"metadata"`
	Status        string          `json:"status"`
	CreatedAt     time.Time       `json:"created_at"`
	PublishedAt   *time.Time      `json:"published_at"`
}

type AssetWithVersion struct {
	Asset   Asset        `json:"asset"`
	Version AssetVersion `json:"version"`
}

type AssetPreset struct {
	ID             string          `json:"id"`
	WorkspaceID    string          `json:"workspace_id"`
	Slug           string          `json:"slug"`
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Status         string          `json:"status"`
	CurrentVersion int             `json:"current_version"`
	Tags           json.RawMessage `json:"tags"`
	ItemCount      int             `json:"item_count"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

type AssetPresetItem struct {
	ID             string       `json:"id"`
	PresetID       string       `json:"preset_id"`
	AssetID        string       `json:"asset_id"`
	AssetVersionID string       `json:"asset_version_id"`
	TargetPath     string       `json:"target_path"`
	SortOrder      int          `json:"sort_order"`
	Required       bool         `json:"required"`
	Asset          Asset        `json:"asset"`
	Version        AssetVersion `json:"version"`
	CreatedAt      time.Time    `json:"created_at"`
}

type AssetPresetDetail struct {
	AssetPreset
	Items []AssetPresetItem `json:"items"`
}

type ApplyAssetPresetResult struct {
	PresetID  string                       `json:"preset_id"`
	ProjectID string                       `json:"project_id"`
	Created   int                          `json:"created"`
	Skipped   int                          `json:"skipped"`
	Items     []ApplyAssetPresetItemResult `json:"items"`
}

type ApplyAssetPresetItemResult struct {
	PresetItemID           string `json:"preset_item_id"`
	TemplateAssetID        string `json:"template_asset_id"`
	TemplateAssetVersionID string `json:"template_asset_version_id"`
	ProjectAssetID         string `json:"project_asset_id,omitempty"`
	ProjectAssetVersionID  string `json:"project_asset_version_id,omitempty"`
	ExistingAssetID        string `json:"existing_asset_id,omitempty"`
	Type                   string `json:"type"`
	Slug                   string `json:"slug"`
	TargetPath             string `json:"target_path,omitempty"`
	Status                 string `json:"status"`
	Message                string `json:"message,omitempty"`
}

type ProjectAssetBinding struct {
	ID               string     `json:"id"`
	ProjectID        string     `json:"project_id"`
	AssetID          string     `json:"asset_id"`
	AssetVersionID   string     `json:"asset_version_id"`
	TargetPath       string     `json:"target_path"`
	SyncPolicy       string     `json:"sync_policy"`
	SyncStatus       string     `json:"sync_status"`
	LastDBChecksum   string     `json:"last_db_checksum"`
	LastRepoChecksum string     `json:"last_repo_checksum"`
	LastSyncedAt     *time.Time `json:"last_synced_at"`
}

type ProjectAsset struct {
	ProjectID       string              `json:"project_id"`
	TargetPath      string              `json:"target_path"`
	TemplateAssetID string              `json:"template_asset_id,omitempty"`
	Binding         ProjectAssetBinding `json:"binding"`
	Asset           Asset               `json:"asset"`
	Version         AssetVersion        `json:"version"`
}

type WorkflowTemplate struct {
	ID             string    `json:"id"`
	WorkspaceID    string    `json:"workspace_id"`
	Name           string    `json:"name"`
	Slug           string    `json:"slug"`
	Version        string    `json:"version"`
	DefinitionYAML string    `json:"definition_yaml"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type TaskRun struct {
	ID                      string          `json:"id"`
	ProjectID               string          `json:"project_id"`
	WorkflowTemplateID      *string         `json:"workflow_template_id"`
	ExternalRunID           string          `json:"external_run_id"`
	Title                   string          `json:"title"`
	InputSummary            string          `json:"input_summary"`
	ChangeSummary           json.RawMessage `json:"change_summary"`
	FinalSummary            string          `json:"final_summary"`
	Status                  string          `json:"status"`
	ReviewStatus            string          `json:"review_status"`
	ExpectedWorkflowID      string          `json:"expected_workflow_id"`
	ExpectedWorkflowVersion string          `json:"expected_workflow_version"`
	ActualWorkflowID        string          `json:"actual_workflow_id"`
	ActualWorkflowVersion   string          `json:"actual_workflow_version"`
	ReportChecksum          string          `json:"report_checksum"`
	ReportSourcePath        string          `json:"report_source_path"`
	ImportedAt              *time.Time      `json:"imported_at"`
	ImportStatus            string          `json:"import_status"`
	StartedAt               *time.Time      `json:"started_at"`
	FinishedAt              *time.Time      `json:"finished_at"`
	CreatedAt               time.Time       `json:"created_at"`
	UpdatedAt               time.Time       `json:"updated_at"`
}

type RepoTreeNode struct {
	Name     string         `json:"name"`
	Path     string         `json:"path"`
	Type     string         `json:"type"`
	Children []RepoTreeNode `json:"children,omitempty"`
}

type ImportSummary struct {
	Scanned   int  `json:"scanned"`
	Imported  int  `json:"imported"`
	Unchanged int  `json:"unchanged"`
	Updated   int  `json:"updated"`
	Invalid   int  `json:"invalid"`
	Locked    bool `json:"locked,omitempty"`
}
