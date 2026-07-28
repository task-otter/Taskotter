# jq Taskfile Public Tasks

## What is this Taskfile?

A cross-platform Taskfile for installing and running
[jq](https://jqlang.org), the lightweight command-line JSON processor.

jq is installed globally via the platform package manager — Homebrew on
macOS, apt-get or dnf on Linux, and Scoop on Windows. The install task is
skipped automatically when jq is already present in PATH.

## Usage

### Standalone

```sh
task -t taskfiles/jq/Taskfile.yml install
task -t taskfiles/jq/Taskfile.yml version
```

### Included

```yaml
includes:
  jq: ./taskfiles/jq/Taskfile.yml
```

Then run:

```sh
task jq:install
task jq:version
```

## Public Tasks

| Task           | Description                              | Key variables |
| -------------- | ---------------------------------------- | ------------- |
| `install`      | Install jq on the current OS            | `VERSION`     |
| `install:undo` | Remove jq from the current OS           | none          |
| `upgrade`      | Upgrade jq to the latest release        | none          |
| `version`      | Show the installed jq version           | none          |

## Notes

Pass `VERSION=x.y.z` to `install` to pin a specific release. Exact-version
availability depends on what the platform's package manager/repository
carries; when `VERSION` is empty (the default), the latest available
release is installed.

On Linux, jq is installed via `apt-get` if available, then `dnf`. If
neither package manager carries jq, install it manually by downloading the
binary from the [jq releases page](https://github.com/jqlang/jq/releases)
and placing it somewhere in your PATH.

On Windows, jq is installed via [Scoop](https://scoop.sh). Install Scoop
first if it is not available, then re-run this task.
