package main

import (
	"encoding/json"
	"testing"
)

func TestFlattenConfig(t *testing.T) {
	config := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL":        "https://mm.example.com",
			"ListenAddress":  ":8065",
			"MaxLoginAttempts": float64(10),
		},
		"SqlSettings": map[string]interface{}{
			"DriverName": "postgres",
		},
	}

	flat := FlattenConfig(config, "")

	expected := map[string]interface{}{
		"ServiceSettings.SiteURL":           "https://mm.example.com",
		"ServiceSettings.ListenAddress":     ":8065",
		"ServiceSettings.MaxLoginAttempts":  float64(10),
		"SqlSettings.DriverName":            "postgres",
	}

	if len(flat) != len(expected) {
		t.Errorf("FlattenConfig returned %d keys, want %d", len(flat), len(expected))
	}

	for k, want := range expected {
		got, ok := flat[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if got != want {
			t.Errorf("flat[%q] = %v, want %v", k, got, want)
		}
	}
}

func TestFlattenConfig_NestedMaps(t *testing.T) {
	config := map[string]interface{}{
		"PluginSettings": map[string]interface{}{
			"Plugins": map[string]interface{}{
				"com.mattermost.nps": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}

	flat := FlattenConfig(config, "")

	if val, ok := flat["PluginSettings.Plugins.com.mattermost.nps.enabled"]; !ok || val != true {
		t.Errorf("deeply nested key not flattened correctly, got %v", val)
	}
}

func TestFlattenConfig_Arrays(t *testing.T) {
	config := map[string]interface{}{
		"Settings": map[string]interface{}{
			"AllowedDomains": []interface{}{"example.com", "test.org"},
		},
	}

	flat := FlattenConfig(config, "")

	val, ok := flat["Settings.AllowedDomains"]
	if !ok {
		t.Fatal("Settings.AllowedDomains not found in flattened config")
	}

	arr, ok := val.([]interface{})
	if !ok {
		t.Fatalf("Settings.AllowedDomains should be []interface{}, got %T", val)
	}
	if len(arr) != 2 {
		t.Errorf("expected 2 elements, got %d", len(arr))
	}
}

func TestStripMetadata(t *testing.T) {
	config := map[string]interface{}{
		"_metadata": map[string]interface{}{
			"tool": "mm-config-diff",
		},
		"ServiceSettings": map[string]interface{}{
			"SiteURL": "https://mm.example.com",
		},
	}

	stripped := StripMetadata(config)

	if _, ok := stripped["_metadata"]; ok {
		t.Error("StripMetadata should remove _metadata")
	}
	if _, ok := stripped["ServiceSettings"]; !ok {
		t.Error("StripMetadata should preserve other keys")
	}
	// Original should be unmodified.
	if _, ok := config["_metadata"]; !ok {
		t.Error("StripMetadata should not modify the original map")
	}
}

func TestCompareConfigs_ChangedFields(t *testing.T) {
	baseline := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"MaximumLoginAttempts": float64(10),
			"EnableOAuthServiceProvider": true,
		},
	}
	target := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"MaximumLoginAttempts": float64(5),
			"EnableOAuthServiceProvider": true,
		},
	}

	result := CompareConfigs(baseline, target, nil)

	if len(result.Changed) != 1 {
		t.Fatalf("expected 1 changed field, got %d", len(result.Changed))
	}
	if result.Changed[0].Field != "ServiceSettings.MaximumLoginAttempts" {
		t.Errorf("changed field = %q, want %q", result.Changed[0].Field, "ServiceSettings.MaximumLoginAttempts")
	}
	if result.Changed[0].Before != float64(10) || result.Changed[0].After != float64(5) {
		t.Errorf("before/after = %v/%v, want 10/5", result.Changed[0].Before, result.Changed[0].After)
	}
	if !result.DriftDetected {
		t.Error("DriftDetected should be true")
	}
}

func TestCompareConfigs_AddedFields(t *testing.T) {
	baseline := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL": "https://mm.example.com",
		},
	}
	target := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL": "https://mm.example.com",
		},
		"ExperimentalSettings": map[string]interface{}{
			"NewFeature": true,
		},
	}

	result := CompareConfigs(baseline, target, nil)

	if len(result.Added) != 1 {
		t.Fatalf("expected 1 added field, got %d", len(result.Added))
	}
	if result.Added[0].Field != "ExperimentalSettings.NewFeature" {
		t.Errorf("added field = %q, want %q", result.Added[0].Field, "ExperimentalSettings.NewFeature")
	}
}

func TestCompareConfigs_RemovedFields(t *testing.T) {
	baseline := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL":       "https://mm.example.com",
			"OldDeprecated": "some-value",
		},
	}
	target := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL": "https://mm.example.com",
		},
	}

	result := CompareConfigs(baseline, target, nil)

	if len(result.Removed) != 1 {
		t.Fatalf("expected 1 removed field, got %d", len(result.Removed))
	}
	if result.Removed[0].Field != "ServiceSettings.OldDeprecated" {
		t.Errorf("removed field = %q, want %q", result.Removed[0].Field, "ServiceSettings.OldDeprecated")
	}
}

func TestCompareConfigs_NoDrift(t *testing.T) {
	config := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL":      "https://mm.example.com",
			"ListenAddress": ":8065",
		},
	}

	result := CompareConfigs(config, config, nil)

	if result.DriftDetected {
		t.Error("DriftDetected should be false when configs are identical")
	}
	if len(result.Changed) != 0 || len(result.Added) != 0 || len(result.Removed) != 0 {
		t.Errorf("expected empty diff, got %d changed, %d added, %d removed",
			len(result.Changed), len(result.Added), len(result.Removed))
	}
}

