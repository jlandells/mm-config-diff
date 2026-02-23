package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"golang.org/x/term"
)

// MattermostClient defines the interface for interacting with the Mattermost API.
type MattermostClient interface {
	GetConfig(ctx context.Context) (map[string]interface{}, error)
	ServerURL() string
}

// LiveClient wraps model.Client4 and implements MattermostClient.
type LiveClient struct {
	client    *model.Client4
	serverURL string
}

// NewLiveClient creates a new LiveClient, authenticating with the provided credentials.
func NewLiveClient(ctx context.Context, serverURL, token, username string, verbose bool) (*LiveClient, error) {
	serverURL = strings.TrimRight(serverURL, "/")

	client := model.NewAPIv4Client(serverURL)

	if token != "" {
		client.SetToken(token)
		if verbose {
			fmt.Fprintln(os.Stderr, "Authenticating with personal access token...")
		}
	} else if username != "" {
		password, err := readPassword()
		if err != nil {
			return nil, NewExitError(ExitConfigError, "error: failed to read password", err)
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "Authenticating as %s...\n", username)
		}
		_, resp, err := client.Login(ctx, username, password)
		if err != nil {
			if resp != nil {
				return nil, ClassifyAPIError(resp.StatusCode, serverURL, err)
			}
			return nil, ClassifyAPIError(0, serverURL, err)
		}
	} else {
		return nil, NewExitError(ExitConfigError,
			"error: authentication required. Use --token (or MM_TOKEN) for token auth, or --username (or MM_USERNAME) for password auth.", nil)
	}

	return &LiveClient{client: client, serverURL: serverURL}, nil
}

// GetConfig retrieves the server configuration as a generic map.
func (c *LiveClient) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	cfg, resp, err := c.client.GetConfig(ctx)
	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		return nil, ClassifyAPIError(statusCode, c.serverURL, err)
	}

	// Convert *model.Config to map[string]interface{} via JSON round-trip.
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, NewExitError(ExitAPIError, "error: failed to marshal config", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, NewExitError(ExitAPIError, "error: failed to unmarshal config", err)
	}

	return result, nil
}

// ServerURL returns the server URL.
func (c *LiveClient) ServerURL() string {
	return c.serverURL
}

// readPassword obtains the password from an interactive prompt or environment variable.
func readPassword() (string, error) {
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprint(os.Stderr, "Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr) // move to next line
		if err != nil {
			return "", fmt.Errorf("failed to read password from terminal: %w", err)
		}
		return string(passwordBytes), nil
	}

	password := os.Getenv("MM_PASSWORD")
	if password == "" {
		return "", fmt.Errorf("no interactive terminal available and MM_PASSWORD is not set")
	}
	return password, nil
}

// flagOrEnv returns the flag value if non-empty, otherwise the environment variable value.
func flagOrEnv(flagVal, envVar string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv(envVar)
}
