# ESLint Taskfile (node/fnm/pnpm) Public Tasks

## What is this Taskfile?

This Taskfile wraps ESLint for JavaScript and TypeScript projects. It installs
ESLint as a local dev dependency, runs cached checks by default, supports strict
CI mode.

This variant uses the `node/fnm/pnpm` stack.


## Setup

```yaml
includes:
  pnpm:
    taskfile: taskfiles/pnpm/Taskfile.yml
  eslint:
    taskfile: taskfiles/eslint/Taskfile.yml
```

## Public Tasks

| Task          | Variables                                                             | Description                                                                              |
| ------------- | --------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| `install`     | Optional `VERSION`, `EXTRA_ARGS`, `CLI_ARGS`                    | Install `eslint` as a local dev dependency. Pass `VERSION=x.y.z` to pin a release. |
| `install:undo`| Optional `EXTRA_ARGS`                                           | Remove the locally installed `eslint` devDependency.                                     |
| `upgrade`     | Optional `EXTRA_ARGS`                                           | Reinstall `eslint` at the latest version.                                                |
| `init`        | Optional `EXTRA_ARGS`, `CLI_ARGS`                               | Alias for `config:init`.                                                                 |
| `config:init` | Optional `EXTRA_ARGS`, `CLI_ARGS`                               | Run the ESLint configuration wizard. Skipped if a recognized config file already exists. |
| `lint`        | Optional `TARGETS`, `CONFIG`, `CACHE`, `EXTRA_ARGS`, `CLI_ARGS` | Lint targets with cache enabled by default.                                              |
| `lint:fix`    | Optional `TARGETS`, `CONFIG`, `CACHE`, `EXTRA_ARGS`, `CLI_ARGS` | Run ESLint with `--fix`.                                                                 |
| `ci`          | Optional `TARGETS`, `CONFIG`, `CACHE`, `EXTRA_ARGS`, `CLI_ARGS` | Run ESLint with `--max-warnings=0`.                                                      |
| `cache:clean` | —                                                                     | Remove `.cache/eslint`.                                                                  |
| `version`     | — | Show the resolved ESLint version.                                                        |
| `help`        | Optional `EXTRA_ARGS`, `CLI_ARGS`                               | Show ESLint CLI help.                                                                    |

## Variables

`TARGETS` defaults to `src/**/*.{js,jsx,ts,tsx}`. `CONFIG` adds
`--config <path>`. `CACHE` defaults to `true`; set `CACHE=false` to omit cache
flags. `EXTRA_ARGS` and arguments after `--` are appended to the command.

## Examples

```bash
task eslint:install
task eslint:install VERSION=8.57.0
task eslint:lint
task eslint:lint:fix TARGETS="src test"
```
