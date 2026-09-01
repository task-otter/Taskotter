# golangci-lint Taskfile

## What is this Taskfile?

A cross-platform Taskfile for linting and formatting Go files with
golangci-lint. The `lint`, `fmt`, and related tasks auto-install Go and
golangci-lint via `nix:install:profile`.

## Usage

### Standalone

```sh
task -t taskfiles/golangci-lint/Taskfile.yml lint
task -t taskfiles/golangci-lint/Taskfile.yml lint:fix
task -t taskfiles/golangci-lint/Taskfile.yml fmt
task -t taskfiles/golangci-lint/Taskfile.yml fmt:check
```

Install only, without linting:

```sh
task nix:install:profile NIX_INSTALLABLE="nixpkgs#go nixpkgs#golangci-lint"
```

### Included

```yaml
includes:
  golangci-lint: ./taskfiles/golangci-lint/Taskfile.yml
```

Then run:

```sh
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

## Public Tasks

| Task          | Description                                            |
| ------------- | ------------------------------------------------------- |
| `lint`        | Lint all Go packages with golangci-lint                 |
| `lint:fix`    | Auto-fix Go lint issues with golangci-lint               |
| `ci`          | Run `fmt:check` then `lint` |
| `ci:fix`      | Run `fmt` then `lint:fix` for CI |
| `fmt`         | Format Go files with golangci-lint formatters            |
| `fmt:check`   | Check Go formatting with golangci-lint formatters         |
| `install` | Install golangci-lint via the Nix profile |
| `version` | Show the active golangci-lint version |

## Variables

| Variable                             | Default                                   | Description                                                      |
| ------------------------------------- | ------------------------------------------ | ------------------------------------------------------------------ |
| `GOLANGCI_LINT_NIX_INSTALLABLE`       | `nixpkgs#go nixpkgs#golangci-lint`         | Flake installables passed to `nix:install:profile` (Go plus the linter) |
| `GOLANGCI_LINT_FMT_FORMATTER_FLAGS`   | `-E gci -E gofmt -E gofumpt -E goimports -E golines -E swaggo` | Formatter set passed to `golangci-lint fmt`     |

Pin a revision by overriding the installable, for example
`GOLANGCI_LINT_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#golangci-lint`.

## Notes

- Install goes through `nix:install:profile` (Nix is installed first if missing). Native Windows is not supported; use WSL2.
- golangci-lint needs `go` on PATH, so the default installable includes both `nixpkgs#go` and `nixpkgs#golangci-lint`.
