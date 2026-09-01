# Hadolint Taskfile Public Tasks

## What is this Taskfile?

A Taskfile for running [hadolint](https://github.com/hadolint/hadolint), the
Dockerfile linter. The `ci` task auto-installs hadolint via `nix:install:profile`.

## Usage

### Standalone

```sh
task -t taskfiles/hadolint/Taskfile.yml ci
task -t taskfiles/hadolint/Taskfile.yml ci HADOLINT_DOCKERFILE=path/to/Dockerfile
```

Install only, without linting:

```sh
task -t taskfiles/hadolint/Taskfile.yml install
```

Pass hadolint arguments after `--`:

```sh
task -t taskfiles/hadolint/Taskfile.yml ci -- path/to/Dockerfile --ignore DL3008
```

### Included

```yaml
includes:
  hadolint: ./taskfiles/hadolint/Taskfile.yml
```

Then run:

```sh
task hadolint:ci
task hadolint:ci HADOLINT_DOCKERFILE=services/api/Dockerfile
```

## Public Tasks

| Task | Description                     | Key variables                                              |
| ---- | ------------------------------- | ---------------------------------------------------------- |
| `ci` | Lint a Dockerfile with hadolint | `HADOLINT_DOCKERFILE`, `HADOLINT_CONFIG`, `HADOLINT_EXTRA_ARGS` |
| `install` | Install hadolint via the Nix profile | `HADOLINT_NIX_INSTALLABLE` |
| `version` | Show the active hadolint version | — |

## Variables

| Variable                     | Default      | Description                                            |
| ---------------------------- | ------------ | ------------------------------------------------------ |
| `HADOLINT_NIX_INSTALLABLE`   | `nixpkgs#hadolint` | Flake installable passed to `nix:install:profile` |
| `HADOLINT_DOCKERFILE`        | `Dockerfile` | Path to the Dockerfile to lint                         |
| `HADOLINT_CONFIG`            | empty        | Path to a hadolint config file passed via `--config`   |
| `HADOLINT_EXTRA_ARGS`        | empty        | Extra arguments appended when CLI_ARGS is not provided |

Pin a revision by overriding the installable, for example
`HADOLINT_NIX_INSTALLABLE=github:NixOS/nixpkgs/<rev>#hadolint`.

## Notes

- Install goes through `nix:install:profile` (Nix is installed first if missing). Native Windows is not supported; use WSL2.
- The `ci` task auto-installs hadolint if it is not already present in `PATH`.
