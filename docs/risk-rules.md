# Deterministic risk rules

SourceWard rules report observable risk signals. A finding does not claim that
an artifact is malicious, compromised, or exploitable. Review the artifact,
its provenance, and the operational context before making a trust decision.

Discovery parses MCP configuration and extension manifests as data. It does
not start MCP servers, run package managers, invoke extension lifecycle
scripts, or load extension code. Credential values and command arguments are
reduced to non-secret signals before entering inventory or findings.

## Agent Skill rules

| ID | Severity | Signal |
|---|---|---|
| `SW001` | Critical | Remote content is piped directly to a shell |
| `SW002` | High | A sensitive credential location is referenced |
| `SW003` | High | A likely secret-bearing environment variable is referenced |
| `SW004` | Medium | Every available tool is automatically allowed |
| `SW005` | Medium | Hidden or bidirectional Unicode controls are present |

## MCP rules

| ID | Severity | Signal |
|---|---|---|
| `SW101` | High | Server launches through a command shell |
| `SW102` | High | Package execution lacks an immutable version |
| `SW103` | High | Configuration contains an inline credential value |
| `SW104` | Medium | Arguments request a broad filesystem root |
| `SW105` | High | A non-local remote endpoint uses plaintext HTTP |
| `SW106` | Medium | Likely sensitive environment variables are provided |

Inline credential detection recognizes credential-bearing URL components and
likely credential headers or environment keys with literal values. Environment
references such as `${API_TOKEN}` are not treated as inline values, although
the sensitive environment-access rule may still apply.

Package pinning checks common on-demand runners such as `npx`, `bunx`, `uvx`,
and `pnpm dlx`. It is a deterministic configuration check, not registry
resolution or proof that the selected release is trustworthy.

## Extension rules

| ID | Severity | Signal |
|---|---|---|
| `SW201` | Medium | Extension activates for every workspace event |
| `SW202` | High | Package declares an install lifecycle script |
| `SW203` | Medium | Manifest depends on a known process-execution package |
| `SW204` | Medium | Manifest depends on a known network-capable package |
| `SW205` | Medium | Manifest depends on a known code-obfuscation package |

Dependency rules use a deliberately small high-signal catalog. The presence of
a dependency identifies capability and review priority, not intent. Absence
from the catalog does not prove that an extension cannot execute processes,
access the network, or contain transformed code.

## Output

Every rule has a stable identifier and appears in table, JSON, and SARIF
output. MCP findings point to the configuration file. Extension findings point
to the installed `package.json`. Personal paths are omitted from SARIF
locations to avoid exposing developer-machine details, while artifact IDs and
severity remain available.
