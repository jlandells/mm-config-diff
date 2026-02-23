# PRD: mm-config-diff — Mattermost Configuration Drift Detector

**Version:** 1.0  
**Status:** Ready for Development  
**Language:** Go  
**Binary Name:** `mm-config-diff`

---

## 1. Overview

`mm-config-diff` is a standalone command-line utility for detecting and reporting changes to a Mattermost instance's configuration over time. It operates via two subcommands: `snapshot`, which captures the current configuration from the API and saves it to a local JSON file; and `diff`, which compares a saved snapshot against either the current live configuration or another snapshot, reporting any fields that have changed, been added, or been removed.

Configuration is stored in the Mattermost database and exposed fully via the API — no file system access to the server is required.

---

## 2. Background & Problem Statement

Mattermost advocates "config in DB" as the recommended approach for production deployments, particularly in clustered and cloud environments. This is the correct architecture, but it comes with a governance trade-off: configuration changes made via the System Console or API leave no inherent audit trail visible to administrators.

In enterprise and government deployments, the ability to answer "what changed in the configuration, and when?" is often a hard requirement — for change management processes, post-incident reviews, and compliance frameworks such as ISO 27001, CIS Controls, and NIST SP 800-53. Without tooling, this requires either direct database access or manual, screenshot-based before/after comparisons.

This tool provides a lightweight, API-based solution that fits into existing change management processes without requiring database access or infrastructure changes.

---

## 3. Goals

- Allow administrators to capture a point-in-time snapshot of Mattermost configuration
- Allow comparison of any two snapshots, or a snapshot against the live instance
- Produce clear, human-readable diff output showing exactly what changed
- Never expose sensitive configuration values (passwords, secrets, tokens) in snapshot files or diff output
- Be simple enough to incorporate into scheduled jobs or change management pipelines

---

## 4. Non-Goals

- This tool does not modify configuration — it is read-only
- This tool does not provide a full audit log of who made each change (that is beyond what the API exposes)
- This tool does not revert configuration to a previous state
- This tool does not validate configuration for correctness
- The `watch` subcommand described below is a stretch goal and is explicitly out of scope for v1.0

---

## 5. Target Users

Mattermost System Administrators and Change Management / IT Operations teams in regulated environments who need to track and evidence configuration changes.

---

## 6. User Stories

- As a System Administrator, I want to capture the current configuration before making changes, so that I can compare before and after.
- As a Change Manager, I want to confirm that the only changes made to Mattermost configuration were the ones approved in the change request.
- As a Security Officer, I want to compare this week's configuration against last week's to check for unexpected drift.
- As a System Administrator responding to an incident, I want to quickly see what configuration changed in the last 24 hours.

---

## 7. Functional Requirements

### 7.1 Snapshot Subcommand

`mm-config-diff snapshot [flags]`

- MUST call `GET /api/v4/config` to retrieve the full current configuration
- MUST strip all sensitive fields from the output before writing to disk (see Section 7.4)
- MUST write the sanitised configuration to a JSON file
- The output file MUST include a metadata header containing:
  - The server URL the snapshot was taken from
  - The timestamp of the snapshot (ISO 8601, UTC)
  - The tool version
- If `--output` is not specified, the default filename MUST be `mm-config-snapshot-{TIMESTAMP}.json`, written to the current working directory
- MUST print the path of the written file to stdout on success

### 7.2 Diff Subcommand

`mm-config-diff diff [flags]`

- MUST accept a `--baseline FILE` specifying the snapshot to compare against
- MUST accept an optional `--against FILE` to compare against a second snapshot; if not specified, the comparison target is the live instance (fetched via the API)
- When comparing against the live instance, the same sensitive field stripping MUST be applied to the live data before comparison
- MUST report:
  - Fields that have **changed** (old value → new value)
  - Fields that have been **added** (present in target, not in baseline)
  - Fields that have been **removed** (present in baseline, not in target)
- If there are no differences, MUST output: `No configuration drift detected.` and exit with code 0
- MUST support `--ignore-fields` to exclude specific fields from comparison (see Section 7.3)

### 7.3 Ignorable Fields

Some configuration fields change legitimately and frequently (e.g. metrics counters, last-updated timestamps). The tool MUST support a `--ignore-fields` flag accepting a comma-separated list of dot-notation field paths to exclude from comparison, e.g.:

```
--ignore-fields ServiceSettings.TLSStrictTransportMaxAge,MetricsSettings.Enable
```

A sensible set of commonly-ignored fields should be documented in the README as a starting point.

