# MVP acceptance

SourceWard's v0.1.0 acceptance suite is a native Go test so the same scenarios
run on Linux, macOS, and Windows without shell-specific behavior.

The suite builds the real CLI in a clean temporary environment and verifies:

- First-run version output from the compiled binary.
- Stable repeated JSON discovery.
- Safe Agent Skill, MCP, and extension-package inventory.
- Deterministic lock creation and clean diff.
- Unified scan with lockfile enforcement.
- Content drift and exit code `6`.
- Unsafe skill, MCP, and extension findings with exit code `4`.
- JSON and SARIF rule coverage.
- Exact policy exceptions and denied-rule exit code `5`.
- Secret-value non-disclosure.
- Sanitized personal extension paths with no absolute home path in output.

The `MVP Acceptance` workflow runs this suite independently on
`ubuntu-latest`, `macos-latest`, and `windows-latest`. The main `make check`
gate also runs it on the development platform alongside race tests, compiled
E2E tests, distribution cross-builds, formatting, vetting, and the full unit
suite.

Run it locally:

```sh
make test-acceptance
make check
```

The acceptance fixtures use inert example domains and sentinel credential
values. They do not execute downloaded content, MCP servers, extension code, or
package lifecycle scripts.
