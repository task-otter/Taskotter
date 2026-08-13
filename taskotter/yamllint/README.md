# yamllint Taskfile Public Tasks

## What is this Taskfile?

A cross-platform Taskfile for installing yamllint, managing upgrades, linting
YAML files, and generating a project configuration.

## Usage

### Standalone

```sh
task -t taskfiles/yamllint/Taskfile.yml install
task -t taskfiles/yamllint/Taskfile.yml config:init
task -t taskfiles/yamllint/Taskfile.yml ci
```

### Included

```yaml
includes:
  yamllint: ./taskfiles/yamllint/Taskfile.yml
```

Then run:

```sh
task yamllint:install
task yamllint:ci
```

## Public Tasks

| Task           | Description                                     | Key variables                     |
| -------------- | ----------------------------------------------- | --------------------------------- |
| `install`      | Install yamllint on the current OS if missing   | `YAMLLINT_VERSION`                |
| `install:undo` | Remove yamllint from the current OS             | none                              |
| `upgrade`      | Upgrade yamllint to the latest release          | none                              |
| `version`      | Show the installed yamllint version             | none                              |
| `ci`           | Strict lint for CI (fails on warnings)          | `YAMLLINT_TARGETS`, `YAMLLINT_CONFIG`, `YAMLLINT_EXTRA_ARGS` |
| `config:init`  | Create a default `.yamllint` configuration file | none                              |

## Variables

| Variable     | Default   | Description                                      |
| ------------ | --------- | ------------------------------------------------ |
| `YAMLLINT_TARGETS`    | `.`       | Files or directories to lint                     |
| `YAMLLINT_CONFIG`     | _(empty)_ | Path to a yamllint config file passed via `-c`   |
| `YAMLLINT_EXTRA_ARGS` | _(empty)_ | Extra flags forwarded to `yamllint` |
| `YAMLLINT_VERSION` | _(empty)_ | Pin a specific yamllint release for `install`/`upgrade`; empty installs latest |
| `YAMLLINT_LINT_SKIP_PATTERN` | _(empty)_ | Forward-slash path glob for files skipped by lint checks |

Skip patterns support `*` within one path segment, `**` across directories, and `?` for one character. Paths are matched relative to the task working directory; for example, `**/generated/**`.

## Notes

**`config:init`** writes a `.yamllint` file in the current directory and is
skipped if the file already exists. To regenerate, delete `.yamllint` first.

For YAML auto-formatting, use the separate [`yamlfix`](../yamlfix/README.md) module.
