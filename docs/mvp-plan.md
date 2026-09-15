# SourceWard MVP plan

This document defines the shortest path from the current prototype to a usable,
testable SourceWard v0.1.0.

## Product promise

SourceWard v0.1.0 lets a developer or small engineering team run one local
command or one GitHub workflow to:

1. Inventory Agent Skills, MCP servers, and installed VS Code-family
   extensions.
2. Explain where those capabilities came from and what they can access or
   execute.
3. Detect high-signal security risks without executing third-party code.
4. Enforce a repository-owned policy and approved artifact lockfile.
5. Produce actionable terminal, JSON, and SARIF results without uploading
   source code.

The initial user is a developer, repository maintainer, platform engineer, or
security engineer responsible for repositories that use AI coding agents.

## MVP boundary

### Included

- GitHub Copilot-compatible Agent Skills.
- Claude Code, Codex, GitHub Copilot/VS Code, and Cursor MCP configurations
  where the provider publishes a stable local format.
- Installed Visual Studio Code and Cursor extension packages.
- Local filesystem and Git provenance.
- Deterministic static analysis.
- Repository policy, exceptions, and lockfile drift.
- Local CLI, JSON, SARIF, GitHub Actions, and versioned binaries.
- macOS, Linux, and Windows acceptance testing.

### Not included

- Runtime endpoint detection or process interception.
- Automatic credential rotation or device quarantine.
- Hosted organization inventory.
- A public skills marketplace.
- Real-time cloud advisory distribution.
- Model-assisted or nondeterministic security judgments.
- Role-based skill recommendations.
- Automatic installation or execution of skills, extensions, or MCP servers.
- Enterprise identity, billing, SSO, SIEM, or device-management integrations.

These are possible post-MVP capabilities. They must not expand the v0.1.0
critical path.

## Current foundation

The repository already provides:

- Agent Skill and editor-extension discovery.
- Five deterministic Agent Skill security rules.
- Human-readable and JSON output.
- Versioned, deterministic artifact lockfiles and drift detection.
- SARIF 2.1.0 output and GitHub code-scanning upload.
- Unit, race, shuffled, and compiled-command tests.
- Protected `main`, pull-request checks, and documented architecture rules.

## Feature backlog

