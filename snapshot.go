package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SnapshotMetadata holds the metadata embedded in a snapshot file.
type SnapshotMetadata struct {
	Tool        string `json:"tool"`
	ToolVersion string `json:"tool_version"`
	ServerURL   string `json:"server_url"`
	CapturedAt  string `json:"captured_at"`
}

// TakeSnapshot fetches the config from the API, redacts sensitive fields,
// and injects metadata.
func TakeSnapshot(ctx context.Context, client MattermostClient, version string) (map[string]interface{}, error) {
	config, err := client.GetConfig(ctx)
	if err != nil {
		return nil, err
	}

	RedactConfig(config)

	metadata := SnapshotMetadata{
		Tool:        "mm-config-diff",
		ToolVersion: version,
		ServerURL:   client.ServerURL(),
		CapturedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	// Convert metadata struct to map for injection.
	metaData, _ := json.Marshal(metadata)
	var metaMap map[string]interface{}
	json.Unmarshal(metaData, &metaMap)

	config["_metadata"] = metaMap

	return config, nil
}

// WriteSnapshot writes the snapshot map to a JSON file and returns the absolute path.
func WriteSnapshot(snapshot map[string]interface{}, outputPath string) (string, error) {
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return "", NewExitError(ExitOutputError, "error: failed to marshal snapshot to JSON", err)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		return "", NewExitError(ExitOutputError, fmt.Sprintf("error: unable to write snapshot to %s", outputPath), err)
	}

	absPath, err := filepath.Abs(outputPath)
	if err != nil {
		return outputPath, nil
	}
	return absPath, nil
}

// LoadSnapshot reads a snapshot file, validates its metadata, and returns
// the config map along with the parsed metadata.
func LoadSnapshot(filePath string) (map[string]interface{}, *SnapshotMetadata, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, NewExitError(ExitConfigError, fmt.Sprintf("error: unable to read snapshot file %s", filePath), err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, nil, NewExitError(ExitConfigError, fmt.Sprintf("error: snapshot file %s is not valid JSON", filePath), err)
	}

	metaRaw, ok := config["_metadata"]
	if !ok {
		return nil, nil, NewExitError(ExitConfigError, fmt.Sprintf("error: snapshot file %s is missing _metadata. Is this a valid mm-config-diff snapshot?", filePath), nil)
	}

	metaMap, ok := metaRaw.(map[string]interface{})
	if !ok {
		return nil, nil, NewExitError(ExitConfigError, fmt.Sprintf("error: snapshot file %s has invalid _metadata format", filePath), nil)
	}

	toolName, _ := metaMap["tool"].(string)
	if toolName != "mm-config-diff" {
		return nil, nil, NewExitError(ExitConfigError, fmt.Sprintf("error: snapshot file %s was not created by mm-config-diff (tool: %q)", filePath, toolName), nil)
	}

	metadata := &SnapshotMetadata{
		Tool:        toolName,
		ToolVersion: stringFromMap(metaMap, "tool_version"),
		ServerURL:   stringFromMap(metaMap, "server_url"),
		CapturedAt:  stringFromMap(metaMap, "captured_at"),
	}

	return config, metadata, nil
}

// DefaultSnapshotFilename generates a default filename based on the current timestamp.
func DefaultSnapshotFilename() string {
	ts := time.Now().UTC().Format(time.RFC3339)
	ts = strings.ReplaceAll(ts, ":", "-")
	return fmt.Sprintf("mm-config-snapshot-%s.json", ts)
}

func stringFromMap(m map[string]interface{}, key string) string {
	v, _ := m[key].(string)
	return v
}
