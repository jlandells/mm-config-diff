package main

import (
	"strings"
)

// RedactedValue is the replacement string for sensitive fields.
const RedactedValue = "[REDACTED]"

// ExplicitRedactPaths lists the exact dot-notation paths that must always be redacted.
var ExplicitRedactPaths = map[string]bool{
	"SqlSettings.DataSource":              true,
	"SqlSettings.DataSourceReplicas":      true,
	"SqlSettings.AtRestEncryptKey":        true,
	"EmailSettings.SMTPPassword":          true,
	"FileSettings.PublicLinkSalt":         true,
	"FileSettings.AmazonS3SecretAccessKey": true,
	"GitLabSettings.Secret":               true,
	"GoogleSettings.Secret":               true,
	"Office365Settings.Secret":            true,
	"OpenIdSettings.ClientSecret":         true,
}

// CatchAllPatterns are substrings matched case-insensitively against the leaf field name.
var CatchAllPatterns = []string{"password", "secret", "salt", "key", "token"}

// RedactConfig walks a nested map and redacts sensitive fields in-place.
func RedactConfig(config map[string]interface{}) {
	redactMap(config, "")
}

func redactMap(m map[string]interface{}, prefix string) {
	for k, v := range m {
		dotPath := k
		if prefix != "" {
			dotPath = prefix + "." + k
		}

		if shouldRedact(dotPath) {
			m[k] = RedactedValue
			continue
		}

		if nested, ok := v.(map[string]interface{}); ok {
			redactMap(nested, dotPath)
		}
	}
}

// shouldRedact returns true if the given dot-notation path should be redacted.
func shouldRedact(dotPath string) bool {
	if ExplicitRedactPaths[dotPath] {
		return true
	}

	// Check the leaf field name against catch-all patterns.
	leaf := dotPath
	if idx := strings.LastIndex(dotPath, "."); idx >= 0 {
		leaf = dotPath[idx+1:]
	}
	leafLower := strings.ToLower(leaf)
	for _, pattern := range CatchAllPatterns {
		if strings.Contains(leafLower, pattern) {
			return true
		}
	}

	return false
}
