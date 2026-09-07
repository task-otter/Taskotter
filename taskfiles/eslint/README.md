# ESLint

## What is this Taskfile?

This Taskfile wraps ESLint for JavaScript and TypeScript projects. It installs
ESLint as a local dev dependency, runs cached checks by default, supports strict
CI mode.

## Variants

Every runtime + package-manager combination ships as its own leaf Taskfile under
`taskfiles/eslint/`. They all expose the identical public interface documented
below; only the underlying runtime and package manager differ. Include the tool
family once in your root Taskfile:

```yaml
includes:
  eslint: taskfiles/eslint/Taskfile.yml
```

Then run the leaf that matches your project through its namespace (replace
`{TASK}` with any public task):

```bash
task eslint:bun:{TASK}                 # Bun runtime + Bun as package manager
task eslint:node:npm:{TASK}        # Node via the nodejs module, npm as package manager
task eslint:node:pnpm:{TASK}       # Node via the nodejs module, pnpm as package manager
```

Available leaves: `bun`, `node/{npm,pnpm,yarn}`.

## Public Tasks

| Task          | Variables                                                             | Description                                                                              |
| ------------- | --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `install`     | Optional `ESLINT_VERSION`, `ESLINT_EXTRA_ARGS`, `CLI_ARGS`                    | Install `eslint` as a local dev dependency. Pass `ESLINT_VERSION=x.y.z` to pin a release. |
| `install:undo`| Optional `ESLINT_EXTRA_ARGS`                                           | Remove the locally installed `eslint` devDependency.                                     |
| `upgrade`     | Optional `ESLINT_EXTRA_ARGS`                                           | Reinstall `eslint` at the latest version.                                                |
| `config:init` | —                                                               | Write a starter `eslint.config.mjs` when none exists (non-interactive). Skipped if a recognized config file already exists. |
| `ci`          | Optional `ESLINT_TARGETS`, `ESLINT_CONFIG`, `ESLINT_CACHE`, `ESLINT_EXTRA_ARGS`, `CLI_ARGS` | Run ESLint with `--max-warnings=0`.                                                      |
| `ci:fix` | Optional `ESLINT_TARGETS`, `ESLINT_CONFIG`, `ESLINT_CACHE`, `ESLINT_EXTRA_ARGS`, `CLI_ARGS` | Run ESLint with `--fix`. |
| `cache:clean` | —                                                                     | Remove `.cache/eslint`.                                                                  |
| `version`     | — | Show the resolved ESLint version.                                                        |
| `help`        | Optional `ESLINT_EXTRA_ARGS`, `CLI_ARGS`                               | Show ESLint CLI help.                                                                    |

## Variables

`ESLINT_TARGETS` defaults to `src/**/*.{js,jsx,ts,tsx}`. `ESLINT_CONFIG` adds
`--config <path>`. `ESLINT_CACHE` defaults to `true`; set `ESLINT_CACHE=false` to omit cache
flags. `ESLINT_EXTRA_ARGS` and arguments after `--` are appended to the command.

## Examples

```bash
task eslint:node:npm:install
task eslint:node:npm:install ESLINT_VERSION=8.57.0
task eslint:node:npm:ci
task eslint:node:npm:ci:fix ESLINT_TARGETS="src test"
```
