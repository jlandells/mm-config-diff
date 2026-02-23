package main

import (
	"context"
	"os"
	"testing"
)

// MockClient implements MattermostClient for testing.
type MockClient struct {
	config    map[string]interface{}
	err       error
	serverURL string
}

func (m *MockClient) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.config, nil
}

func (m *MockClient) ServerURL() string {
	return m.serverURL
}

func TestFlagOrEnv(t *testing.T) {
	tests := []struct {
		name    string
		flagVal string
		envKey  string
		envVal  string
		want    string
	}{
		{
			name:    "flag takes precedence",
			flagVal: "from-flag",
			envKey:  "TEST_FLAG_OR_ENV_1",
			envVal:  "from-env",
			want:    "from-flag",
		},
		{
			name:    "falls back to env var",
			flagVal: "",
			envKey:  "TEST_FLAG_OR_ENV_2",
			envVal:  "from-env",
			want:    "from-env",
		},
		{
			name:    "both empty",
			flagVal: "",
			envKey:  "TEST_FLAG_OR_ENV_3",
			envVal:  "",
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVal != "" {
				os.Setenv(tt.envKey, tt.envVal)
				defer os.Unsetenv(tt.envKey)
			}
			got := flagOrEnv(tt.flagVal, tt.envKey)
			if got != tt.want {
				t.Errorf("flagOrEnv(%q, %q) = %q, want %q", tt.flagVal, tt.envKey, got, tt.want)
			}
		})
	}
}
