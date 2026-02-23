package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFormatDiffText_WithChanges(t *testing.T) {
	result := &DiffResult{
		Baseline: DiffSource{
			File:       "snapshot-baseline.json",
			Source:     "file",
			CapturedAt: "2025-10-01T09:00:00Z",
		},
		Compared: DiffSource{
			Source:     "live",
			ServerURL:  "https://mm.example.com",
			CapturedAt: "2025-11-01T10:00:00Z",
		},
		DriftDetected: true,
		Changed: []ChangedField{
			{Field: "ServiceSettings.MaximumLoginAttempts", Before: float64(10), After: float64(5)},
		},
		Added: []AddedField{
			{Field: "ExperimentalSettings.NewSetting", Value: "some-value"},
		},
		Removed: []RemovedField{},
	}

	output := FormatDiffText(result)

	if !strings.Contains(output, "Configuration drift detected") {
		t.Error("output should contain drift header")
	}
	if !strings.Contains(output, "CHANGED (1)") {
		t.Error("output should show CHANGED count")
	}
	if !strings.Contains(output, "ServiceSettings.MaximumLoginAttempts") {
		t.Error("output should contain changed field name")
	}
	if !strings.Contains(output, "Before : 10") {
		t.Error("output should show before value")
	}
	if !strings.Contains(output, "After  : 5") {
		t.Error("output should show after value")
	}
	if !strings.Contains(output, "ADDED (1)") {
		t.Error("output should show ADDED count")
	}
	if !strings.Contains(output, "REMOVED (0)") {
		t.Error("output should show REMOVED count")
	}
	if !strings.Contains(output, "(none)") {
		t.Error("output should show (none) for empty sections")
	}
}

func TestFormatDiffText_NoDrift(t *testing.T) {
	result := &DiffResult{
		DriftDetected: false,
		Changed:       []ChangedField{},
		Added:         []AddedField{},
		Removed:       []RemovedField{},
	}

	output := FormatDiffText(result)

	if !strings.Contains(output, "No configuration drift detected.") {
		t.Errorf("expected 'No configuration drift detected.', got %q", output)
	}
}

func TestFormatDiffJSON_ValidJSON(t *testing.T) {
	result := &DiffResult{
		Baseline: DiffSource{
			File:       "baseline.json",
			CapturedAt: "2025-10-01T09:00:00Z",
		},
		Compared: DiffSource{
			Source:     "live",
			ServerURL:  "https://mm.example.com",
			CapturedAt: "2025-11-01T10:00:00Z",
		},
		DriftDetected: true,
		Changed: []ChangedField{
			{Field: "ServiceSettings.MaximumLoginAttempts", Before: float64(10), After: float64(5)},
		},
		Added:   []AddedField{},
		Removed: []RemovedField{},
	}

	output, err := FormatDiffJSON(result)
	if err != nil {
		t.Fatalf("FormatDiffJSON failed: %v", err)
	}

	// Verify it's valid JSON.
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}

	if parsed["drift_detected"] != true {
		t.Error("drift_detected should be true in JSON output")
	}

	changed, ok := parsed["changed"].([]interface{})
	if !ok {
		t.Fatal("changed should be an array")
	}
	if len(changed) != 1 {
		t.Errorf("expected 1 changed item, got %d", len(changed))
	}
}

func TestFormatDiffJSON_NoDrift(t *testing.T) {
	result := &DiffResult{
		DriftDetected: false,
		Changed:       []ChangedField{},
		Added:         []AddedField{},
		Removed:       []RemovedField{},
	}

	output, err := FormatDiffJSON(result)
	if err != nil {
		t.Fatalf("FormatDiffJSON failed: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal([]byte(output), &parsed)

	if parsed["drift_detected"] != false {
		t.Error("drift_detected should be false")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name  string
		input interface{}
		want  string
	}{
		{"nil", nil, "null"},
		{"string", "hello", `"hello"`},
		{"integer float", float64(42), "42"},
		{"decimal float", float64(3.14), "3.14"},
		{"true", true, "true"},
		{"false", false, "false"},
		{"string array", []interface{}{"a", "b"}, `["a","b"]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatValue(tt.input)
			if got != tt.want {
				t.Errorf("FormatValue(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWriteOutput_ToFile(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.txt")

	err := WriteOutput("test content", outputPath)
	if err != nil {
		t.Fatalf("WriteOutput failed: %v", err)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(data) != "test content" {
		t.Errorf("file content = %q, want %q", string(data), "test content")
	}
}

func TestWriteOutput_ToStdout(t *testing.T) {
	// Writing to stdout (empty path) should not error.
	err := WriteOutput("", "")
	if err != nil {
		t.Fatalf("WriteOutput to stdout failed: %v", err)
	}
}
