package main

import (
	"testing"
)

func TestRedactConfig_ExplicitPaths(t *testing.T) {
	config := map[string]interface{}{
		"SqlSettings": map[string]interface{}{
			"DataSource":         "postgres://user:pass@localhost/mattermost?sslmode=disable",
			"DataSourceReplicas": []interface{}{"postgres://replica1:pass@localhost/mm"},
			"AtRestEncryptKey":   "s3cr3tK3y12345678901234567890123",
			"DriverName":         "postgres",
		},
		"EmailSettings": map[string]interface{}{
			"SMTPPassword":        "smtp-secret-password",
			"EnableSignInWithEmail": true,
		},
		"FileSettings": map[string]interface{}{
			"PublicLinkSalt":          "random-salt-value-here-32chars!!",
			"AmazonS3SecretAccessKey": "AKIAIOSFODNN7EXAMPLE",
			"MaxFileSize":             104857600,
		},
		"GitLabSettings": map[string]interface{}{
			"Secret": "gitlab-oauth-secret",
			"Enable": false,
		},
		"GoogleSettings": map[string]interface{}{
			"Secret": "google-oauth-secret",
			"Enable": false,
		},
		"Office365Settings": map[string]interface{}{
			"Secret": "office365-oauth-secret",
			"Enable": false,
		},
		"OpenIdSettings": map[string]interface{}{
			"ClientSecret": "openid-client-secret",
			"Enable":       false,
		},
	}

	RedactConfig(config)

	// Verify all explicit paths are redacted.
	expectations := []struct {
		section string
		field   string
	}{
		{"SqlSettings", "DataSource"},
		{"SqlSettings", "DataSourceReplicas"},
		{"SqlSettings", "AtRestEncryptKey"},
		{"EmailSettings", "SMTPPassword"},
		{"FileSettings", "PublicLinkSalt"},
		{"FileSettings", "AmazonS3SecretAccessKey"},
		{"GitLabSettings", "Secret"},
		{"GoogleSettings", "Secret"},
		{"Office365Settings", "Secret"},
		{"OpenIdSettings", "ClientSecret"},
	}

	for _, exp := range expectations {
		section, ok := config[exp.section].(map[string]interface{})
		if !ok {
			t.Errorf("section %s not found or not a map", exp.section)
			continue
		}
		val := section[exp.field]
		if val != RedactedValue {
			t.Errorf("%s.%s = %v, want %q", exp.section, exp.field, val, RedactedValue)
		}
	}

	// Verify non-sensitive fields are NOT redacted.
	sqlSettings := config["SqlSettings"].(map[string]interface{})
	if sqlSettings["DriverName"] != "postgres" {
		t.Errorf("SqlSettings.DriverName should not be redacted, got %v", sqlSettings["DriverName"])
	}

	emailSettings := config["EmailSettings"].(map[string]interface{})
	if emailSettings["EnableSignInWithEmail"] != true {
		t.Errorf("EmailSettings.EnableSignInWithEmail should not be redacted, got %v", emailSettings["EnableSignInWithEmail"])
	}

	fileSettings := config["FileSettings"].(map[string]interface{})
	if fileSettings["MaxFileSize"] != 104857600 {
		t.Errorf("FileSettings.MaxFileSize should not be redacted, got %v", fileSettings["MaxFileSize"])
	}
}

func TestRedactConfig_CatchAllPatterns(t *testing.T) {
	config := map[string]interface{}{
		"SomePlugin": map[string]interface{}{
			"AccessToken":     "should-be-redacted",
			"ApiKey":          "should-be-redacted",
			"DatabasePassword": "should-be-redacted",
			"EncryptionSalt":  "should-be-redacted",
			"ClientSecret":    "should-be-redacted",
			"DisplayName":     "should-not-be-redacted",
			"Enable":          true,
		},
	}

	RedactConfig(config)

	section := config["SomePlugin"].(map[string]interface{})

	redacted := []string{"AccessToken", "ApiKey", "DatabasePassword", "EncryptionSalt", "ClientSecret"}
	for _, field := range redacted {
		if section[field] != RedactedValue {
			t.Errorf("SomePlugin.%s = %v, want %q (catch-all should redact)", field, section[field], RedactedValue)
		}
	}

	if section["DisplayName"] != "should-not-be-redacted" {
		t.Errorf("SomePlugin.DisplayName should not be redacted, got %v", section["DisplayName"])
	}
	if section["Enable"] != true {
		t.Errorf("SomePlugin.Enable should not be redacted, got %v", section["Enable"])
	}
}

func TestRedactConfig_CaseInsensitive(t *testing.T) {
	config := map[string]interface{}{
		"Settings": map[string]interface{}{
			"MYPASSWORD":  "upper-case",
			"mysecret":    "lower-case",
			"SomeToken":   "mixed-case",
			"aKEYvalue":   "embedded-key",
			"MySALTField": "embedded-salt",
		},
	}

	RedactConfig(config)

	section := config["Settings"].(map[string]interface{})
	for field, val := range section {
		if val != RedactedValue {
			t.Errorf("Settings.%s = %v, want %q (case-insensitive catch-all)", field, val, RedactedValue)
		}
	}
}

func TestShouldRedact(t *testing.T) {
	tests := []struct {
		dotPath string
		want    bool
	}{
		{"SqlSettings.DataSource", true},
		{"SqlSettings.DriverName", false},
		{"EmailSettings.SMTPPassword", true},
		{"SomePlugin.AccessToken", true},
		{"SomePlugin.ApiKey", true},
		{"SomePlugin.DisplayName", false},
		{"ServiceSettings.SiteURL", false},
		{"ServiceSettings.ListenAddress", false},
		{"CustomSection.MySecretValue", true},
	}
	for _, tt := range tests {
		t.Run(tt.dotPath, func(t *testing.T) {
			got := shouldRedact(tt.dotPath)
			if got != tt.want {
				t.Errorf("shouldRedact(%q) = %v, want %v", tt.dotPath, got, tt.want)
			}
		})
	}
}

func TestRedactConfig_ArrayField(t *testing.T) {
	config := map[string]interface{}{
		"SqlSettings": map[string]interface{}{
			"DataSourceReplicas": []interface{}{
				"postgres://replica1:pass@host1/mm",
				"postgres://replica2:pass@host2/mm",
			},
		},
	}

	RedactConfig(config)

	section := config["SqlSettings"].(map[string]interface{})
	if section["DataSourceReplicas"] != RedactedValue {
		t.Errorf("SqlSettings.DataSourceReplicas = %v, want %q", section["DataSourceReplicas"], RedactedValue)
	}
}
