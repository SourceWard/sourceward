# GitHub code scanning

SourceWard can publish Agent Skill findings as GitHub code scanning alerts by
generating SARIF 2.1.0 and uploading it with GitHub's CodeQL action.

```yaml
name: SourceWard

on:
  push:
    branches: [main]
  pull_request:
  schedule:
    - cron: "23 6 * * 1"

permissions:
  actions: read
  contents: read
  security-events: write

jobs:
  scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
      - uses: actions/setup-go@v7
        with:
          go-version: "1.26"
      - name: Install SourceWard
        run: go install github.com/SourceWard/sourceward/cmd/sourceward@latest
      - name: Generate SARIF
        run: sourceward audit --format sarif --fail-on none > sourceward.sarif
      - name: Upload SARIF
        if: github.event_name != 'pull_request' || github.event.pull_request.head.repo.full_name == github.repository
        uses: github/codeql-action/upload-sarif@v4
        with:
          sarif_file: sourceward.sarif
          category: sourceward
```

`--fail-on none` ensures the upload step still runs when findings exist. Teams
that also want a blocking policy can add a separate command after upload:

```sh
sourceward audit --fail-on high
```

SARIF locations are emitted only for files inside `--root`. Findings in
personal skills remain in the report but omit filesystem locations so uploaded
results do not reveal home-directory paths.

The upload condition skips fork pull requests because their `GITHUB_TOKEN`
cannot receive `security-events: write`. SARIF generation still runs normally.
