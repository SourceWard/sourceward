# GitHub Action and releases

SourceWard publishes standalone binaries for Linux, macOS, and Windows on
amd64 and arm64. Release assets include `checksums.txt` and GitHub build
provenance attestations.

Verify a downloaded artifact with GitHub CLI:

```sh
gh attestation verify sourceward_0.1.0_linux_amd64 \
  --repo SourceWard/sourceward
```

## Repository workflow

A public repository can adopt SourceWard with one workflow:

```yaml
name: SourceWard

on:
  pull_request:
  push:
    branches: [main]

permissions:
  contents: read
  security-events: write

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803
      - uses: SourceWard/sourceward@v0.1.2
        with:
          check-lock: "true"
```

Pin SourceWard to a release tag or immutable commit. The action downloads the
matching release binary, verifies it against the published SHA-256 checksum,
runs `sourceward scan`, optionally uploads SARIF, and returns the CLI's stable
exit code.

SARIF upload is skipped for pull requests from forks so write credentials are
not used with untrusted fork code. GitHub also withholds write tokens for fork
pull-request workflows.

## Inputs

| Input | Default | CLI mapping |
|---|---|---|
| `version` | `v0.1.2` | Release binary version |
| `root` | `.` | `--root` |
| `format` | `sarif` | `--format` |
| `fail-on` | `high` | `--fail-on` |
| `check-lock` | `false` | `--check-lock` |
| `include-personal` | `false` | `--include-personal` |
| `upload-sarif` | `true` | Upload generated SARIF when safe |
| `sarif-file` | `sourceward.sarif` | SARIF output path |

The action exposes `exit-code` and then exits with that same value after any
eligible SARIF upload.
