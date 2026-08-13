# yamlfix

A [TaskOtter](https://github.com/task-otter/store) module for [yamlfix](https://github.com/lyz-code/yamlfix), a YAML formatter and auto-fixer.

## What is this Taskfile?

This module installs `yamlfix` via [uv](https://docs.astral.sh/uv/) into an isolated tool environment, then formats YAML files in place. It excludes `Taskfile.yml` / `Taskfile.yaml` whose Go template syntax is incompatible with yamlfix.

## Usage

### Standalone

```sh
task -t taskfiles/yamlfix/Taskfile.yml install
task -t taskfiles/yamlfix/Taskfile.yml ci:fix YAMLFIX_TARGETS=.
task -t taskfiles/yamlfix/Taskfile.yml version
```

### Included in your Taskfile

```yaml
includes:
  yamlfix:
    taskfile: taskfiles/yamlfix/Taskfile.yml
```

Then run:

```sh
task yamlfix:ci:fix
task yamlfix:install YAMLFIX_VERSION=1.17.0
```

## Public Tasks

| Task | Description | Key variables |
|---|---|---|
| `install` | Install yamlfix on the current OS | `YAMLFIX_VERSION` |
| `install:undo` | Remove yamlfix from the current OS | none |
| `upgrade` | Upgrade yamlfix to the latest release | `YAMLFIX_VERSION` |
| `version` | Show the installed yamlfix version | none |
| `ci:fix` | Auto-fix YAML files with yamlfix | `YAMLFIX_TARGETS`, `YAMLFIX_EXTRA_ARGS` |

## Variables

| Variable | Default | Description |
|---|---|---|
| `YAMLFIX_TARGETS` | `.` | Files or directories to format |
| `YAMLFIX_EXTRA_ARGS` | _(empty)_ | Extra flags forwarded to `yamlfix` |
| `YAMLFIX_VERSION` | _(empty)_ | Pin a specific yamlfix release for `install`/`upgrade`; empty installs latest |
| `YAMLFIX_FMT_SKIP_PATTERN` | _(empty)_ | Forward-slash path glob passed to yamlfix `--exclude` |
| `UV_LOAD` | `export PATH="$HOME/.local/bin:$PATH"` | Shell snippet that puts uv-managed tools on PATH (unix) |

Skip patterns support `*` within one path segment, `**` across directories, and `?` for one character. Paths are matched relative to the task working directory; for example, `**/generated/**`.

## Notes

- `ci:fix` skips `Taskfile.yml` and `Taskfile.yaml` because Go template syntax breaks yamlfix.
