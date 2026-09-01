# go-junit-report Taskfile

## What is this Taskfile?

A Taskfile for running Go unit tests with JUnit XML and coverage reports via
`go-junit-report`. Go and `go-junit-report` are installed through
`nix:install:profile`.

For plain `go test` without report artifacts, use the [`go`](../go/README.md)
Taskfile.

## Usage

### Standalone

```sh
task nix:install:profile NIX_INSTALLABLE="nixpkgs#go nixpkgs#go-junit-report"
task -t taskfiles/go-junit-report/Taskfile.yml verify
task -t taskfiles/go-junit-report/Taskfile.yml test
```

### Included

```yaml
includes:
  go-junit-report: ./taskfiles/go-junit-report/Taskfile.yml
```

Then run:

```sh
task go-junit-report:verify
task go-junit-report:test
```

Install only, without running tests:

```sh
task nix:install:profile NIX_INSTALLABLE="nixpkgs#go nixpkgs#go-junit-report"
```

## Testing

Run unit tests with coverage and write JUnit XML against the current project:

```sh
task go-junit-report:test
task go-junit-report:test GO_JUNIT_REPORT=report.xml GO_COVER_PROFILE=cover.out -- -race ./internal/...
```

`test` runs `go test -v`, streams test output to stdout, writes a JUnit XML
report to `GO_JUNIT_REPORT` (default `junit.xml`), and writes a coverage profile
to `GO_COVER_PROFILE` (default `coverage.out`). Pass extra `go test` flags or a
narrower target after `--`.

Pin a revision by overriding the installable, for example
`GO_JUNIT_REPORT_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#go-junit-report`.

## Public Tasks

| Task     | Description                                                |
| -------- | ---------------------------------------------------------- |
| `which`  | Show the path to the go-junit-report binary                |
| `verify` | Print go-junit-report version                              |
| `test`   | Run Go unit tests and write JUnit XML and coverage reports |
| `install` | Install go-junit-report via the Nix profile |
| `version` | Show the active go-junit-report version |

## Variables

| Variable                          | Default                              | Description                                         |
| --------------------------------- | -------------------------------------- | --------------------------------------------------- |
| `GO_JUNIT_REPORT_NIX_INSTALLABLE` | `nixpkgs#go nixpkgs#go-junit-report` | Flake installables passed to `nix:install:profile`  |
| `GO_JUNIT_REPORT`                 | empty (`junit.xml`)                    | Output path for the `test` XML report               |
| `GO_COVER_PROFILE`                | empty (`coverage.out`)                 | Output path for the `test` coverage profile file    |

## Notes

- Install goes through `nix:install:profile` (Nix is installed first if missing). Native Windows is not supported; use WSL2.
- `go-junit-report` needs `go` on PATH, so the default installable includes both `nixpkgs#go` and `nixpkgs#go-junit-report`.
- `test`, `which`, and `verify` auto-install Go and `go-junit-report`.
