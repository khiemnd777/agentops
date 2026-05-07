package main

import (
	"reflect"
	"testing"
)

func TestNormalizedMode(t *testing.T) {
	tests := map[string]string{
		"":             "REPO_TO_DB",
		"files-to-db":  "REPO_TO_DB",
		"repo-to-db":   "REPO_TO_DB",
		"db-to-files":  "DB_TO_REPO",
		"db-to-repo":   "DB_TO_REPO",
		"compare-only": "COMPARE_ONLY",
		"reconcile":    "COMPARE_ONLY",
	}
	for input, want := range tests {
		if got := normalizedMode(input); got != want {
			t.Fatalf("normalizedMode(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV("AGENTS.md, .codex/skills/backend/SKILL.md,")
	want := []string{"AGENTS.md", ".codex/skills/backend/SKILL.md"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitCSV returned %#v, want %#v", got, want)
	}
}
