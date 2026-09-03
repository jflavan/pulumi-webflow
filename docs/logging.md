# Logging and Debugging Guide

This guide explains how to use the structured logging features in the Pulumi Webflow provider to debug issues and monitor operations.

## Overview

The Pulumi Webflow provider includes comprehensive structured logging to help you:
- Debug issues during development and production
- Monitor API interactions and resource operations
- Trace rate limiting and retry behavior
- Audit provider operations for compliance

All logging uses Pulumi's native logging framework and respects Pulumi's log level configuration.

## Log Levels

The provider uses standard log levels following Pulumi conventions:

### DEBUG
Detailed information useful for development and debugging:
- API request and response details (with sensitive data redacted)
- Dry-run/preview mode notifications
- Internal operation steps

**Example:**
```
DEBUG Creating Webflow site [workspaceId=abc123, displayName=My Site]
DEBUG Calling Webflow API to create site
DEBUG HTTP request completed [method=POST, url=/v2/sites, status=200, attempt=1]
```

### INFO
General operational information about resource lifecycle:
- Resource creation, updates, and deletions
- Successful API operations
- Resource state transitions

**Example:**
```
INFO Creating Webflow site [workspaceId=abc123, displayName=My Site]
INFO Site created successfully [siteId=def456]
INFO Site published successfully
```

