# mm-config-diff

A standalone command-line utility that detects configuration drift in Mattermost instances. It captures point-in-time snapshots of your Mattermost configuration via the API and compares them, reporting exactly which settings have changed, been added, or been removed.

## Why You'd Use It

Mattermost's recommended "config in DB" approach stores configuration in the database and exposes it via the System Console and API — but provides no built-in audit trail for configuration changes. In regulated environments (ISO 27001, CIS Controls, NIST SP 800-53), the ability to answer "what changed in the configuration, and when?" is often a hard requirement.

`mm-config-diff` fills this gap by letting you:

- Capture a baseline snapshot before a maintenance window
- Compare the current live config against that baseline after changes
- Detect unexpected configuration drift between scheduled checks
- Produce evidence of configuration changes for compliance audits

No database access or infrastructure changes required — just API access with a System Administrator account.

## Installation

Download the pre-built binary for your platform from the [Releases](https://github.com/jlandells/mm-config-diff/releases) page.

| Platform       | Filename                            |
|----------------|-------------------------------------|
| Linux (amd64)  | `mm-config-diff-linux-amd64`        |
| macOS (amd64)  | `mm-config-diff-darwin-amd64`       |
| macOS (arm64)  | `mm-config-diff-darwin-arm64`       |
| Windows (amd64)| `mm-config-diff-windows-amd64.exe`  |

On Linux and macOS, make the binary executable after downloading:

```bash
chmod +x mm-config-diff-*
```

No other dependencies are required.

## Authentication

`mm-config-diff` requires a **System Administrator** account to access the `/api/v4/config` endpoint.

### Personal Access Token (recommended)

Generate a Personal Access Token in the Mattermost System Console under **Integrations > Bot Accounts** or your user profile. Pass it via `--token` or the `MM_TOKEN` environment variable.

```bash
mm-config-diff snapshot --url https://mattermost.example.com --token your-token-here
```

> **Note:** Personal Access Tokens may be disabled on some Mattermost instances. If so, use username/password authentication instead.

### Username and Password

Pass the username via `--username` or `MM_USERNAME`. The tool will prompt for your password interactively (with echo suppressed). For non-interactive/automation use, set the `MM_PASSWORD` environment variable.

```bash
mm-config-diff snapshot --url https://mattermost.example.com --username admin
Password:
```

> **Security note:** There is intentionally no `--password` flag. Passwords passed as CLI flags appear in shell history, `ps` output, and system logs. Use the interactive prompt or `MM_PASSWORD` environment variable instead.

## Usage

`mm-config-diff` has two subcommands: `snapshot` and `diff`.

### Global Flags

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--url` | `MM_URL` | *(required)* | Mattermost server URL |
| `--token` | `MM_TOKEN` | *(empty)* | Personal Access Token |
| `--username` | `MM_USERNAME` | *(empty)* | Username for password auth |
| `--verbose` / `-v` | — | `false` | Enable verbose logging to stderr |
| `--version` | — | — | Print version and exit |

### Snapshot

Captures the current configuration and saves it to a JSON file.

```
mm-config-diff snapshot [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--output` | `mm-config-snapshot-{TIMESTAMP}.json` | Output file path |

### Diff

Compares a baseline snapshot against the live instance or a second snapshot.

```
mm-config-diff diff [flags]
```

| Flag | Default | Description |
|------|---------|-------------|
| `--baseline` | *(required)* | Path to the baseline snapshot file |
| `--against` | *(live instance)* | Path to a second snapshot to compare against |
| `--ignore-fields` | *(none)* | Comma-separated dot-notation field paths to exclude |
| `--format` | `text` | Output format: `text` or `json` |
| `--output` | *(stdout)* | Write output to a file |

When `--against` is omitted, the tool fetches the live configuration from the server (requires `--url` and authentication). When `--against` is provided, no API connection is needed.

## Examples

### Capture a snapshot with token auth

```bash
mm-config-diff snapshot --url https://mattermost.example.com --token xoxp-your-token
```

### Capture a snapshot with username/password auth

```bash
mm-config-diff snapshot --url https://mattermost.example.com --username admin
Password:
```

### Using environment variables

```bash
export MM_URL=https://mattermost.example.com
export MM_TOKEN=xoxp-your-token

mm-config-diff snapshot
mm-config-diff diff --baseline mm-config-snapshot-2025-10-01T09-00-00Z.json
```

### Compare a snapshot against the live instance

```bash
mm-config-diff diff --baseline mm-config-snapshot-2025-10-01T09-00-00Z.json \
  --url https://mattermost.example.com --token xoxp-your-token
```

### Compare two snapshot files

```bash
mm-config-diff diff \
  --baseline mm-config-snapshot-2025-10-01T09-00-00Z.json \
  --against mm-config-snapshot-2025-11-01T10-00-00Z.json
```

### Write output to a file

```bash
mm-config-diff diff --baseline snapshot-before.json --against snapshot-after.json \
  --format json --output drift-report.json
```

### Ignore frequently-changing fields

```bash
mm-config-diff diff --baseline snapshot-before.json \
  --ignore-fields "ServiceSettings.SiteURL,MetricsSettings.BlockProfileRate"
```

### Use in a script to detect drift

```bash
mm-config-diff diff --baseline baseline.json --url https://mm.example.com --token "$MM_TOKEN"
if [ $? -eq 3 ]; then
  echo "Configuration drift detected!"
  # Send alert, create ticket, etc.
fi
```

## Output Formats

### Text (default)

```
Configuration drift detected between:
  Baseline : mm-config-snapshot-2025-10-01T09-00-00Z.json (captured 2025-10-01T09:00:00Z)
  Compared : live instance at https://mattermost.example.com (captured now)

CHANGED (2):
  ServiceSettings.MaximumLoginAttempts
    Before : 10
    After  : 5

  PluginSettings.Enable
    Before : true
    After  : false

ADDED (1):
  ExperimentalSettings.NewSetting : "some-value"

REMOVED (0):
  (none)
```

When no differences are found:

```
No configuration drift detected.
```

### JSON

```json
{
  "baseline": {
    "file": "mm-config-snapshot-2025-10-01T09-00-00Z.json",
    "captured_at": "2025-10-01T09:00:00Z"
  },
  "compared": {
    "source": "live",
    "server_url": "https://mattermost.example.com",
    "captured_at": "now"
  },
  "drift_detected": true,
  "changed": [
    {
      "field": "ServiceSettings.MaximumLoginAttempts",
      "before": 10,
      "after": 5
    }
  ],
  "added": [
    {
      "field": "ExperimentalSettings.NewSetting",
      "value": "some-value"
    }
  ],
  "removed": []
}
```

## Sensitive Field Redaction

The following fields are always redacted (replaced with `[REDACTED]`) in both snapshots and diff output:

- `SqlSettings.DataSource` — database connection string
- `SqlSettings.DataSourceReplicas` — replica connection strings
- `SqlSettings.AtRestEncryptKey`
- `EmailSettings.SMTPPassword`
- `FileSettings.PublicLinkSalt`
- `FileSettings.AmazonS3SecretAccessKey`
- `GitLabSettings.Secret`
- `GoogleSettings.Secret`
- `Office365Settings.Secret`
- `OpenIdSettings.ClientSecret`

Additionally, any field whose name contains `Password`, `Secret`, `Salt`, `Key`, or `Token` (case-insensitive) is automatically redacted.

## Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success — snapshot written, or diff completed with no differences |
| `1` | Configuration error — missing flags, invalid file, auth failure |
| `2` | API error — connection failure, unexpected server response |
| `3` | Drift detected — diff completed and differences were found |
| `4` | Output error — unable to write output file |

Exit code `3` is intentional — it allows the tool to be used in scripts and pipelines where detecting drift should trigger further action.

## Commonly Ignored Fields

Some fields change frequently and are often not meaningful for drift detection. Consider adding these to `--ignore-fields`:

- `ServiceSettings.SiteURL` — instance-specific, not a configuration change
- `MetricsSettings.BlockProfileRate` — may change dynamically

## Limitations

- This tool is **read-only** — it does not modify configuration
- The tool cannot determine *who* made a configuration change (this information is not exposed by the Mattermost API)
- The tool cannot revert configuration to a previous state
- When using file-based configuration (`config.json`), the tool can only see the configuration of the node it connects to — it cannot verify that other nodes in a cluster have the same configuration
- Snapshot files capture the complete configuration. As Mattermost adds new configuration fields in future versions, new fields may appear as "added" when comparing snapshots from different server versions

## Integration Testing

To test against a local Mattermost instance:

1. Start a local Mattermost instance (e.g. via Docker)
2. Create a System Administrator account and generate a Personal Access Token
3. Run a snapshot: `mm-config-diff snapshot --url http://localhost:8065 --token <token>`
4. Make a configuration change via the System Console
5. Run a diff: `mm-config-diff diff --baseline <snapshot-file> --url http://localhost:8065 --token <token>`
6. Verify the change appears in the output

## Contributing

We welcome contributions from the community! Whether it's a bug report, a feature suggestion,
or a pull request, your input is valuable to us. Please feel free to contribute in the
following ways:
- **Issues and Pull Requests**: For specific questions, issues, or suggestions for improvements,
  open an issue or a pull request in this repository.
- **Mattermost Community**: Join the discussion in the
  [Integrations and Apps](https://community.mattermost.com/core/channels/integrations) channel
  on the Mattermost Community server.

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Contact

For questions, feedback, or contributions regarding this project, please use the following methods:
- **Issues and Pull Requests**: For specific questions, issues, or suggestions for improvements,
  feel free to open an issue or a pull request in this repository.
- **Mattermost Community**: Join us in the Mattermost Community server, where we discuss all
  things related to extending Mattermost. You can find me in the channel
  [Integrations and Apps](https://community.mattermost.com/core/channels/integrations).
- **Social Media**: Follow and message me on Twitter, where I'm
  [@jlandells](https://twitter.com/jlandells).
