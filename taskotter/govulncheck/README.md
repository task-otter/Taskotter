# govulncheck Taskfile

## What is this Taskfile?

A cross-platform Taskfile for scanning Go packages for known vulnerabilities
with govulncheck. Auto-installs govulncheck into the global Go bin (`GOBIN`,
falling back to `GOPATH/bin`) and auto-installs the Go toolchain itself if
it is missing.

## Usage

### Standalone

```sh
task -t taskfiles/govulncheck/Taskfile.yml install
task -t taskfiles/govulncheck/Taskfile.yml ci
```

### Included

```yaml
includes:
  govulncheck: ./taskfiles/govulncheck/Taskfile.yml
```

Then run:

```sh
task govulncheck:install
task govulncheck:ci
```

## Scanning

Scan all Go packages for known vulnerabilities by default:

```sh
task -t taskfiles/govulncheck/Taskfile.yml ci
task govulncheck:ci
```

Auto-installs govulncheck if missing. Override the default `./...` target or
pass extra flags with `--`:

```sh
task govulncheck:ci -- -test ./...
```

Set `GOVULNCHECK_LINT_SKIP_PATTERN` to exclude matching file paths from the
scan; it uses the same shell-style path glob syntax as the other Taskfiles
in this repo. govulncheck operates on packages rather than individual
files, so any package containing a matching file is omitted:

```sh
task govulncheck:ci GOVULNCHECK_LINT_SKIP_PATTERN="**/generated/**"
```

## Versions

Leave `GOVULNCHECK_VERSION` empty to install the latest release, or set an
exact module version:

```sh
task govulncheck:install GOVULNCHECK_VERSION=v1.1.4
```

Supplying a version forces the installer to run even when govulncheck
already exists.

## Public Tasks

| Task      | Description                                | Key variables                    |
| --------- | -------------------------------------------- | ----------------------------------- |
| `install` | Install govulncheck into the global Go bin   | `GO_GLOBAL_BIN`, `GOVULNCHECK_VERSION` |
| `ci`    | Scan Go packages for known vulnerabilities   | `GOVULNCHECK_LINT_SKIP_PATTERN`     |

## Variables

| Variable                        | Default                  | Description                                                    |
| --------------------------------- | -------------------------- | ------------------------------------------------------------------ |
| `GOVULNCHECK_VERSION`            | empty (`latest`)          | Optional govulncheck module version                                |
| `GOVULNCHECK_LINT_SKIP_PATTERN`  | empty                     | Shell-style path glob for Go files skipped by `ci`                |
| `GO_GLOBAL_BIN`                  | `GOBIN` or `GOPATH/bin`   | Destination and lookup directory for the installed govulncheck binary |