### 7.4 Sensitive Field Redaction

The following categories of fields MUST be redacted from both snapshots and diff output. Redacted fields should be replaced with the string `[REDACTED]` rather than being omitted entirely, so that their presence (but not their value) is visible.

Fields to redact include (but are not limited to):

- `SqlSettings.DataSource` — database connection string (contains credentials)
- `SqlSettings.DataSourceReplicas` — same
- `SqlSettings.AtRestEncryptKey`
- `EmailSettings.SMTPPassword`
- `FileSettings.PublicLinkSalt`
- `FileSettings.AmazonS3SecretAccessKey`
- `GitLabSettings.Secret`
- `GoogleSettings.Secret`
- `Office365Settings.Secret`
- `OpenIdSettings.ClientSecret`
- `ServiceSettings.SiteURL` — not sensitive, but instance-specific; document it as commonly ignored
- Any field whose name contains `Password`, `Secret`, `Salt`, `Key`, or `Token` (case-insensitive) — as a belt-and-braces catch for fields not explicitly listed

The redaction list MUST be maintained as a clearly documented constant in the code, and the README must explicitly list all redacted fields so that administrators know what is and is not captured.

### 7.5 Configuration Store Detection

As part of every run (both `snapshot` and `diff` subcommands), the tool MUST determine whether
the instance is using database-stored configuration ("config in DB") or file-based configuration
(`config.json`). This can be inferred from the configuration returned by the API.

**If config-in-DB is in use:** no action required. This is the recommended configuration and
requires no comment in the output.

**If config-in-DB is NOT in use:** the tool MUST print a prominent warning to stderr at the
start of the run, and include the following notice in the summary section of all output formats:

```
⚠  Warning: This instance is using file-based configuration (config.json) rather than
   database-stored configuration. If you are running more than one Mattermost node, it is
   essential that the config.json is identical across all nodes — inconsistent configuration
   in a multi-node deployment can cause unpredictable and difficult-to-diagnose behaviour.
   Please verify that your config.json files are in sync across all nodes.
```

**Rationale:** The Mattermost API cannot reliably report the true number of configured nodes —
a cluster with one or more failing nodes will appear identical to a single-node deployment.
It is therefore impossible to programmatically determine with certainty whether a multi-node
deployment exists. The warning is shown whenever file-based config is detected, regardless of
apparent node count, because the cost of an unnecessary warning is negligible compared to the
cost of silently missing a genuine config inconsistency across nodes.

### 7.6 Snapshot File Format

```json
{
  "_metadata": {
    "tool": "mm-config-diff",
    "tool_version": "1.0.0",
    "server_url": "https://mattermost.example.com",
    "captured_at": "2025-11-01T09:00:00Z"
  },
  "ServiceSettings": {
    "SiteURL": "https://mattermost.example.com",
    "ListenAddress": ":8065",
    ...
  },
  ...
}
```

---

## 8. CLI Specification

### Usage

```
mm-config-diff <subcommand> [flags]
```

Subcommands: `snapshot`, `diff`

### Connection Flags (required for snapshot and live diff)

| Flag | Environment Variable | Description |
|------|----------------------|-------------|
| `--url URL` | `MM_URL` | Mattermost server URL |

Not required when using `diff --baseline FILE --against FILE` (two-snapshot comparison).

### Authentication Flags

| Flag | Environment Variable | Description |
|------|----------------------|-------------|
| `--token TOKEN` | `MM_TOKEN` | Personal Access Token (preferred) |
| `--username USERNAME` | `MM_USERNAME` | Username for password-based auth |
| *(no flag)* | `MM_PASSWORD` | Password (env var only — never a CLI flag) |

Authentication resolution order:
1. `--token` / `MM_TOKEN`
2. `--username` + interactive password prompt (if terminal is interactive)
3. `--username` + `MM_PASSWORD` environment variable

### Snapshot Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--output FILE` | `mm-config-snapshot-{TIMESTAMP}.json` | Output file path |

### Diff Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--baseline FILE` | *(required)* | Path to the baseline snapshot JSON file |
| `--against FILE` | *(live instance)* | Path to a second snapshot to compare against; if omitted, compares against live |
| `--ignore-fields FIELDS` | *(none)* | Comma-separated dot-notation field paths to exclude from comparison |
| `--format text\|json` | `text` | Output format for diff results |
| `--output FILE` | *(stdout)* | Write diff output to a file |

### Common Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--verbose` / `-v` | `false` | Enable verbose logging to stderr |

