# govulncheck Taskfile

## What is this Taskfile?

A cross-platform Taskfile for scanning Go packages for known vulnerabilities
with govulncheck. The `ci` task auto-installs Go and govulncheck via
`nix:install:profile`.

## Usage

### Standalone

```sh
task -t taskfiles/govulncheck/Taskfile.yml ci
```

Install only, without scanning:

```sh
task nix:install:profile NIX_INSTALLABLE="nixpkgs#go nixpkgs#govulncheck"
```

### Included

```yaml
includes:
  govulncheck: ./taskfiles/govulncheck/Taskfile.yml
```

Then run:

```sh
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

## Public Tasks

| Task | Description |
| ---- | ----------- |
| `ci` | Scan Go packages for known vulnerabilities |
| `install` | Install govulncheck via Nix (Unix) or go install (Windows) |
| `version` | Show the active govulncheck version |

## Variables

| Variable                        | Default                                        | Description |
| --------------------------------- | ---------------------------------------------- | ----------- |
| `GOVULNCHECK_NIX_INSTALLABLE`    | `nixpkgs#go nixpkgs#govulncheck`              | Flake installables passed to `nix:install:profile` |
| `GOVULNCHECK_GO_PKG`             | `golang.org/x/vuln/cmd/govulncheck@latest`    | Go module passed to `go:install:pkg` on Windows |

Pin a revision by overriding the installable, for example
`GOVULNCHECK_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#govulncheck`.

## Notes

- Unix install uses Nix (`GOVULNCHECK_NIX_INSTALLABLE`). On Windows, install uses `go:install:pkg` with `GOVULNCHECK_GO_PKG`.
- On Unix the default Nix installable includes both `nixpkgs#go` and `nixpkgs#govulncheck`. On Windows, Go comes from the included go Taskfile.
