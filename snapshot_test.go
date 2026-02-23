package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTakeSnapshot(t *testing.T) {
	mockConfig := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL":       "https://mm.example.com",
			"ListenAddress": ":8065",
		},
		"SqlSettings": map[string]interface{}{
			"DriverName": "postgres",
			"DataSource": "postgres://user:pass@localhost/mattermost",
		},
	}

	client := &MockClient{
		config:    mockConfig,
		serverURL: "https://mm.example.com",
	}

	snapshot, err := TakeSnapshot(context.Background(), client, "1.0.0")
	if err != nil {
		t.Fatalf("TakeSnapshot failed: %v", err)
	}

	// Check metadata is present.
	metaRaw, ok := snapshot["_metadata"]
	if !ok {
		t.Fatal("snapshot missing _metadata")
	}
	meta, ok := metaRaw.(map[string]interface{})
	if !ok {
		t.Fatal("_metadata is not a map")
	}
	if meta["tool"] != "mm-config-diff" {
		t.Errorf("tool = %v, want mm-config-diff", meta["tool"])
	}
	if meta["tool_version"] != "1.0.0" {
		t.Errorf("tool_version = %v, want 1.0.0", meta["tool_version"])
	}
	if meta["server_url"] != "https://mm.example.com" {
		t.Errorf("server_url = %v, want https://mm.example.com", meta["server_url"])
	}
	if meta["captured_at"] == nil || meta["captured_at"] == "" {
		t.Error("captured_at should not be empty")
	}

	// Check sensitive fields are redacted.
	sql := snapshot["SqlSettings"].(map[string]interface{})
	if sql["DataSource"] != RedactedValue {
		t.Errorf("SqlSettings.DataSource should be redacted, got %v", sql["DataSource"])
	}
	if sql["DriverName"] != "postgres" {
		t.Errorf("SqlSettings.DriverName should NOT be redacted, got %v", sql["DriverName"])
	}
}

func TestWriteAndLoadSnapshot(t *testing.T) {
	snapshot := map[string]interface{}{
		"_metadata": map[string]interface{}{
			"tool":         "mm-config-diff",
			"tool_version": "1.0.0",
			"server_url":   "https://mm.example.com",
			"captured_at":  "2025-10-01T09:00:00Z",
		},
		"ServiceSettings": map[string]interface{}{
			"SiteURL":       "https://mm.example.com",
			"ListenAddress": ":8065",
		},
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-snapshot.json")

	absPath, err := WriteSnapshot(snapshot, outputPath)
	if err != nil {
		t.Fatalf("WriteSnapshot failed: %v", err)
	}

	if !filepath.IsAbs(absPath) {
		t.Errorf("WriteSnapshot should return absolute path, got %q", absPath)
	}

	// Load it back.
	loaded, meta, err := LoadSnapshot(outputPath)
	if err != nil {
		t.Fatalf("LoadSnapshot failed: %v", err)
	}

	if meta.Tool != "mm-config-diff" {
		t.Errorf("meta.Tool = %q, want mm-config-diff", meta.Tool)
	}
	if meta.ToolVersion != "1.0.0" {
		t.Errorf("meta.ToolVersion = %q, want 1.0.0", meta.ToolVersion)
	}
	if meta.ServerURL != "https://mm.example.com" {
		t.Errorf("meta.ServerURL = %q", meta.ServerURL)
	}
	if meta.CapturedAt != "2025-10-01T09:00:00Z" {
		t.Errorf("meta.CapturedAt = %q", meta.CapturedAt)
	}

	svc := loaded["ServiceSettings"].(map[string]interface{})
	if svc["SiteURL"] != "https://mm.example.com" {
		t.Errorf("loaded ServiceSettings.SiteURL = %v", svc["SiteURL"])
	}
}

func TestLoadSnapshot_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.json")
	os.WriteFile(badFile, []byte("not json at all"), 0644)

	_, _, err := LoadSnapshot(badFile)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("exit code = %d, want %d", exitErr.Code, ExitConfigError)
	}
}

func TestLoadSnapshot_MissingMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	noMetaFile := filepath.Join(tmpDir, "no-meta.json")
	os.WriteFile(noMetaFile, []byte(`{"ServiceSettings": {"SiteURL": "https://mm.example.com"}}`), 0644)

	_, _, err := LoadSnapshot(noMetaFile)
	if err == nil {
		t.Fatal("expected error for missing metadata")
	}
	if !strings.Contains(err.Error(), "_metadata") {
		t.Errorf("error should mention _metadata, got %q", err.Error())
	}
}

