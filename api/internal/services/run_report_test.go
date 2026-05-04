package services

import (
	"os"
	"testing"
)

func TestRunReportValidatorRejectsUnsafeSecret(t *testing.T) {
	data := []byte(`{"schema_version":"1.0","run_id":"x","project_slug":"demo","task":{"title":"t"},"workflow":{"expected_id":"w","expected_version":"1"},"nodes":[{"node_key":"n","title":"N","type":"skill","status":"completed"}],"change_summary":["api_key = abcdefghijklmnopqrstuvwxyz"]}`)
	_, err := (RunReportValidator{}).ParseAndValidate(data, "demo")
	if err == nil {
		t.Fatal("expected unsafe secret error")
	}
	if validationStatus(err) != "unsafe_secret_found" {
		t.Fatalf("unexpected status: %s", validationStatus(err))
	}
}

func TestRunReportValidatorAcceptsFixture(t *testing.T) {
	data, err := os.ReadFile("../../../../fixtures/run_reports/fullstack-feature-success.report.json")
	if err != nil {
		t.Skip(err)
	}
	if _, err := (RunReportValidator{}).ParseAndValidate(data, "demo"); err != nil {
		t.Fatalf("expected fixture to validate: %v", err)
	}
}
