# Architecture

SourceWard is a local-first security tool. Its architecture prioritizes
deterministic behavior, explicit evidence, minimal privileges, and stable
machine-readable output.

## Package boundaries

```text
cmd/sourceward
    Process entry point and exit behavior
          |
          v
internal/cli
    Command parsing, orchestration, and rendering
       |              |             |
       v              v             v
internal/discovery  internal/audit  internal/lockfile
  Artifact adapters   Rules          Integrity and drift
       \              |             /
        \             |            /
         +------------+-----------+
                      |
                      v
             internal/inventory
               Domain model
```

- `cmd/sourceward` contains process wiring only.
- `internal/cli` validates input before invoking application behavior.
- `internal/discovery` finds provider-specific artifacts and normalizes them.
- `internal/audit` evaluates normalized artifacts and returns evidence.
- `internal/lockfile` creates deterministic integrity records and detects drift.
- `internal/inventory` defines provider-neutral domain types.

Dependencies should point toward the domain model. Provider-specific concepts
must not leak into generic policy or output without a deliberate schema change.

## Design principles

### Local first

Scanning happens locally by default. Source code, skill instructions, extension
contents, credentials, and findings are not uploaded without explicit user
consent.

### Deterministic by default

The same inputs and scanner version should produce the same findings. Network
intelligence and model-assisted analysis must be explicit layers rather than
hidden dependencies of the baseline scanner.

### Evidence over labels

Findings identify the artifact, location, rule, severity, and observed evidence.
SourceWard distinguishes risky behavior from confirmed malicious intent.

### Validate before side effects

Commands reject invalid arguments before discovery, filesystem changes, network
requests, or other work begins.

### Provider-neutral core

Agent Skills, IDE extensions, MCP servers, hooks, and future artifact types use
one normalized inventory. Each provider integration is an adapter into that
model.

### Stable automation contract

Human-readable output may evolve, but JSON field changes require compatibility
consideration, documentation, and tests.

Lockfiles exclude timestamps and personal artifacts by default. Project paths
are repository-relative, artifact ordering is deterministic, and the schema is
explicitly versioned.

## Testing strategy

- Unit tests cover parsing, normalization, rules, ordering, and command logic.
- Command tests inject dependencies to avoid developer-machine state.
- End-to-end tests compile the real binary and run it against isolated fixtures.
- Security rules include positive tests and should add negative tests when a
  pattern has meaningful false-positive risk.
- CI runs formatting checks, static analysis, race-enabled tests, end-to-end
  command tests, and a clean build.

New behavior is complete only when its public command or package contract is
tested at the narrowest useful layer.
