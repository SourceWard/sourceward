# Artifact provenance

SourceWard records what can be established locally about an artifact's origin.
Provenance is evidence, not a trust verdict: a known repository or publisher
can still distribute unsafe content, while missing provenance is represented
explicitly rather than inferred as safe.

## Provenance kinds

| Kind | Artifacts | Retained evidence |
|---|---|---|
| `git` | Agent Skills inside a Git worktree | Public repository URL, revision, artifact subdirectory, dirty state |
| `marketplace` | VS Code and Cursor extensions | Provider, publisher, package identifier |
| `configuration` | MCP servers | Configuration provider and server identifier |
| `unknown` | Artifacts without verifiable local evidence | No inferred source or trust |

The immutable Git revision and mutable dirty state are separate fields. Dirty
state is scoped to the artifact subdirectory, so unrelated repository changes
and lockfile creation do not alter the artifact's provenance.

## Git worktrees and nested artifacts

For an Agent Skill, SourceWard asks the local Git executable for the enclosing
worktree root and current `HEAD`. The recorded subdirectory is relative to that
worktree root and uses slash separators so lockfiles remain portable across
operating systems.

Linked Git worktrees are supported. Filesystem aliases and symlinks are
canonicalized before calculating the relative subdirectory.

If the directory is not in a Git worktree, Git is unavailable, or `HEAD`
cannot be resolved, provenance is `unknown`. SourceWard does not guess from
directory names or manifest text.

## Repository URL sanitization

Only public network repository locations are retained:

- `http` and `https`
- `ssh`
- `git`
- SCP-style SSH remotes such as `git@example.com:owner/repository.git`

User information, passwords, query parameters, and fragments are removed.
SCP-style remotes are normalized to an `ssh://` URL without the username.
Local paths and `file://` remotes are omitted because they are not portable and
may reveal private filesystem locations.

## Lockfile compatibility

Lockfile schema version 2 adds structured provenance to each artifact. This is
an intentional pre-v0.1 compatibility break: schema version 1 lockfiles must be
regenerated with the current SourceWard CLI.

The lockfile stores only portable provenance. It does not store the enclosing
worktree path, user home directory, credential-bearing remote, or raw Git
command output.
