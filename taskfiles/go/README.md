# Go Taskfile

## What is this Taskfile?

A cross-platform Taskfile for installing the Go toolchain and running Go
unit tests, benchmarks, and fuzz targets.

macOS uses Homebrew. Linux uses the official tarball from go.dev and installs it
under `/usr/local/go` by default. Windows uses the official MSI installer from
go.dev. Development tools are installed into `GOBIN`, falling back to
`GOPATH/bin`.

Linting and formatting live in the [`golangci-lint`](../golangci-lint/README.md)
Taskfile, and vulnerability scanning lives in the
[`govulncheck`](../govulncheck/README.md) Taskfile.

## Usage

### Standalone

```sh
task -t taskfiles/go/Taskfile.yml install
task -t taskfiles/go/Taskfile.yml version
task -t taskfiles/go/Taskfile.yml verify
task -t taskfiles/go/Taskfile.yml test
```

### Included

```yaml
includes:
  go: ./taskfiles/go/Taskfile.yml
```

Then run:

```sh
task go:install
task go:version
task go:verify
task go:test
```

## Testing

Run unit tests, benchmarks, fuzz targets, and coverage against the current
project:

```sh
task go:test
task go:bench
task go:fuzz -- -fuzz FuzzName ./internal/parser
```

Each task defaults to the `./...` package pattern (except `fuzz`, which fuzzes
one target in a single package). Pass extra `go test` flags or a narrower target
after `--`:

```sh
task go:test -- -race -run TestName ./internal/...
task go:test GO_JUNIT_REPORT=report.xml GO_COVER_PROFILE=cover.out -- -race ./internal/...
task go:bench -- -bench BenchmarkName ./internal/parser
```

`test` runs `go test -v`, streams test output to stdout, writes a JUnit XML
report to `GO_JUNIT_REPORT` (default `junit.xml`), and writes a coverage profile
to `GO_COVER_PROFILE` (default `coverage.out`).
`fuzz` runs a single target for `GO_FUZZTIME` (default `30s`); Go fuzzes one
target in one package per run, so supply the `-fuzz` pattern and package after
`--`:

```sh
task go:fuzz GO_FUZZTIME=60s -- -fuzz FuzzName ./internal/parser
```

## Versions

Use `GO_VERSION` to install a specific Go toolchain release. It must use the
official release name, including the `go` prefix:

```sh
task -t taskfiles/go/Taskfile.yml install GO_VERSION=go1.26.2
task go:install GO_VERSION=go1.26.2
```

When `GO_VERSION` is empty, `install` uses the latest stable Go release. On
macOS, latest uses Homebrew while an explicit version uses the official Go
package. Linux and Windows use official Go downloads for both modes.

`go-junit-report` is installed automatically by `test` and pinned to v2.1.0:

```sh
task go:install:go-junit-report
```

## Public Tasks

| Task                        | Description                                           | Key variables      |
| --------------------------- | ----------------------------------------------------- | ------------------ |
| `install`                   | Install Go on the current operating system if missing | `INSTALL_DIR_UNIX`, `GO_VERSION` |
| `install:undo`              | Remove Go from the current operating system            | `INSTALL_DIR_UNIX` |
| `install:go-junit-report`   | Install go-junit-report into the global Go bin        | `GLOBAL_GO_BIN` |
| `upgrade`                   | Upgrade Go to the selected or latest stable release    | `INSTALL_DIR_UNIX`, `GO_VERSION` |
| `version`                   | Show the installed Go version                          | none               |
| `which`                     | Show the path to the Go binary                         | none               |
| `verify`                    | Print Go version, GOROOT, and GOPATH                   | none               |
| `test`                      | Run Go unit tests and write JUnit XML and coverage reports | `GO_JUNIT_REPORT`, `GO_COVER_PROFILE` |
| `bench`                     | Run Go benchmarks                                      | none               |
| `fuzz`                      | Run a Go fuzz target                                   | `GO_FUZZTIME`      |

## Variables

| Variable               | Default                         | Description                                                           |
| ---------------------- | -------------------------------- | ---------------------------------------------------------------------- |
| `INSTALL_DIR_UNIX`     | `/usr/local`                    | Parent directory for the Linux tarball install                        |
| `GO_ROOT_UNIX`         | `{{.INSTALL_DIR_UNIX}}/go`      | Linux Go root directory                                               |
| `GO_BIN_UNIX`          | `{{.GO_ROOT_UNIX}}/bin`         | Linux Go binary directory added to shell profiles                     |
| `GO_CMD_UNIX`          | `{{.GO_BIN_UNIX}}/go`           | Linux Go binary path used as a fallback before the shell reloads PATH |
| `GO_VERSION_URL`       | `https://go.dev/VERSION?m=text` | Endpoint used to resolve the latest stable Go version                 |
| `GO_DOWNLOAD_BASE_URL` | `https://go.dev/dl`             | Base URL for official Go downloads                                    |
| `GO_VERSION`           | empty (latest stable)           | Optional official Go release name, such as `go1.26.2`                 |
| `GO_JUNIT_REPORT`      | empty (`junit.xml`)             | Output path for the `test` XML report                                 |
| `GO_COVER_PROFILE`     | empty (`coverage.out`)          | Output path for the `test` coverage profile file                      |
| `GO_FUZZTIME`          | empty (`30s`)                   | Duration a single `fuzz` target runs before stopping                 |
| `GLOBAL_GO_BIN`        | `GOBIN` or `GOPATH/bin`         | Destination and lookup directory for installed Go development tools   |

## Notes

Linux installs replace `INSTALL_DIR_UNIX/go`. The task uses `sudo` when it is
not already running as root, then adds `GO_BIN_UNIX` to the current user's shell
profile if Go is not already available on PATH.

Downloaded Go archives are checked against the official `.sha256` published
alongside each release, and the new toolchain is extracted and smoke-tested in a
temporary directory before it replaces `INSTALL_DIR_UNIX/go`. A failed download,
a checksum mismatch, or a bad archive therefore leaves the existing installation
untouched.

macOS requires Homebrew to already be installed.