All MVP work is tracked in the
[MVP v0.1.0 milestone](https://github.com/SourceWard/sourceward/milestone/1).

| Order | Issue | Feature | Deliverable |
|---:|---|---|---|
| 0 | [#6](https://github.com/SourceWard/sourceward/issues/6) | MVP epic | Product-level acceptance checklist |
| 1 | [#7](https://github.com/SourceWard/sourceward/issues/7) | Discovery adapters and diagnostics | Extensible providers with explicit partial-failure reporting |
| 2 | [#8](https://github.com/SourceWard/sourceward/issues/8) | MCP discovery | Secret-safe normalized MCP inventory |
| 3 | [#9](https://github.com/SourceWard/sourceward/issues/9) | Extension contents | Cross-platform package metadata and content integrity |
| 4 | [#10](https://github.com/SourceWard/sourceward/issues/10) | Provenance | Git revision, publisher, source, and dirty-state evidence |
| 5 | [#11](https://github.com/SourceWard/sourceward/issues/11) | MCP and extension analysis | High-signal deterministic rules for new artifact types |
| 6 | [#12](https://github.com/SourceWard/sourceward/issues/12) | Repository policy | Versioned policy, scoped exceptions, and enforcement |
| 7 | [#13](https://github.com/SourceWard/sourceward/issues/13) | Artifact diff | Actionable added, removed, and changed lockfile details |
| 8 | [#14](https://github.com/SourceWard/sourceward/issues/14) | Unified scan | One command and documented stable exit codes |
| 9 | [#15](https://github.com/SourceWard/sourceward/issues/15) | Distribution | Signed binaries and a reusable GitHub Action |
| 10 | [#16](https://github.com/SourceWard/sourceward/issues/16) | Acceptance suite | Cross-platform proof of the complete product promise |

## Delivery sequence

### Phase 1: trustworthy inventory

Issues: #7, #8, #9, #10.

Create a provider-adapter boundary, expose discovery diagnostics, add MCP and
installed-extension packages, and capture provenance. At the end of this phase,
SourceWard must describe what is present without executing it or leaking
credentials.

**Gate:** representative fixtures produce stable, secret-free inventory and
content integrity on every supported operating system.

### Phase 2: actionable evaluation

Issues: #11, #12.

Add artifact-specific deterministic rules, then apply repository-owned policy
to findings and inventory. Policy follows the normalized model rather than
provider-specific configuration.

**Gate:** a repository can express approved sources and risk thresholds, and
every enforcement result includes evidence and a specific remediation path.

### Phase 3: usable workflow

Issues: #13, #14.

Explain drift and compose discovery, analysis, policy, and lockfile checks
behind one `sourceward scan` command.

**Gate:** a developer can understand and resolve a failed scan without reading
SourceWard source code or manually comparing JSON.

### Phase 4: distributable MVP

Issues: #15, #16.

Publish verifiable binaries and a reusable action, then exercise the complete
workflow on macOS, Linux, and Windows.

**Gate:** a new repository can install SourceWard, create policy and a
lockfile, run locally, and publish code-scanning results by following only the
public documentation.

## Dependency graph

```text
#7 Discovery adapters
 ├── #8 MCP discovery
 └── #9 Extension contents
       \        /
        \      /
         #10 Provenance
               |
         #11 Risk analysis
               |
         #12 Repository policy
               |
         #13 Artifact diff
               |
         #14 Unified scan
               |
         #15 Distribution
               |
         #16 Acceptance
```

#8 and #9 may be developed independently after #7. The remaining issues form
the critical path.

## Engineering invariants

Every MVP feature must preserve these constraints:

- Local scanning is the default; no source or artifact content is uploaded.
- Third-party code is parsed as data and never imported or executed.
- Secret values never appear in output, diagnostics, lockfiles, fixtures, or
  logs.
- Optional providers may be unavailable; malformed or inaccessible configured
  providers must produce diagnostics.
- Baseline results are deterministic for the same inputs and SourceWard
  version.
- JSON, SARIF, policy, and lockfile formats are versioned automation contracts.
- Findings describe observed behavior and evidence, not unverified malicious
  intent.
- Paths uploaded through SARIF are repository-relative; personal paths are
  omitted.
- Symlinks do not expand the scanning boundary.
- Unsupported input fails explicitly rather than producing success-shaped
  output.

## Issue execution protocol

Work proceeds one feature issue at a time:

1. Confirm the previous feature PR is merged.
2. Delete its local and remote branch.
3. Update `main` with a fast-forward pull.
4. Mark the next issue in progress and create a focused branch.
5. Restate acceptance criteria as tests before or alongside implementation.
6. Implement the smallest complete vertical slice.
7. Run focused tests while iterating.
8. Run the complete required checks before committing.
9. Separate behavioral, corrective, and documentation commits when that
   improves reviewability.
10. Open a PR linked to the issue with security, privacy, compatibility, and
    validation details.
11. Resolve review feedback and require all checks before merge.
12. Update the epic and roadmap only from verified merged behavior.

Branch names use `feat/`, `fix/`, `test/`, `docs/`, or `chore/`. Commits use an
imperative subject and explain why the change exists.

## Required validation

Every behavioral pull request runs:

```sh
make check
go test -race ./...
git diff --check main...HEAD
```

Packages containing ordering, parsing, policy, integrity, or output logic also
run shuffled repeated tests:

```sh
go test -shuffle=on -count=10 ./internal/...
```

Additional requirements:

- Parsers receive malformed, empty, permission-denied, and unknown-field tests.
- Security rules receive positive and meaningful false-positive tests.
- User-visible behavior is exercised through the compiled binary.
- Output schema changes include compatibility fixtures.
- Provider path logic is tested without depending on software installed on the
  contributor's machine.
- Release changes are tested from clean environments.

## MVP exit criteria

v0.1.0 is ready only when:

- Issues #7 through #16 are complete and the epic checklist is satisfied.
- The CLI supports `discover`, `audit`, `lock`, `diff`, and `scan`.
- The supported artifact matrix is documented and tested.
- Policy can block unapproved or unsafe artifacts.
- Lockfile drift reports actionable differences.
- JSON and SARIF contain stable rule and artifact identities.
- A clean repository passes; representative unsafe fixtures fail as expected.
- No acceptance output contains fixture secrets or personal absolute paths.
- Signed release artifacts and checksums are publicly available.
- Installation and first-scan instructions work from a clean machine.
- There are no unresolved critical or high-severity defects in SourceWard
  itself.

## Effort model

At 20 focused founder hours per week, the ten feature issues are planned as
small vertical slices rather than parallel projects. The expected sequence is
approximately six to ten weeks, but quality gates control release readiness;
calendar dates do not override failed security, portability, or usability
checks.

The first usable internal model should appear after Phase 2. The public MVP is
the end of Phase 4.
