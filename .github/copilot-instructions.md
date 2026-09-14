# SourceWard development instructions

SourceWard is a local-first Go security product for Agent Skills, IDE
extensions, MCP servers, hooks, and related developer-agent capabilities.

When changing this repository:

- Follow the boundaries in `docs/architecture.md`.
- Keep `cmd/sourceward` limited to process wiring.
- Normalize provider-specific artifacts in `internal/discovery`.
- Keep deterministic analysis in `internal/audit`.
- Preserve type safety and explicit error propagation.
- Validate command input before discovery or other side effects.
- Do not add network access to baseline discovery or audit behavior.
- Do not upload artifact contents, source code, findings, or credentials.
- Include exact evidence and location data in security findings.
- Do not describe an artifact or maintainer as malicious without verified proof.
- Treat JSON output as an automation contract.
- Add focused unit tests and an end-to-end command test when behavior is
  user-visible.
- Run `make check` before considering a change complete.

Keep pull requests focused. Separate refactoring, generated changes, and
behavioral changes when they can be reviewed independently.
