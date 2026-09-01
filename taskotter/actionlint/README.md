# actionlint

A [TaskOtter](https://github.com/task-otter/store) module for [actionlint](https://github.com/rhysd/actionlint) — a static checker for GitHub Actions workflow files.

## What is this Taskfile?

This module lints GitHub Actions workflow files with [actionlint](https://github.com/rhysd/actionlint). actionlint statically checks workflows for syntax errors, type mismatches in expressions, incorrect event payloads, and more. The `ci` task auto-installs actionlint via `nix:install:profile`.

## Usage

### Standalone

```sh
task -t taskfiles/actionlint/Taskfile.yml ci
task -t taskfiles/actionlint/Taskfile.yml ci ACTIONLINT_TARGETS=.github/workflows/ci.yml
task -t taskfiles/actionlint/Taskfile.yml ci ACTIONLINT_EXTRA_ARGS="-ignore 'label.*'"
```

Install only, without linting:

```sh
task nix:install:profile NIX_INSTALLABLE=nixpkgs#actionlint
```

### Included in your Taskfile

```yaml
includes:
  actionlint:
    taskfile: taskfiles/actionlint/Taskfile.yml
```

Then run:

```sh
task actionlint:ci
```

## Public Tasks

| Task | Description |
|---|---|
| `ci` | Lint GitHub Actions workflow files with actionlint |
| `install` | Install actionlint via the Nix profile |
| `version` | Show the active actionlint version |

## Variables

| Variable | Default | Description |
|---|---|---|
| `ACTIONLINT_NIX_INSTALLABLE` | `nixpkgs#actionlint` | Flake installable passed to `nix:install:profile` |
| `ACTIONLINT_EXTRA_ARGS` | `""` | Additional flags passed to `actionlint` (e.g. `-ignore`, `-format`) |
| `ACTIONLINT_TARGETS` | `""` | Paths to workflow files; empty = auto-discover `.github/workflows` |

Pin a revision by overriding the installable, for example
`ACTIONLINT_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#actionlint`.

## Notes

- Install goes through `nix:install:profile` (Nix is installed first if missing). Native Windows is not supported; use WSL2.
- When `ACTIONLINT_TARGETS` is empty, actionlint automatically discovers all files under `.github/workflows/` in the current working directory.
- The `ci` task auto-installs actionlint if it is not already present in `PATH`.
