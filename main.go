package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	var (
		urlFlag      string
		tokenFlag    string
		usernameFlag string
		verbose      bool
	)

	rootCmd := &cobra.Command{
		Use:   "mm-config-diff",
		Short: "Mattermost Configuration Drift Detector",
		Long:  "Detects configuration drift by capturing and comparing Mattermost instance configurations.",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Resolve flags from environment variables.
			urlFlag = flagOrEnv(urlFlag, "MM_URL")
			tokenFlag = flagOrEnv(tokenFlag, "MM_TOKEN")
			usernameFlag = flagOrEnv(usernameFlag, "MM_USERNAME")
		},
	}

	rootCmd.PersistentFlags().StringVar(&urlFlag, "url", "", "Mattermost server URL (env: MM_URL)")
	rootCmd.PersistentFlags().StringVar(&tokenFlag, "token", "", "Personal access token (env: MM_TOKEN)")
	rootCmd.PersistentFlags().StringVar(&usernameFlag, "username", "", "Username for password auth (env: MM_USERNAME)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging to stderr")

	rootCmd.Version = version
	rootCmd.SetVersionTemplate("mm-config-diff version {{.Version}}\n")

	// --- Snapshot subcommand ---
	var snapshotOutput string

	snapshotCmd := &cobra.Command{
		Use:   "snapshot",
		Short: "Capture a point-in-time configuration snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if urlFlag == "" {
				return &ExitError{Code: ExitConfigError, Message: "error: server URL is required. Use --url or set the MM_URL environment variable."}
			}

			ctx := context.Background()
			client, err := NewLiveClient(ctx, urlFlag, tokenFlag, usernameFlag, verbose)
			if err != nil {
				return err
			}

			snapshot, err := TakeSnapshot(ctx, client, version)
			if err != nil {
				return err
			}

			outputPath := snapshotOutput
			if outputPath == "" {
				outputPath = DefaultSnapshotFilename()
			}

			absPath, err := WriteSnapshot(snapshot, outputPath)
			if err != nil {
				return err
			}

			fmt.Println(absPath)
			return nil
		},
	}

	snapshotCmd.Flags().StringVar(&snapshotOutput, "output", "", "Output file path (default: mm-config-snapshot-{TIMESTAMP}.json)")
	rootCmd.AddCommand(snapshotCmd)

	// --- Diff subcommand ---
	var (
		diffBaseline     string
		diffAgainst      string
		diffIgnoreFields string
		diffFormat       string
		diffOutput       string
	)

	diffCmd := &cobra.Command{
		Use:   "diff",
		Short: "Compare configurations and report drift",
		RunE: func(cmd *cobra.Command, args []string) error {
			if diffBaseline == "" {
				return &ExitError{Code: ExitConfigError, Message: "error: --baseline is required."}
			}

			baselineConfig, baselineMeta, err := LoadSnapshot(diffBaseline)
			if err != nil {
				return err
			}

			var targetConfig map[string]interface{}
			var comparedSource DiffSource

			if diffAgainst != "" {
				// Two-file comparison — no API needed.
				var targetMeta *SnapshotMetadata
				targetConfig, targetMeta, err = LoadSnapshot(diffAgainst)
				if err != nil {
					return err
				}
				comparedSource = DiffSource{
					File:       filepath.Base(diffAgainst),
					Source:     "file",
					CapturedAt: targetMeta.CapturedAt,
				}
			} else {
				// Live comparison — requires API.
				if urlFlag == "" {
					return &ExitError{Code: ExitConfigError, Message: "error: server URL is required for live comparison. Use --url or set MM_URL, or use --against to compare two snapshot files."}
				}

				ctx := context.Background()
				client, err := NewLiveClient(ctx, urlFlag, tokenFlag, usernameFlag, verbose)
				if err != nil {
					return err
				}

				liveConfig, err := client.GetConfig(ctx)
				if err != nil {
					return err
				}

				RedactConfig(liveConfig)

				targetConfig = liveConfig
				comparedSource = DiffSource{
					Source:     "live",
					ServerURL:  client.ServerURL(),
					CapturedAt: "now",
				}
			}

			ignoreFields := ParseIgnoreFields(diffIgnoreFields)
			result := CompareConfigs(baselineConfig, targetConfig, ignoreFields)

			result.Baseline = DiffSource{
				File:       filepath.Base(diffBaseline),
				Source:     "file",
				CapturedAt: baselineMeta.CapturedAt,
			}
			result.Compared = comparedSource

			var output string
			switch diffFormat {
			case "json":
				output, err = FormatDiffJSON(result)
				if err != nil {
					return err
				}
				output += "\n"
			case "text":
				output = FormatDiffText(result)
			default:
				return &ExitError{Code: ExitConfigError, Message: fmt.Sprintf("error: unsupported format %q. Use 'text' or 'json'.", diffFormat)}
			}

			if err := WriteOutput(output, diffOutput); err != nil {
				return err
			}

			if result.DriftDetected {
				return &ExitError{Code: ExitDriftFound, Message: ""}
			}

			return nil
		},
	}

	diffCmd.Flags().StringVar(&diffBaseline, "baseline", "", "Path to the baseline snapshot file (required)")
	diffCmd.Flags().StringVar(&diffAgainst, "against", "", "Path to a second snapshot to compare against (default: live instance)")
	diffCmd.Flags().StringVar(&diffIgnoreFields, "ignore-fields", "", "Comma-separated dot-notation field paths to exclude from comparison")
	diffCmd.Flags().StringVar(&diffFormat, "format", "text", "Output format: text, json")
	diffCmd.Flags().StringVar(&diffOutput, "output", "", "Write output to file (default: stdout)")
	rootCmd.AddCommand(diffCmd)

	// Execute the root command.
	if err := rootCmd.Execute(); err != nil {
		if exitErr, ok := err.(*ExitError); ok {
			if exitErr.Message != "" {
				fmt.Fprintln(os.Stderr, exitErr.Message)
			}
			return exitErr.Code
		}
		return ExitConfigError
	}

	return ExitSuccess
}
