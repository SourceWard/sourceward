# Contributing

SourceWard welcomes focused bug reports, detection improvements, tests, and
support for additional capability formats.

## Development

```sh
make check
```

`make check` verifies formatting, runs static analysis and unit tests, exercises
the compiled CLI against an isolated fixture, and builds the binary.

Changes should:

- Include tests for new behavior.
- Keep scans deterministic by default.
- Explain the evidence behind security findings.
- Avoid uploading source code or skill contents without explicit consent.
- Distinguish suspicious behavior from confirmed malicious behavior.

For substantial changes, open an issue before implementation so the security
and compatibility implications can be discussed.

By contributing, you agree that your contribution is licensed under Apache-2.0.
