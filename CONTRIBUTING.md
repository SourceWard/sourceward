# Contributing

SourceWard welcomes focused bug reports, detection improvements, tests, and
support for additional capability formats.

## Workflow

All changes should be made through a pull request:

1. Branch from the latest `main`.
2. Keep the branch focused on one outcome.
3. Use imperative commit subjects that explain the change.
4. Keep mechanical cleanup separate from behavioral changes.
5. Rebase or merge the latest `main` before requesting final review.
6. Merge only after required checks pass.

Suggested branch prefixes are `feat/`, `fix/`, `docs/`, `test/`, and `chore/`.
Pull requests should explain the user-visible outcome, security and privacy
impact, and validation performed.

## Development

```sh
make check
```

`make check` verifies formatting, runs static analysis and unit tests, exercises
the compiled CLI against an isolated fixture, and builds the binary.

Changes should:

- Include tests for new behavior.
- Keep scans deterministic by default.
- Preserve stable JSON fields or explicitly document compatibility changes.
- Keep provider-specific behavior behind discovery or analysis adapters.
- Explain the evidence behind security findings.
- Avoid uploading source code or skill contents without explicit consent.
- Distinguish suspicious behavior from confirmed malicious behavior.

For substantial changes, open an issue before implementation so the security
and compatibility implications can be discussed.

See [docs/architecture.md](docs/architecture.md) for package boundaries and
design principles.

By contributing, you agree that your contribution is licensed under Apache-2.0.
