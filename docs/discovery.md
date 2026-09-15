# Artifact discovery

SourceWard discovery is provider-based. Each adapter inspects one capability
source and returns normalized artifacts plus structured diagnostics.

```text
Provider adapter
    |
    +-- artifacts ------> stable inventory
    |
    +-- diagnostics ----> partial-failure evidence
```

Discovery is best effort across independent providers. An unavailable optional
editor does not prevent Agent Skill discovery, while an unreadable configured
location remains visible rather than being silently ignored.

## Initial adapters

| Provider | Input |
|---|---|
| `agent-skills` | Project, personal, and `COPILOT_SKILLS_DIRS` skill directories |
| `mcp` | Supported project, local, and personal MCP configuration files |
| `visual-studio-code` | `code --list-extensions --show-versions` |
| `cursor` | `cursor --list-extensions --show-versions` |

See [MCP discovery](mcp-discovery.md) for supported providers and secret
handling. Future extension-package discovery will implement the same adapter
contract.

## Diagnostics

Diagnostics are included in JSON output:

```json
{
  "artifacts": [],
  "diagnostics": [
    {
      "code": "provider_unavailable",
      "level": "info",
      "provider": "cursor",
      "message": "editor command is not installed or not available on PATH"
    }
  ]
}
```

Table output writes diagnostics to standard error, keeping standard output
suitable for artifact processing.

### Levels

| Level | Meaning |
|---|---|
| `error` | Configured data could not be inspected; the inventory is incomplete |
| `warning` | Data was partially usable or malformed |
| `info` | An optional provider is not available in the current environment |

### Current codes

| Code | Meaning |
|---|---|
| `provider_unavailable` | An optional provider command is not installed or available |
| `provider_failed` | A provider command exists but failed |
| `configured_location_unavailable` | An explicitly configured skill directory does not exist |
| `location_unreadable` | A skill directory cannot be read |
| `artifact_unreadable` | A discovered skill manifest cannot be inspected or read |
| `malformed_artifact` | A skill has an invalid local structure or unterminated frontmatter |
| `malformed_output` | A provider command returned an extension without a usable name and version |
| `malformed_configuration` | An MCP configuration file could not be parsed |
| `unsupported_transport` | An MCP server does not declare exactly one supported command or URL |

## Privacy

Diagnostics use repository-relative paths for project artifacts and `~/` paths
for files under the current home directory. Paths outside both boundaries are
reduced to their final component. SourceWard does not include raw provider
command output or operating-system error text in diagnostics because either may
contain sensitive values.

Diagnostics describe SourceWard's inspection state. They are not security
findings and do not imply that an artifact or provider is malicious.
