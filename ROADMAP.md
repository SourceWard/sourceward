# Roadmap

SourceWard is being built in narrow, verifiable layers.

## 0.1: Local inventory and baseline audit

- Discover project and personal Agent Skills.
- Inventory Visual Studio Code and Cursor extensions.
- Normalize artifacts into a provider-neutral data model.
- Detect high-signal unsafe patterns.
- Provide human-readable and JSON output.

## 0.2: Provenance and change intelligence

- [x] Hash artifact contents.
- Track source repositories and immutable revisions.
- Compare newly installed versions with prior versions.
- [x] Emit a SourceWard lockfile.
- Export SARIF.

## 0.3: MCP and organization policy

- Discover MCP configurations across supported agents.
- Model permissions, credentials, tools, and network destinations.
- Apply repository and organization policy.
- Produce GitHub pull-request checks.

## Future

- Signed advisories and emergency recall.
- Sandboxed behavioral analysis.
- Cross-artifact dependency and data-flow graphs.
- Private organization inventories.
- Compatibility and effectiveness evaluation.

Roadmap items are directional and may change based on security research and
user feedback.