func TestCompareConfigs_IgnoreFields(t *testing.T) {
	baseline := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL":              "https://old.example.com",
			"MaximumLoginAttempts": float64(10),
		},
	}
	target := map[string]interface{}{
		"ServiceSettings": map[string]interface{}{
			"SiteURL":              "https://new.example.com",
			"MaximumLoginAttempts": float64(5),
		},
	}

	ignoreFields := map[string]bool{
		"ServiceSettings.SiteURL": true,
	}

	result := CompareConfigs(baseline, target, ignoreFields)

	if len(result.Changed) != 1 {
		t.Fatalf("expected 1 changed field (SiteURL ignored), got %d", len(result.Changed))
	}
	if result.Changed[0].Field != "ServiceSettings.MaximumLoginAttempts" {
		t.Errorf("changed field = %q, want MaximumLoginAttempts", result.Changed[0].Field)
	}
}

func TestCompareConfigs_StripsMetadata(t *testing.T) {
	baseline := map[string]interface{}{
		"_metadata": map[string]interface{}{
			"tool":         "mm-config-diff",
			"tool_version": "1.0.0",
			"captured_at":  "2025-10-01T09:00:00Z",
		},
		"ServiceSettings": map[string]interface{}{
			"SiteURL": "https://mm.example.com",
		},
	}
	target := map[string]interface{}{
		"_metadata": map[string]interface{}{
			"tool":         "mm-config-diff",
			"tool_version": "1.0.0",
			"captured_at":  "2025-11-01T09:00:00Z",
		},
		"ServiceSettings": map[string]interface{}{
			"SiteURL": "https://mm.example.com",
		},
	}

	result := CompareConfigs(baseline, target, nil)

	if result.DriftDetected {
		t.Error("metadata differences should not cause drift detection")
	}
}

func TestCompareConfigs_SortedOutput(t *testing.T) {
	baseline := map[string]interface{}{
		"Zebra": map[string]interface{}{"Field": "a"},
		"Alpha": map[string]interface{}{"Field": "a"},
		"Middle": map[string]interface{}{"Field": "a"},
	}
	target := map[string]interface{}{
		"Zebra": map[string]interface{}{"Field": "b"},
		"Alpha": map[string]interface{}{"Field": "b"},
		"Middle": map[string]interface{}{"Field": "b"},
	}

	result := CompareConfigs(baseline, target, nil)

	if len(result.Changed) != 3 {
		t.Fatalf("expected 3 changed, got %d", len(result.Changed))
	}
	if result.Changed[0].Field != "Alpha.Field" {
		t.Errorf("first changed should be Alpha.Field, got %s", result.Changed[0].Field)
	}
	if result.Changed[1].Field != "Middle.Field" {
		t.Errorf("second changed should be Middle.Field, got %s", result.Changed[1].Field)
	}
	if result.Changed[2].Field != "Zebra.Field" {
		t.Errorf("third changed should be Zebra.Field, got %s", result.Changed[2].Field)
	}
}

func TestValuesEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b interface{}
		want bool
	}{
		{"equal strings", "hello", "hello", true},
		{"different strings", "hello", "world", false},
		{"equal numbers", float64(42), float64(42), true},
		{"different numbers", float64(42), float64(43), false},
		{"equal bools", true, true, true},
		{"different bools", true, false, false},
		{"both nil", nil, nil, true},
		{"one nil", nil, "value", false},
		{"equal arrays", []interface{}{"a", "b"}, []interface{}{"a", "b"}, true},
		{"different arrays", []interface{}{"a", "b"}, []interface{}{"a", "c"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := valuesEqual(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("valuesEqual(%v, %v) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseIgnoreFields(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]bool
	}{
		{"empty string", "", map[string]bool{}},
		{"single field", "ServiceSettings.SiteURL", map[string]bool{"ServiceSettings.SiteURL": true}},
		{"multiple fields", "ServiceSettings.SiteURL,MetricsSettings.Enable", map[string]bool{
			"ServiceSettings.SiteURL": true,
			"MetricsSettings.Enable":  true,
		}},
		{"with spaces", " ServiceSettings.SiteURL , MetricsSettings.Enable ", map[string]bool{
			"ServiceSettings.SiteURL": true,
			"MetricsSettings.Enable":  true,
		}},
		{"trailing comma", "ServiceSettings.SiteURL,", map[string]bool{
			"ServiceSettings.SiteURL": true,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseIgnoreFields(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseIgnoreFields(%q) returned %d fields, want %d", tt.input, len(got), len(tt.want))
			}
			for k := range tt.want {
				if !got[k] {
					t.Errorf("missing field %q", k)
				}
			}
		})
	}
}

func TestCompareConfigs_JSONRoundTrip(t *testing.T) {
	// Simulate configs that have been through JSON marshal/unmarshal.
	baseJSON := `{
		"ServiceSettings": {
			"SiteURL": "https://mm.example.com",
			"MaximumLoginAttempts": 10,
			"EnableOpenServer": true
		}
	}`
	targetJSON := `{
		"ServiceSettings": {
			"SiteURL": "https://mm.example.com",
			"MaximumLoginAttempts": 5,
			"EnableOpenServer": true
		}
	}`

	var baseline, target map[string]interface{}
	json.Unmarshal([]byte(baseJSON), &baseline)
	json.Unmarshal([]byte(targetJSON), &target)

	result := CompareConfigs(baseline, target, nil)

	if len(result.Changed) != 1 {
		t.Fatalf("expected 1 changed, got %d", len(result.Changed))
	}
	if result.Changed[0].Field != "ServiceSettings.MaximumLoginAttempts" {
		t.Errorf("changed field = %q", result.Changed[0].Field)
	}
}