### WARN
Potentially problematic situations that don't prevent operations:
- Rate limiting encountered with automatic retry
- Non-fatal issues or limitations
- API limitations (e.g., resources that can't be deleted)

**Example:**
```
WARN Rate limited, retrying after 2s [method=POST, url=/v2/sites, attempt=2, retryAfter=2s]
WARN Asset folder cannot be deleted via API - removing from Pulumi state only [siteId=abc123, folderName=Images]
WARN Deleting Webflow site - this is a destructive operation [siteId=abc123]
```

### ERROR
Errors that prevent an operation from completing:
- API failures with context
- Validation errors
- Authentication failures
- Unexpected API responses

**Example:**
```
ERROR Validation failed: siteId is required [workspaceId=, displayName=My Site]
ERROR Failed to create site via API: 401 Unauthorized
ERROR API returned empty site ID
```

## Enabling Verbose Logging

There is no `PULUMI_LOG_LEVEL` environment variable. The provider emits its messages through
Pulumi's logging API: INFO/WARN/ERROR messages are shown as diagnostics in normal `pulumi`
output, while DEBUG messages are only visible when the Pulumi CLI runs with verbose logging
enabled.

### Command Line Flags
Use the CLI's `-v` (verbosity) flag together with `--logtostderr`:

```bash
# Maximum verbosity - shows the provider's DEBUG messages
pulumi up -v=9 --logtostderr

# Less verbose (engine-level messages only)
pulumi up -v=3 --logtostderr

# Capture everything to a file for analysis
pulumi up -v=9 --logtostderr 2>&1 | tee deployment.log
```

Without `--logtostderr`, Pulumi writes verbose logs to files in the system temp directory
instead of the terminal.

### Per-Operation
Enable logging for a specific operation:

```bash
# Debug a preview operation
pulumi preview -v=9 --logtostderr

# Debug an update operation
pulumi up -v=9 --logtostderr

# Debug a refresh operation
pulumi refresh -v=9 --logtostderr
```

### Provider Internals
`PULUMI_DEBUG_GRPC=<file>` records the raw gRPC traffic between the engine and the provider,
which is occasionally useful for reporting provider bugs.

## Logging in Different Scenarios

### Resource Creation
```
INFO Creating Webflow site [workspaceId=ws123, displayName=Marketing Site]
DEBUG Calling Webflow API to create site
DEBUG HTTP request completed [method=POST, url=/v2/sites, status=201, attempt=1]
INFO Site created successfully [siteId=site456]
```

### API Rate Limiting
```
DEBUG HTTP request completed [method=POST, url=/v2/collections, status=429, attempt=1]
WARN Rate limited, retrying after 1s [method=POST, url=/v2/collections, attempt=1, retryAfter=1s]
DEBUG HTTP request completed [method=POST, url=/v2/collections, status=201, attempt=2]
INFO Collection created successfully [collectionId=col789]
```

### Validation Errors
```
INFO Creating Webflow site [workspaceId=, displayName=]
ERROR Validation failed: workspaceId is required [workspaceId=, displayName=]
```

### Destructive Operations
```
WARN Deleting Webflow site - this is a destructive operation [siteId=site456, displayName=Marketing Site]
DEBUG Calling Webflow API to delete site
DEBUG HTTP request completed [method=DELETE, url=/v2/sites/site456, status=204, attempt=1]
INFO Site deleted successfully [siteId=site456]
```

## Structured Log Fields

Logs include structured fields for easy parsing and filtering:

| Field | Description | Example |
|-------|-------------|---------|
| `siteId` | Webflow site ID | `5f0c8c9e1c9d440000e8d8c3` |
| `workspaceId` | Webflow workspace ID | `ws123abc` |
| `displayName` | Resource display name | `My Site` |
| `fileName` | Asset file name | `logo.png` |
| `method` | HTTP method | `POST`, `GET`, `PATCH`, `DELETE` |
| `url` | API endpoint path | `/v2/sites`, `/v2/collections` |
| `status` | HTTP status code | `200`, `201`, `429`, `401` |
| `attempt` | Retry attempt number | `1`, `2`, `3` |
| `retryAfter` | Retry delay duration | `1s`, `2s`, `4s` |

## Sensitive Data Protection

The provider automatically redacts sensitive information in logs:

### API Tokens
Always redacted as `[REDACTED]`:
```
ERROR Failed to authenticate: token=[REDACTED]
```

### Large Responses
Truncated to prevent log spam:
```
DEBUG Response body: {...}... (truncated, 5234 total chars)
```

### Field-Based Redaction
Fields containing sensitive keywords are automatically redacted:
- `token`, `apiToken`
- `password`, `secret`
- `key`, `authorization`

## Debugging Common Issues

### Issue: "Site not found" after creation
**Enable DEBUG logging to see the API response:**
```bash
pulumi up -v=9 --logtostderr
```

Look for:
```
DEBUG HTTP request completed [method=POST, url=/v2/sites, status=201, attempt=1]
INFO Site created successfully [siteId=abc123]
```

Verify the `siteId` is correctly returned.

### Issue: Intermittent failures
**Enable DEBUG logging to see retry behavior:**
```bash
pulumi up -v=9 --logtostderr
```

Look for:
```
WARN Rate limited, retrying after 2s [method=POST, attempt=2]
WARN Rate limit exceeded, max retries exhausted [maxRetries=3]
```

This indicates you're hitting API rate limits.

### Issue: Unexpected resource updates
**INFO messages are shown by default; use `--diff` to see the property changes:**
```bash
pulumi preview --diff
```

Look for:
```
INFO Updating Webflow site [siteId=abc123, displayName=New Name]
INFO Site updated successfully
```

Compare with your code to identify the difference.

### Issue: Authentication failures
**ERROR messages are always shown; run the operation and read the diagnostic:**
```bash
pulumi up
```

Look for:
```
ERROR Failed to create HTTP client: [WEBFLOW_AUTH_001] Webflow API token not configured
ERROR Validation failed: API token cannot be empty
```

## Programmatic Access to Logs

If you're building automation around the provider, you can parse structured logs:

### JSON Format
Pulumi can output logs in JSON format:
```bash
pulumi up --json | jq '.message'
```

### Filtering Specific Events
Filter for specific operations:
```bash
# Show only site creation events
pulumi up --json | jq 'select(.message | contains("Creating Webflow site"))'

# Show only rate limiting events
pulumi up --json | jq 'select(.message | contains("Rate limited"))'

# Show only errors
pulumi up --json | jq 'select(.type == "error")'
```

## Best Practices

1. **Development**: Use `pulumi up -v=9 --logtostderr` to see all provider operations
2. **Production**: Run without `-v`; INFO/WARN/ERROR diagnostics are shown by default
3. **CI/CD**: Keep the default verbosity and archive the output; add `-v=9 --logtostderr` only when re-running a failed job
4. **Troubleshooting**: Enable `-v=9` temporarily when investigating issues
5. **Audit Trail**: Capture INFO logs for compliance and audit requirements

## Performance Considerations

- **DEBUG logging**: Minimal overhead, logs are generated but may not be displayed
- **Structured fields**: Efficient - no string concatenation until needed
- **Log level**: Respects Pulumi's configuration - logs below threshold are skipped

## Related Documentation

- [Troubleshooting Guide](./troubleshooting.md) - Common issues and solutions
- [Pulumi Logging Documentation](https://www.pulumi.com/docs/support/troubleshooting/#verbose-logging) - Pulumi's logging system
- [Performance Guide](./performance.md) - Optimizing provider operations

## Feedback

If you have suggestions for improving logging or need additional log messages, please open an issue on the [GitHub repository](https://github.com/JDetmar/pulumi-webflow/issues).
