# MCP discovery

SourceWard inventories local MCP configuration as data. It never starts a
server, imports server code, resolves packages, follows configuration URLs, or
tests credentials.

## Supported locations

| Scope | Provider | Location | Format |
|---|---|---|---|
| Project | Portable MCP | `.mcp.json` | JSON `mcpServers` or `servers` |
| Project | Visual Studio Code | `.vscode/mcp.json` | JSON `servers` |
| Project | Cursor | `.cursor/mcp.json` | JSON `mcpServers` |
| Project | Codex | `.codex/config.toml` | TOML `mcp_servers` |
| Personal | GitHub Copilot | `~/.copilot/mcp-config.json` | JSON |
| Personal | Cursor | `~/.cursor/mcp.json` | JSON |
| Personal | Claude Code | `~/.claude.json` | JSON user-level `mcpServers` |
| Local | Claude Code | `~/.claude.json` | Current project entry under `projects` |
| Personal | Codex | `~/.codex/config.toml` | TOML `mcp_servers` |

Portable `.mcp.json` entries identify their known consumers in artifact
metadata rather than being duplicated once per compatible application.

## Artifact identity

MCP server identity includes provider, scope, and configured server name:

```text
mcp:<provider>:<scope>:<server-name>
```

Examples:

```text
mcp:portable-mcp:project:github
mcp:cursor:personal:playwright
mcp:claude-code:local:database
```

This preserves independent configurations that happen to use the same display
name.

## Retained metadata

SourceWard retains only the information needed for inventory and deterministic
policy:

- Transport type.
- Command basename for local stdio servers.
- Endpoint scheme and hostname for remote servers.
- Environment-variable names.
- HTTP-header names.
- Enabled state where the provider exposes it.
- Configuration format and compatible consumers.

The complete command path, command arguments, endpoint path, query parameters,
URL credentials, environment values, header values, and unrelated provider
settings are not exported.

## Secret handling

SourceWard reads configuration files locally but never includes credential
values in artifacts, diagnostics, lockfiles, or test output. MCP lockfile
integrity is derived from sanitized metadata rather than the raw configuration
file. Rotating a secret therefore does not expose or change a committed
lockfile, while changes to transport, command identity, endpoint host, or
variable names do.

Malformed configurations produce a fixed diagnostic without including parser
errors or source fragments.

## Limitations

- Dynamic MCP registrations performed only through an editor extension API are
  not visible in v0.1.0 discovery.
- VS Code profile-specific user configuration is not yet inspected; the
  portable Copilot user configuration is supported.
- Command arguments are intentionally not exported. Later security analysis may
  inspect them locally but must continue to redact values from results.
- SourceWard does not determine whether a configured server is reachable or
  trustworthy during discovery.
