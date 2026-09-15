# SourceWard

**Trust what your agents run.**

SourceWard is a local-first inventory and security tool for the capabilities
developers and AI agents install and execute.

The initial CLI discovers:

- Agent Skills in GitHub Copilot-compatible project and personal locations.
- Skills configured through `COPILOT_SKILLS_DIRS`.
- MCP servers configured for GitHub Copilot, VS Code, Cursor, Claude Code, and
  Codex without exposing credential values.
- Installed Visual Studio Code and Cursor extensions, including versions.

It also performs deterministic baseline checks for dangerous skill behavior,
including remote content piped to a shell, credential access, broad tool
permissions, likely secret use, and hidden Unicode controls.

> [!IMPORTANT]
> SourceWard is early-stage security software. A clean report is not proof that
> an artifact is safe.

## Install from source

SourceWard currently requires Go 1.26 or newer:

```sh
go install github.com/SourceWard/sourceward/cmd/sourceward@latest
```

For local development:

```sh
git clone https://github.com/SourceWard/sourceward.git
cd sourceward
make build
./bin/sourceward discover
```

## Usage

Inventory skills and editor extensions:

```sh
sourceward discover
sourceward discover --format json
sourceward discover --root /path/to/repository
```

Discovery JSON includes structured diagnostics for unavailable providers,
malformed artifacts, and inaccessible configured locations. Human-readable
diagnostics are written to standard error. See
[artifact discovery](docs/discovery.md) for the adapter and diagnostic contract.
MCP-specific locations and privacy guarantees are documented in
[MCP discovery](docs/mcp-discovery.md).

Audit discovered skills:

```sh
sourceward audit
sourceward audit --fail-on medium
sourceward audit --format json
sourceward audit --format sarif > sourceward.sarif
```

`audit` exits unsuccessfully when it finds an issue at or above the configured
threshold. The default threshold is `high`.

SARIF output is compatible with GitHub code scanning. Finding locations inside
the selected repository root use repository-relative paths. Locations outside
the repository, such as personal skills, are omitted from SARIF rather than
exposing local filesystem paths.

See [GitHub code scanning](docs/github-code-scanning.md) for a complete workflow.

Create a deterministic lockfile for project artifacts:

```sh
sourceward lock
sourceward lock --check
sourceward lock --output security/sourceward.lock.json
```

Lockfiles include project artifacts by default so they can be committed without
capturing developer-machine state. Use `--include-personal` only for an
explicitly local inventory. Content-backed artifacts receive a SHA-256 digest
covering all files in the artifact directory; artifacts without locally
addressable contents receive a metadata digest.

## Current scope

This first release establishes the normalized artifact inventory and scanner
interfaces. Planned artifact types include MCP servers, hooks, custom agents,
IDE extensions, and agent-downloaded executables.

See [ROADMAP.md](ROADMAP.md) for product direction and the
[MVP plan](docs/mvp-plan.md) for the ordered implementation backlog and release
criteria.

## Security

Report security vulnerabilities according to [SECURITY.md](SECURITY.md). Do not
open public issues for undisclosed vulnerabilities.

## License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE).
