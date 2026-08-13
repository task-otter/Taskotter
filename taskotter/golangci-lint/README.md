# golangci-lint Taskfile

## What is this Taskfile?

A cross-platform Taskfile for linting and formatting Go files with
golangci-lint. Auto-installs golangci-lint into the global Go bin
(`GOBIN`, falling back to `GOPATH/bin`) and auto-installs the Go toolchain
itself if it is missing.

## Usage

### Standalone

```sh
task -t taskfiles/golangci-lint/Taskfile.yml install
task -t taskfiles/golangci-lint/Taskfile.yml lint
task -t taskfiles/golangci-lint/Taskfile.yml lint:fix
task -t taskfiles/golangci-lint/Taskfile.yml fmt
task -t taskfiles/golangci-lint/Taskfile.yml fmt:check
```

### Included

```yaml
includes:
  golangci-lint: ./taskfiles/golangci-lint/Taskfile.yml
```

Then run:

```sh
task golangci-lint:install
task golangci-lint:lint
task golangci-lint:lint:fix
task golangci-lint:fmt
task golangci-lint:fmt:check
```

## Linting

Run golangci-lint against all Go packages by default:

```sh
task -t taskfiles/golangci-lint/Taskfile.yml lint
task golangci-lint:lint
```

Auto-installs golangci-lint if missing. When `.custom-gcl.yml` exists in the
working directory, `lint` and `lint:fix` build or reuse its custom binary
instead of the stock golangci-lint binary.

Override the default `./...` target or pass extra flags with `--`:

```sh
task golangci-lint:lint -- ./internal/...
```

Set `GOLANGCI_LINT_LINT_SKIP_PATTERN` to exclude matching file paths from
lint analysis. It uses the same shell-style path glob syntax as
`GOLANGCI_LINT_FMT_SKIP_PATTERN`; quote the value so your shell passes it
through unchanged:

```sh
task golangci-lint:lint GOLANGCI_LINT_LINT_SKIP_PATTERN="**/generated/**"
```

The `config:skip` task translates the pattern into a
`linters.exclusions.paths` regex and merges it into a copy of the existing
golangci-lint YAML or JSON configuration, so project-specific settings
remain active. The copy is written as `.golangci-taskotter-skip.yml` next to
your golangci-lint config, and `lint` and `lint:fix` run it first. It is
rewritten on every run and is not deleted afterwards, so **add
`.golangci-taskotter-skip.yml` to your `.gitignore`**; running `config:skip`
with no skip pattern set deletes it.

Auto-fix lint issues golangci-lint can rewrite (run `fmt` separately to also
reformat):

```sh
task -t taskfiles/golangci-lint/Taskfile.yml lint:fix
task golangci-lint:lint:fix -- ./internal/...
```

## Formatting

Format Go files in place:

```sh
task -t taskfiles/golangci-lint/Taskfile.yml fmt
task golangci-lint:fmt
```

The formatter runs `golangci-lint fmt` with `gci`, `gofmt`, `gofumpt`,
`goimports`, `golines`, and `swaggo` enabled. It defaults to `.` and accepts
CLI arguments after `--`:

```sh
task golangci-lint:fmt -- ./internal/...
```

Set `GOLANGCI_LINT_FMT_SKIP_PATTERN` to exclude matching Go file paths from
both `fmt` and `fmt:check`. The value is a shell-style path glob matched
against paths with forward slashes; quote it so your shell does not expand
it before Task receives it:

```sh
task golangci-lint:fmt GOLANGCI_LINT_FMT_SKIP_PATTERN="**/generated/**"
task golangci-lint:fmt:check GOLANGCI_LINT_FMT_SKIP_PATTERN="**/mocks/*.go"
```

An empty pattern keeps the default behavior. When a pattern is set,
formatter targets are expanded to `.go` files first and matching paths are
omitted.

## Versions

Leave `GOLANGCI_LINT_VERSION` empty to install the latest release, or set an
exact module version:

```sh
task golangci-lint:install GOLANGCI_LINT_VERSION=v2.1.6
```

Supplying a version forces the installer to run even when golangci-lint
already exists.

## Public Tasks

| Task          | Description                                            | Key variables                       |
| ------------- | ------------------------------------------------------- | ------------------------------------ |
| `install`     | Install golangci-lint into the global Go bin            | `GO_GLOBAL_BIN`, `GOLANGCI_LINT_VERSION` |
| `lint`        | Lint all Go packages with golangci-lint                 | `GOLANGCI_LINT_LINT_SKIP_PATTERN`    |
| `lint:fix`    | Auto-fix Go lint issues with golangci-lint               | `GOLANGCI_LINT_LINT_SKIP_PATTERN`    |
| `ci` | Run `fmt:check` then `lint` | — |
| `ci:fix` | Run `fmt` then `lint:fix` for CI | — |
| `fmt`         | Format Go files with golangci-lint formatters            | `GOLANGCI_LINT_FMT_SKIP_PATTERN`     |
| `fmt:check`   | Check Go formatting with golangci-lint formatters         | `GOLANGCI_LINT_FMT_SKIP_PATTERN`     |
| `config:skip` | Write the golangci-lint skip-pattern config overlay       | `GOLANGCI_LINT_LINT_SKIP_PATTERN`    |

## Variables

| Variable                             | Default                                   | Description                                                      |
| ------------------------------------- | ------------------------------------------ | ------------------------------------------------------------------ |
| `GOLANGCI_LINT_VERSION`               | empty (`latest`)                          | Optional golangci-lint module version                              |
| `GOLANGCI_LINT_LINT_SKIP_PATTERN`     | empty                                      | Shell-style path glob for Go files skipped by `lint` and `lint:fix` |
| `GOLANGCI_LINT_FMT_SKIP_PATTERN`      | empty                                      | Shell-style path glob for Go files skipped by `fmt` and `fmt:check` |
| `GOLANGCI_LINT_INTERNAL_SKIP_CONFIG`  | `.golangci-taskotter-skip.yml`             | Filename for the generated skip-pattern config overlay              |
| `GOLANGCI_LINT_FMT_FORMATTER_FLAGS`   | `-E gci -E gofmt -E gofumpt -E goimports -E golines -E swaggo` | Formatter set passed to `golangci-lint fmt`     |
| `GO_GLOBAL_BIN`                       | `GOBIN` or `GOPATH/bin`                    | Destination and lookup directory for the installed golangci-lint binary |
