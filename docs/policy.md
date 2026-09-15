# Repository policy

SourceWard loads `sourceward.yaml` from the selected repository root before
running discovery. Unknown fields, invalid values, multiple YAML documents, or
credential-shaped values fail validation before provider commands or artifact
inspection begin.

## Schema

```yaml
version: 1
severity_threshold: high
allowed_artifact_types:
  - agent-skill
  - mcp-server
approved_publishers:
  - github
approved_sources:
  - https://github.com/SourceWard/sourceward.git
  - portable-mcp
denied_rules:
  - SW103
require_lockfile: true
exceptions:
  - rule: SW104
    artifact: mcp:portable-mcp:project:filesystem
    reason: Repository fixture intentionally exposes its workspace root
    expires: 2026-12-31
```

`version` must be `1`. Unknown fields are rejected rather than ignored.

## Enforcement

- `severity_threshold` replaces the command's failure threshold when policy is
  present.
- `allowed_artifact_types` restricts normalized artifact kinds.
- `approved_publishers` applies to extension publisher provenance.
- `approved_sources` matches an artifact source, provenance provider, or
  sanitized repository URL.
- `denied_rules` fails policy whenever the named rule produces an active
  finding, regardless of severity threshold.
- `require_lockfile` requires `sourceward.lock.json` at the repository root.

Empty allowlists do not impose a restriction.

Policy evaluation is included in audit JSON under `policy`, with deterministic
`violations` and `applied_exceptions` arrays. Policy violations cause a
non-zero exit after output is written.

## Exceptions

Every exception must identify one stable rule ID and one exact artifact ID and
must include a reason. Global rule suppression and wildcard artifact matching
are intentionally unsupported.

`expires` is optional and uses `YYYY-MM-DD`. An exception remains valid through
the specified calendar date. Once expired, the finding remains active and
SourceWard emits an `exception_expired` policy violation.

## Secret handling

Policy is intended for source control. Credential-bearing URLs, private-key
blocks, common token prefixes, and bearer values are rejected. Store only
public source identities and non-sensitive exception rationale in
`sourceward.yaml`.
