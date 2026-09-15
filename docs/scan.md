# Unified scanning

`sourceward scan` is the normal local and CI entry point. One invocation:

1. Validates `sourceward.yaml` before discovery.
2. Discovers skills, MCP servers, and extension packages.
3. Evaluates deterministic audit rules.
4. Applies repository policy and exact exceptions.
5. Checks `sourceward.lock.json` when `--check-lock` is set or policy requires
   the lockfile.
6. Writes safe output before returning an enforcement exit where possible.

Individual `discover`, `audit`, `lock`, and `diff` commands remain available
for focused inspection and scripting.

## Formats

- `table` is the default human-readable format.
- `json` includes inventory, findings, policy evaluation, and an optional
  lockfile diff.
- `sarif` includes the stable audit-rule catalog and findings for code
  scanning. Policy and lockfile enforcement still affect the process exit.

## Exit codes

| Code | Meaning |
|---:|---|
| `0` | Scan completed and all requested enforcement passed |
| `2` | Invalid command input or invalid repository policy |
| `3` | Operational failure such as unreadable data or missing requested lockfile |
| `4` | Findings met or exceeded the effective severity threshold |
| `5` | Repository policy violation |
| `6` | Lockfile drift |

When multiple enforcement conditions exist, policy violations take precedence,
then lockfile drift, then the finding threshold. Invalid input and operational
errors occur before enforcement output when the requested operation cannot be
performed safely.

## CI

Generate SARIF while preserving repository policy enforcement:

```sh
sourceward scan --format sarif --fail-on none > sourceward.sarif
```

Run complete policy and lockfile enforcement with JSON evidence:

```sh
sourceward scan --format json --check-lock
```
