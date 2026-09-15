# Extension package discovery

SourceWard combines editor CLI inventory with direct inspection of installed
Visual Studio Code-family extension packages. It parses package manifests as
data and never imports, starts, or evaluates extension code.

## Supported locations

The default extension directories are the same under the user's home directory
on macOS, Linux, and Windows:

| Provider | Location |
|---|---|
| Visual Studio Code | `~/.vscode/extensions` |
| Cursor | `~/.cursor/extensions` |

Custom `--extensions-dir` locations and editor profile-specific package
directories are not yet discovered.

## Artifact identity and metadata

An installed package uses the existing editor extension identity:

```text
<provider>:<publisher>.<name>
```

The artifact records the manifest version, installed package path, provider,
and personal scope. SourceWard also retains the publisher and a deterministic
set of high-level manifest capabilities:

- Contribution-point names such as `contributes.commands`.
- Node and browser runtime declarations.
- Presence of activation events.
- Presence of extension dependencies.
- Presence of extension-pack members.

Command implementations, activation-event values, configuration contents, and
extension source code are not copied into inventory metadata.

## CLI reconciliation

When the editor command is available, SourceWard compares
`--list-extensions --show-versions` output with installed package manifests.
It reports `extension_inventory_mismatch` when:

- The CLI reports an extension without a package in an inspected default
  directory.
- A package is absent from successful CLI inventory.
- The CLI and package manifest report different versions.

If the editor command is unavailable, filesystem package discovery still
succeeds. If the default package directory does not exist, CLI-only inventory
is retained without a mismatch because the editor may use a custom location.

## Symlink and integrity handling

Symlinked extension package directories and symlinked `package.json` manifests
are rejected before parsing. Symlinks found inside an accepted package are
hashed as links: SourceWard records the link target text but does not read or
walk the target.

With `sourceward lock --include-personal`, installed extension packages receive
content integrity covering their local package trees. Repeated scans of the
same files produce the same digest, package changes alter it, and changes to
files outside the package through a symlink do not.

## Limitations

- Dynamic extensions exposed only through an editor API are not visible.
- Custom extension directories and profile-specific locations require future
  configuration support.
- CLI reconciliation is evidence of local inventory drift, not proof that an
  extension is malicious.
