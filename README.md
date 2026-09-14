# SourceWard

**Trust what your agents run.**

SourceWard is a local-first inventory and security tool for the capabilities
developers and AI agents install and execute.

The initial CLI discovers:

- Agent Skills in GitHub Copilot-compatible project and personal locations.
- Skills configured through `COPILOT_SKILLS_DIRS`.
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

Audit discovered skills:

```sh
sourceward audit
sourceward audit --fail-on medium
sourceward audit --format json
```

`audit` exits unsuccessfully when it finds an issue at or above the configured
threshold. The default threshold is `high`.

## Current scope

This first release establishes the normalized artifact inventory and scanner
interfaces. Planned artifact types include MCP servers, hooks, custom agents,
IDE extensions, and agent-downloaded executables.

See [ROADMAP.md](ROADMAP.md) for the intended sequence.

## Security

Report security vulnerabilities according to [SECURITY.md](SECURITY.md). Do not
open public issues for undisclosed vulnerabilities.

## License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE).
