package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// DiffSource describes one side of a comparison.
type DiffSource struct {
	File       string `json:"file,omitempty"`
	Source     string `json:"source,omitempty"` // "file" or "live"
	ServerURL  string `json:"server_url,omitempty"`
	CapturedAt string `json:"captured_at,omitempty"`
}

// ChangedField records a field that has a different value between baseline and target.
type ChangedField struct {
	Field  string      `json:"field"`
	Before interface{} `json:"before"`
	After  interface{} `json:"after"`
}

// AddedField records a field present in the target but not the baseline.
type AddedField struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
}

// RemovedField records a field present in the baseline but not the target.
type RemovedField struct {
	Field string      `json:"field"`
	Value interface{} `json:"value"`
}

// DiffResult holds the complete comparison result.
type DiffResult struct {
	Baseline      DiffSource     `json:"baseline"`
	Compared      DiffSource     `json:"compared"`
	DriftDetected bool           `json:"drift_detected"`
	Changed       []ChangedField `json:"changed"`
	Added         []AddedField   `json:"added"`
	Removed       []RemovedField `json:"removed"`
}

// FlattenConfig recursively flattens a nested map into dot-notation keys.
// Array and non-map values are stored as-is at their dot-notation path.
func FlattenConfig(config map[string]interface{}, prefix string) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range config {
		dotPath := k
		if prefix != "" {
			dotPath = prefix + "." + k
		}

		switch val := v.(type) {
		case map[string]interface{}:
			for fk, fv := range FlattenConfig(val, dotPath) {
				result[fk] = fv
			}
		default:
			result[dotPath] = val
		}
	}
	return result
}

// StripMetadata returns a copy of the config map without the _metadata key.
func StripMetadata(config map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(config))
	for k, v := range config {
		if k == "_metadata" {
			continue
		}
		result[k] = v
	}
	return result
}

// CompareConfigs compares a baseline and target config map, returning a DiffResult.
// Both maps are stripped of metadata and flattened before comparison.
// Fields in the ignoreFields set are excluded.
func CompareConfigs(baseline, target map[string]interface{}, ignoreFields map[string]bool) *DiffResult {
	baseFlat := FlattenConfig(StripMetadata(baseline), "")
	targetFlat := FlattenConfig(StripMetadata(target), "")

	result := &DiffResult{
		Changed: []ChangedField{},
		Added:   []AddedField{},
		Removed: []RemovedField{},
	}

	// Changed and removed: iterate baseline keys
	for k, baseVal := range baseFlat {
		if ignoreFields[k] {
			continue
		}
		if targetVal, exists := targetFlat[k]; exists {
			if !valuesEqual(baseVal, targetVal) {
				result.Changed = append(result.Changed, ChangedField{
					Field:  k,
					Before: baseVal,
					After:  targetVal,
				})
			}
		} else {
			result.Removed = append(result.Removed, RemovedField{
				Field: k,
				Value: baseVal,
			})
		}
	}

	// Added: iterate target keys not in baseline
	for k, targetVal := range targetFlat {
		if ignoreFields[k] {
			continue
		}
		if _, exists := baseFlat[k]; !exists {
			result.Added = append(result.Added, AddedField{
				Field: k,
				Value: targetVal,
			})
		}
	}

	// Sort all results alphabetically by field name
	sort.Slice(result.Changed, func(i, j int) bool {
		return result.Changed[i].Field < result.Changed[j].Field
	})
	sort.Slice(result.Added, func(i, j int) bool {
		return result.Added[i].Field < result.Added[j].Field
	})
	sort.Slice(result.Removed, func(i, j int) bool {
		return result.Removed[i].Field < result.Removed[j].Field
	})

	result.DriftDetected = len(result.Changed) > 0 || len(result.Added) > 0 || len(result.Removed) > 0

	return result
}

// valuesEqual compares two values for equality.
// For complex types (slices, maps), it serialises to JSON and compares the strings.
func valuesEqual(a, b interface{}) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	// Handle numeric type differences: JSON unmarshal produces float64 for all numbers.
	// Direct comparison works for most cases, but we need reflect.DeepEqual for slices/maps.
	aKind := reflect.TypeOf(a).Kind()
	bKind := reflect.TypeOf(b).Kind()

	// Simple types: direct comparison
	if aKind != reflect.Slice && aKind != reflect.Map && bKind != reflect.Slice && bKind != reflect.Map {
		return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
	}

	// Complex types: JSON comparison
	aJSON, errA := json.Marshal(a)
	bJSON, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return reflect.DeepEqual(a, b)
	}
	return string(aJSON) == string(bJSON)
}

// ParseIgnoreFields splits a comma-separated string into a set of field names.
func ParseIgnoreFields(raw string) map[string]bool {
	result := make(map[string]bool)
	if raw == "" {
		return result
	}
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)
		if field != "" {
			result[field] = true
		}
	}
	return result
}
