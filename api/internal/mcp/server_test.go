package mcp

import (
	"encoding/json"
	"testing"
)

func TestToolsExposeV1Surface(t *testing.T) {
	got := tools()
	names := map[string]bool{}
	for _, tool := range got {
		name, _ := tool["name"].(string)
		names[name] = true
		if tool["inputSchema"] == nil {
			t.Fatalf("tool %q missing input schema", name)
		}
	}
	for _, want := range []string{
		"agentops_get_project_manifest",
		"agentops_get_sync_status",
		"agentops_preview_files_to_db",
		"agentops_sync_files_to_db",
		"agentops_submit_run_report",
	} {
		if !names[want] {
			t.Fatalf("missing tool %q in %#v", want, got)
		}
	}
}

func TestSubmitReportArgsAcceptsObjectOrJSONString(t *testing.T) {
	rawObject := json.RawMessage(`{"run_id":"run-1"}`)
	data, err := submitReportArgs{RunReport: rawObject}.reportBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"run_id":"run-1"}` {
		t.Fatalf("unexpected object report bytes: %s", data)
	}

	rawString := json.RawMessage(`"{\"run_id\":\"run-1\"}"`)
	data, err = submitReportArgs{RunReport: rawString}.reportBytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"run_id":"run-1"}` {
		t.Fatalf("unexpected string report bytes: %s", data)
	}
}
