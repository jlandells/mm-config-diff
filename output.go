package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// FormatDiffText produces human-readable text output for a diff result.
func FormatDiffText(result *DiffResult) string {
	var sb strings.Builder

	if !result.DriftDetected {
		sb.WriteString("No configuration drift detected.\n")
		return sb.String()
	}

	sb.WriteString("Configuration drift detected between:\n")
	sb.WriteString(fmt.Sprintf("  Baseline : %s", formatSource(result.Baseline)))
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("  Compared : %s", formatSource(result.Compared)))
	sb.WriteString("\n\n")

	// Changed
	sb.WriteString(fmt.Sprintf("CHANGED (%d):\n", len(result.Changed)))
	if len(result.Changed) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, c := range result.Changed {
			sb.WriteString(fmt.Sprintf("  %s\n", c.Field))
			sb.WriteString(fmt.Sprintf("    Before : %s\n", FormatValue(c.Before)))
			sb.WriteString(fmt.Sprintf("    After  : %s\n", FormatValue(c.After)))
			sb.WriteString("\n")
		}
	}

	// Added
	sb.WriteString(fmt.Sprintf("ADDED (%d):\n", len(result.Added)))
	if len(result.Added) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, a := range result.Added {
			sb.WriteString(fmt.Sprintf("  %s : %s\n", a.Field, FormatValue(a.Value)))
		}
	}

	sb.WriteString("\n")

	// Removed
	sb.WriteString(fmt.Sprintf("REMOVED (%d):\n", len(result.Removed)))
	if len(result.Removed) == 0 {
		sb.WriteString("  (none)\n")
	} else {
		for _, r := range result.Removed {
			sb.WriteString(fmt.Sprintf("  %s : %s\n", r.Field, FormatValue(r.Value)))
		}
	}

	return sb.String()
}

// FormatDiffJSON produces JSON output for a diff result.
func FormatDiffJSON(result *DiffResult) (string, error) {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return "", NewExitError(ExitOutputError, "error: failed to marshal diff result to JSON", err)
	}
	return string(data), nil
}

// WriteOutput writes content to the specified file path, or to stdout if path is empty.
// Falls back to stdout with a stderr warning if file write fails.
func WriteOutput(content, outputPath string) error {
	if outputPath == "" {
		fmt.Print(content)
		return nil
	}

	if err := os.WriteFile(outputPath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "warning: unable to write to %s, writing to stdout instead: %v\n", outputPath, err)
		fmt.Print(content)
		return nil
	}
	return nil
}

// FormatValue formats a value for text display.
func FormatValue(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case float64:
		// Display integers without decimal places.
		if val == math.Trunc(val) && !math.IsInf(val, 0) {
			return fmt.Sprintf("%d", int64(val))
		}
		return fmt.Sprintf("%g", val)
	case bool:
		return fmt.Sprintf("%t", val)
	case string:
		return fmt.Sprintf("%q", val)
	case []interface{}:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	case map[string]interface{}:
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(data)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func formatSource(src DiffSource) string {
	if src.File != "" {
		s := src.File
		if src.CapturedAt != "" {
			s += fmt.Sprintf(" (captured %s)", formatTimestamp(src.CapturedAt))
		}
		return s
	}
	if src.ServerURL != "" {
		s := fmt.Sprintf("live instance at %s", src.ServerURL)
		if src.CapturedAt != "" {
			s += fmt.Sprintf(" (captured %s)", formatTimestamp(src.CapturedAt))
		}
		return s
	}
	return "unknown"
}

func formatTimestamp(iso string) string {
	// Keep it simple: return as-is since it's already ISO 8601
	return iso
}
