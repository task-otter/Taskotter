# pnpm Taskfile Public Tasks

This Taskfile wraps common pnpm project workflows while loading Node.js through a
**node version manager**. pnpm commands run through Corepack so the project's
`packageManager` field controls the pnpm version. The runners set
`COREPACK_ENABLE_AUTO_PIN=1`, so first use adds the field when it is missing.

## Variants

pnpm ships as two variants, one per node version manager. Both expose exactly the
same tasks; pick the one matching your stack.

| Variant | Node version manager | Taskfile                          |
| ------- | -------------------- | --------------------------------- |
| `fnm`   | [fnm](../fnm/)       | `taskfiles/pnpm/fnm/Taskfile.yml` |
| `nvm`   | [nvm](../nvm/)       | `taskfiles/pnpm/nvm/Taskfile.yml` |

## Setup

Include the family and address a variant through its namespace:

```yaml
includes:
  pnpm: taskfiles/pnpm/Taskfile.yml
```

```bash
task pnpm:fnm:install
task pnpm:nvm:install
```

Or include a single variant directly to keep the task names short (`task pnpm:install`).
Swap `fnm` for `nvm` throughout to use the nvm stack:

```yaml
includes:
  fnm:
    taskfile: taskfiles/fnm/Taskfile.yml
  corepack:
    taskfile: taskfiles/corepack/fnm/Taskfile.yml
  pnpm:
    taskfile: taskfiles/pnpm/fnm/Taskfile.yml
```

## Public Tasks

| Task              | Variables                              | Description                                                 |
| ----------------- | -------------------------------------- | ----------------------------------------------------------- |
| `add`             | Required `PACKAGES`; optional `EXTRA_ARGS` | Add packages as devDependencies with `pnpm add -D`.         |
| `node:setup`      | Optional `NODE_VERSION`                | Install Node.js via fnm or nvm.                             |
| `manager:setup`   | Optional `NODE_VERSION`                | Install Corepack when needed and enable its shims.          |
| `manager:pin`     | Required `PACKAGE_MANAGER_VERSION`     | Pin pnpm in `package.json` with Corepack.                   |
| `version`         | Optional `NODE_VERSION`                | Show active Node.js and pnpm versions.                      |
| `install`         | Optional `NODE_VERSION`                | Run `pnpm install`.                                         |
| `install:undo`    | —                                       | Disable the pnpm Corepack shim.                              |
| `upgrade`         | —                                       | Upgrade pnpm via `corepack prepare pnpm@latest --activate`. |
| `remove`          | Required `PACKAGES`; optional `EXTRA_ARGS` | Remove local packages with `pnpm remove`.                   |
| `ci`              | Optional `NODE_VERSION`                | Run `pnpm install --frozen-lockfile` with `pnpm-lock.yaml`. |
| `ci:fix` | — | Run `fmt` for CI fixing |
| `run`             | Required `SCRIPT`; optional `NODE_VERSION` | Run a script via `pnpm run`.                                |
| `dev`             | Optional `NODE_VERSION`                | Run `pnpm run dev`.                                         |
| `exec`            | Required `BINARY`; optional `ARGS`, `EXTRA_ARGS` | Execute a local project binary via `pnpm exec`.             |
| `test`            | Optional `NODE_VERSION`                | Run `pnpm test`.                                            |
| `build`           | Optional `NODE_VERSION`                | Run `pnpm run build`.                                       |
| `lint`            | Optional `NODE_VERSION`                | Run `pnpm run lint`.                                        |
| `fmt`          | Optional `NODE_VERSION`                | Run `pnpm run format`.                                      |
| `typecheck`       | Optional `NODE_VERSION`                | Run `pnpm run typecheck`.                                   |
| `outdated`        | Optional `NODE_VERSION`                | List outdated packages without failing.                     |
| `outdated:strict` | Optional `NODE_VERSION`                | List outdated packages with the pnpm exit code.             |
| `update`          | Optional `NODE_VERSION`                | Run `pnpm update`.                                          |
| `audit`           | Optional `NODE_VERSION`                | Run strict `pnpm audit`.                                    |
| `audit:report`    | Optional `NODE_VERSION`                | Report audit findings without failing.                      |
| `audit:fix`       | Optional `NODE_VERSION`                | Run `pnpm audit --fix`.                                     |
| `audit:json`      | Optional `NODE_VERSION`                | Emit `pnpm audit --json`.                                   |
| `store:prune`     | Optional `NODE_VERSION`                | Remove unreferenced pnpm store packages.                    |
| `clean`           | -                                      | Remove `node_modules`.                                      |
| `clean:all`       | -                                      | Remove `node_modules` and `pnpm-lock.yaml`.                 |

## Examples

```bash
task pnpm:node:setup NODE_VERSION=22
task pnpm:manager:setup
task pnpm:manager:pin PACKAGE_MANAGER_VERSION=latest
task pnpm:install
task pnpm:ci
task pnpm:run SCRIPT=test -- --watch
task pnpm:audit:report
task pnpm:store:prune
task pnpm:clean --yes
```
