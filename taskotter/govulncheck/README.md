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
| `install` | Install govulncheck via the Nix profile |
| `version` | Show the active govulncheck version |

## Variables

| Variable                        | Default                              | Description |
| --------------------------------- | -------------------------------------- | ----------- |
| `GOVULNCHECK_NIX_INSTALLABLE`    | `nixpkgs#go nixpkgs#govulncheck`      | Flake installables passed to `nix:install:profile` |

Pin a revision by overriding the installable, for example
`GOVULNCHECK_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#govulncheck`.

## Notes

- Install goes through `nix:install:profile` (Nix is installed first if missing). Native Windows is not supported; use WSL2.
- govulncheck needs `go` on PATH, so the default installable includes both `nixpkgs#go` and `nixpkgs#govulncheck`.