func TestLoadSnapshot_WrongTool(t *testing.T) {
	tmpDir := t.TempDir()
	wrongToolFile := filepath.Join(tmpDir, "wrong-tool.json")
	os.WriteFile(wrongToolFile, []byte(`{
		"_metadata": {"tool": "some-other-tool", "tool_version": "1.0.0"},
		"ServiceSettings": {}
	}`), 0644)

	_, _, err := LoadSnapshot(wrongToolFile)
	if err == nil {
		t.Fatal("expected error for wrong tool")
	}
	if !strings.Contains(err.Error(), "not created by mm-config-diff") {
		t.Errorf("error should mention wrong tool, got %q", err.Error())
	}
}

func TestLoadSnapshot_FileNotFound(t *testing.T) {
	_, _, err := LoadSnapshot("/nonexistent/path/snapshot.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	exitErr, ok := err.(*ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T", err)
	}
	if exitErr.Code != ExitConfigError {
		t.Errorf("exit code = %d, want %d", exitErr.Code, ExitConfigError)
	}
}

func TestDefaultSnapshotFilename(t *testing.T) {
	filename := DefaultSnapshotFilename()
	if !strings.HasPrefix(filename, "mm-config-snapshot-") {
		t.Errorf("filename should start with mm-config-snapshot-, got %q", filename)
	}
	if !strings.HasSuffix(filename, ".json") {
		t.Errorf("filename should end with .json, got %q", filename)
	}
	if strings.Contains(filename, ":") {
		t.Errorf("filename should not contain colons, got %q", filename)
	}
}

func TestLoadSnapshot_FromTestdata(t *testing.T) {
	// Test with fixture files.
	config, meta, err := LoadSnapshot("testdata/valid-snapshot.json")
	if err != nil {
		t.Fatalf("LoadSnapshot testdata/valid-snapshot.json failed: %v", err)
	}
	if meta.Tool != "mm-config-diff" {
		t.Errorf("tool = %q", meta.Tool)
	}
	if meta.CapturedAt != "2025-10-01T09:00:00Z" {
		t.Errorf("captured_at = %q", meta.CapturedAt)
	}
	svc := config["ServiceSettings"].(map[string]interface{})
	if svc["MaximumLoginAttempts"] != float64(10) {
		t.Errorf("MaximumLoginAttempts = %v", svc["MaximumLoginAttempts"])
	}

	// Wrong tool fixture.
	_, _, err = LoadSnapshot("testdata/wrong-tool.json")
	if err == nil {
		t.Error("expected error for wrong-tool.json")
	}

	// No metadata fixture.
	_, _, err = LoadSnapshot("testdata/no-metadata.json")
	if err == nil {
		t.Error("expected error for no-metadata.json")
	}
}

func TestDiffWithTestdataFixtures(t *testing.T) {
	baseline, _, err := LoadSnapshot("testdata/valid-snapshot.json")
	if err != nil {
		t.Fatalf("LoadSnapshot baseline: %v", err)
	}
	target, _, err := LoadSnapshot("testdata/valid-snapshot-2.json")
	if err != nil {
		t.Fatalf("LoadSnapshot target: %v", err)
	}

	result := CompareConfigs(baseline, target, nil)

	if !result.DriftDetected {
		t.Fatal("expected drift between fixtures")
	}
	if len(result.Changed) == 0 {
		t.Error("expected changed fields")
	}
	if len(result.Added) == 0 {
		t.Error("expected added fields")
	}

	// Verify specific changes from the fixtures.
	changedFields := make(map[string]bool)
	for _, c := range result.Changed {
		changedFields[c.Field] = true
	}
	if !changedFields["ServiceSettings.MaximumLoginAttempts"] {
		t.Error("expected MaximumLoginAttempts to be changed")
	}
	if !changedFields["PluginSettings.Enable"] {
		t.Error("expected PluginSettings.Enable to be changed")
	}

	addedFields := make(map[string]bool)
	for _, a := range result.Added {
		addedFields[a.Field] = true
	}
	if !addedFields["ExperimentalSettings.NewFeature"] {
		t.Error("expected ExperimentalSettings.NewFeature to be added")
	}
}

