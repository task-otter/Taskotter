# pnpm Taskfile Public Tasks

## What is this Taskfile?

This Taskfile wraps common `pnpm` operations — installing dependencies, running scripts, auditing, and cleaning — behind consistent, cross-platform task commands. Node.js is provided via the [`nodejs`](../nodejs/) module (Nix profile); pnpm is installed from nixpkgs through this module's `install:tool` task.

## Usage

```yaml
includes:
  nodejs:
    taskfile: taskfiles/nodejs/Taskfile.yml
  pnpm:
    taskfile: taskfiles/pnpm/Taskfile.yml
```

```bash
task pnpm:install
task pnpm:install:clean
task pnpm:run SCRIPT=build
```

Override the Node.js version by setting `NODEJS_NIX_INSTALLABLE` on the nodejs module before dependents run `nodejs:install`.

## Public Tasks

| Task              | Variables                                  | Description                                                               |
| ----------------- | ------------------------------------------ | ------------------------------------------------------------------------- |
| `add`             | Required `PACKAGES`; optional `EXTRA_ARGS` | Add packages as devDependencies with `pnpm add -D`.                       |
| `version`         | —                                          | Show the active Node.js and pnpm versions.                                |
| `install`         | —                                          | Run `pnpm install` to install all dependencies from `package.json`.       |
| `install:undo`    | —                                          | Explain how to remove the pnpm Nix profile installable.                   |
| `upgrade`         | —                                          | Upgrade pnpm via the configured Nix installable.                          |
| `install:clean`   | —                                          | Run `pnpm install --frozen-lockfile` with `pnpm-lock.yaml`.               |
| `ci:fix`          | —                                          | Run `pnpm run format`.                                                    |
| `clean`           | —                                          | Remove `node_modules`.                                                    |
| `clean:all`       | —                                          | Remove `node_modules` and `pnpm-lock.yaml`.                               |
| `dev`             | —                                          | Run `pnpm run dev`.                                                       |
| `build`           | —                                          | Run `pnpm run build`.                                                     |
| `test`            | —                                          | Run `pnpm test`.                                                          |
| `lint`            | —                                          | Run `pnpm run lint`.                                                      |
| `typecheck`       | —                                          | Run `pnpm run typecheck`.                                                 |
| `run`             | Required `SCRIPT`; optional CLI args       | Run a `package.json` script.                                              |
| `exec`            | Required `BINARY`                          | Execute a local binary via `pnpm exec`.                                   |
| `remove`          | Required `PACKAGES`                        | Uninstall packages.                                                       |
| `outdated`        | —                                          | List outdated packages (non-strict).                                      |
| `outdated:strict` | —                                          | List outdated packages and fail if any exist.                             |
| `audit`           | —                                          | Run `pnpm audit` (strict).                                                |
| `audit:report`    | —                                          | Run `pnpm audit` without failing.                                         |
| `audit:fix`       | —                                          | Run `pnpm audit --fix`.                                                   |
| `audit:json`      | —                                          | Output audit results as JSON.                                             |
| `update`          | —                                          | Update packages within declared ranges.                                   |
| `store:prune`     | —                                          | Remove unreferenced packages from the pnpm store.                         |
| `install:tool`    | —                                          | Install the pnpm binary via the Nix profile.                              |
| `version:tool`    | —                                          | Show the version of the pnpm binary itself.                               |

## Runtime

Project commands depend on `nodejs:install` and run `pnpm` with `NIX_LOAD` so the Nix profile tools are on PATH. Must be run from a directory containing `package.json`.

Native Windows auto-install requires WSL2 (same as other Nix profile modules).
