package mcp

func tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "agentops_get_project_manifest",
			"description": "Return the AgentOps project manifest and Codex-native managed path contract.",
			"inputSchema": projectSchema(),
		},
		{
			"name":        "agentops_get_sync_status",
			"description": "Return recent sync runs for an AgentOps project.",
			"inputSchema": projectSchema(),
		},
		{
			"name":        "agentops_preview_files_to_db",
			"description": "Preview importing project repo Codex files into AgentOps DB as draft versions.",
			"inputSchema": syncSchema(),
		},
		{
			"name":        "agentops_sync_files_to_db",
			"description": "Import project repo Codex files into AgentOps DB as draft versions.",
			"inputSchema": syncSchema(),
		},
		{
			"name":        "agentops_submit_run_report",
			"description": "Submit a structured Codex run report for validation, import, audit, and review.",
			"inputSchema": reportSchema(),
		},
	}
}

func projectSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"project_id":   map[string]string{"type": "string"},
			"project_slug": map[string]string{"type": "string"},
			"project":      map[string]string{"type": "string"},
			"repo_path":    map[string]string{"type": "string"},
		},
	}
}

func syncSchema() map[string]any {
	schema := projectSchema()
	schema["properties"].(map[string]any)["paths"] = map[string]any{
		"type":  "array",
		"items": map[string]string{"type": "string"},
	}
	return schema
}

func reportSchema() map[string]any {
	schema := projectSchema()
	props := schema["properties"].(map[string]any)
	props["source_path"] = map[string]string{"type": "string"}
	props["run_report"] = map[string]any{"description": "Run report JSON object or JSON string"}
	schema["required"] = []string{"run_report"}
	return schema
}