### Exit Codes

| Code | Meaning |
|------|---------|
| `0` | Success (snapshot written, or diff completed with no differences found) |
| `1` | Configuration error (missing required flags, invalid file path) |
| `2` | API error (connection failure, auth failure, unexpected response) |
| `3` | Differences found (diff subcommand only — allows scripts to detect drift) |
| `4` | Output error (unable to write file) |

Note: exit code 3 is intentional and important — it allows the tool to be used in scripts and pipelines where detecting drift should trigger further action.

---

## 9. Output Specification

### 9.1 Text Diff Format

Human-readable, similar in style to a unified diff but using field paths:

```
Configuration drift detected between:
  Baseline : mm-config-snapshot-2025-10-01T09:00:00Z.json (captured 2025-10-01 09:00 UTC)
  Compared : live instance at https://mattermost.example.com (captured 2025-11-01 10:00 UTC)

CHANGED (3):
  ServiceSettings.MaximumLoginAttempts
    Before : 10
    After  : 5

  EmailSettings.EnableSignInWithEmail
    Before : true
    After  : false

  PluginSettings.Enable
    Before : true
    After  : false

ADDED (1):
  ExperimentalSettings.NewSetting : "some-value"

REMOVED (0):
  (none)
```

### 9.2 JSON Diff Format

```json
{
  "baseline": {
    "file": "mm-config-snapshot-2025-10-01T09:00:00Z.json",
    "captured_at": "2025-10-01T09:00:00Z"
  },
  "compared": {
    "source": "live",
    "server_url": "https://mattermost.example.com",
    "captured_at": "2025-11-01T10:00:00Z"
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

---

## 10. Authentication Detail

The token or user account MUST have **System Administrator** role. The `/api/v4/config` endpoint is restricted to System Administrators.

Password handling:
- Interactive terminal: prompt with echo suppressed via `golang.org/x/term`
- Non-interactive: use `MM_PASSWORD` environment variable
- Never accept password as a CLI flag

---

## 11. API Endpoints Used

| Endpoint | Purpose |
|----------|---------|
| `GET /api/v4/config` | Retrieve full instance configuration (also used to detect config store type) |
| `GET /api/v4/cluster/status` | Check for active cluster nodes (informational only — see section 7.5 for why this cannot be relied upon alone) |

---

## 12. Error Handling

- Missing `--url` / `MM_URL` when required: exit code 1 with clear message
- Authentication failure: exit code 1 with clear message
- Baseline file not found or unreadable: exit code 1 with clear message
- Baseline file metadata does not match expected format (wrong tool, corrupted): exit code 1 with clear message
- API unreachable: exit code 2 with clear message
- No differences found: exit code 0 with message `No configuration drift detected.`
- Differences found: exit code 3 (documented behaviour, not an error)

---

## 13. Testing Requirements

- Unit tests for sensitive field redaction (verify that known sensitive fields are replaced with `[REDACTED]`)
- Unit tests for catch-all redaction (fields containing `Password`, `Secret`, etc. in their name)
- Unit tests for diff logic (changed, added, removed detection)
- Unit tests for `--ignore-fields` exclusion
- Unit tests for JSON snapshot serialisation and deserialisation round-trip
- Unit tests for exit code 3 on drift detection

---

## 14. Out of Scope (v1.0)

- `watch` subcommand (periodic polling with Mattermost webhook alerting) — deferred to v2.0
- Reverting configuration to a previous snapshot
- Diffing specific sections only (full config diff only in v1.0)

---

## 15. Acceptance Criteria

- [ ] `mm-config-diff snapshot --url URL --token TOKEN` writes a valid JSON file and prints its path
- [ ] The snapshot file contains `_metadata` with server URL and timestamp
- [ ] All fields matching the sensitive field list are replaced with `[REDACTED]` in the snapshot
- [ ] `mm-config-diff diff --baseline FILE` compares the baseline against the live instance and reports changes
- [ ] `mm-config-diff diff --baseline FILE --against FILE2` compares two snapshots without requiring `--url`
- [ ] When no differences are found, output is `No configuration drift detected.` and exit code is 0
- [ ] When differences are found, exit code is 3
- [ ] `--ignore-fields` correctly excludes the specified fields from comparison
- [ ] `--format json` produces valid, `jq`-parseable JSON diff output
- [ ] All errors go to stderr; all data output goes to stdout
- [ ] Binary runs on Linux (amd64), macOS (arm64 and amd64), and Windows (amd64) without dependencies