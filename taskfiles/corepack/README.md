# Corepack Taskfile Public Tasks

Corepack keeps Yarn and pnpm versions tied to a project's `packageManager`
field and can provide package manager shims for the Node.js runtime managed by
your node version manager. Corepack itself may need installation on Node.js
releases that do not bundle it.

## Variants

Corepack ships as two variants, one per node version manager. Both expose exactly
the same tasks; pick the one matching your stack.

| Variant | Node version manager | Taskfile                              |
| ------- | -------------------- | ------------------------------------- |
| `fnm`   | [fnm](../fnm/)       | `taskfiles/corepack/fnm/Taskfile.yml` |
| `nvm`   | [nvm](../nvm/)       | `taskfiles/corepack/nvm/Taskfile.yml` |

Include the family and address a variant through its namespace:

```yaml
includes:
  corepack: taskfiles/corepack/Taskfile.yml
```

```bash
task corepack:fnm:setup
task corepack:nvm:setup
```

Or include a single variant alongside its node version manager to keep the task
names short (`task corepack:setup`). Swap `fnm` for `nvm` to use the nvm stack:

```yaml
includes:
  fnm:
    taskfile: taskfiles/fnm/Taskfile.yml
  corepack:
    taskfile: taskfiles/corepack/fnm/Taskfile.yml
```

## Public Tasks

| Task          | Variables                              | Description                                                         |
| ------------- | -------------------------------------- | ------------------------------------------------------------------- |
| `node:setup`  | Optional `NODE_VERSION`                | Install Node.js via fnm or nvm.                                     |
| `install`     | Optional `NODE_VERSION`, `COREPACK_VERSION` | Install Corepack through npm when the active Node runtime lacks it. |
| `install:undo`| Optional `NODE_VERSION`                | Remove Corepack installed through npm.                              |
| `upgrade`     | Optional `NODE_VERSION`, `COREPACK_VERSION` | Reinstall Corepack at the pinned COREPACK_VERSION.                  |
| `setup`       | Optional `NODE_VERSION`, `COREPACK_VERSION` | Install Corepack when needed and enable its shims.                  |
| `version`     | Optional `NODE_VERSION`                | Show the active Corepack version.                                   |
| `enable`      | Optional `NODE_VERSION`                | Enable Corepack shims.                                              |
| `disable`     | Optional `NODE_VERSION`                | Disable Corepack shims.                                             |
| `use`         | Required `PACKAGE_MANAGER`, `COREPACK_VERSION`  | Pin `npm`, `pnpm`, or `yarn` in the current `package.json`.         |
| `cache:clean` | Optional `NODE_VERSION`                | Clear cached package manager archives.                              |

## Examples

```bash
task corepack:setup
task corepack:install COREPACK_VERSION=latest
task corepack:use PACKAGE_MANAGER=yarn COREPACK_VERSION=stable
task corepack:use PACKAGE_MANAGER=pnpm COREPACK_VERSION=10.0.0
task corepack:cache:clean
```

`COREPACK_VERSION` defaults to `0.34.0` for reproducible CI/tooling. Override it
when you intentionally want another release.
