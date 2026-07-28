# Hadolint Taskfile Public Tasks

## What is this Taskfile?

A cross-platform Taskfile for installing and running
[hadolint](https://github.com/hadolint/hadolint), the Dockerfile linter.

hadolint is installed globally via the platform package manager — Homebrew on
macOS, apt-get or dnf on Linux, and Scoop on Windows. The install task is
skipped automatically when hadolint is already present in PATH.

## Usage

### Standalone

```sh
task -t taskfiles/hadolint/Taskfile.yml install
task -t taskfiles/hadolint/Taskfile.yml lint
task -t taskfiles/hadolint/Taskfile.yml version
```

Lint a specific Dockerfile:

```sh
task -t taskfiles/hadolint/Taskfile.yml lint DOCKERFILE=path/to/Dockerfile
```

Pass hadolint arguments after `--`:

```sh
task -t taskfiles/hadolint/Taskfile.yml lint -- path/to/Dockerfile --ignore DL3008
```

### Included

```yaml
includes:
  hadolint: ./taskfiles/hadolint/Taskfile.yml
```

Then run:

```sh
task hadolint:lint
task hadolint:lint DOCKERFILE=services/api/Dockerfile
task hadolint:version
```

## Public Tasks

| Task           | Description                                       | Key variables                        |
| -------------- | ------------------------------------------------- | ------------------------------------ |
| `install`      | Install hadolint on the current operating system  | none                                 |
| `install:undo` | Remove hadolint from the current operating system | none                                 |
| `lint`         | Lint a Dockerfile with hadolint                   | `DOCKERFILE`, `CONFIG`, `EXTRA_ARGS` |
| `upgrade`      | Upgrade hadolint to the latest release            | none                                 |
| `version`      | Show the installed hadolint version               | none                                 |

## Variables

| Variable     | Default      | Description                                            |
| ------------ | ------------ | ------------------------------------------------------ |
| `DOCKERFILE` | `Dockerfile` | Path to the Dockerfile to lint                         |
| `CONFIG`     | empty        | Path to a hadolint config file passed via `--config`   |
| `EXTRA_ARGS` | empty        | Extra arguments appended when CLI_ARGS is not provided |
| `HADOLINT_LINT_SKIP_PATTERN` | _(empty)_ | Forward-slash path glob for files skipped by lint checks and fixes |

Skip patterns support `*` within one path segment, `**` across directories, and `?` for one character. Paths are matched relative to the task working directory; for example, `**/generated/**`.

## Notes

On Linux, hadolint is installed via `apt-get` if available, then `dnf`. If
neither package manager carries hadolint (e.g. older Ubuntu releases), install
it manually by downloading the binary from the
[hadolint releases page](https://github.com/hadolint/hadolint/releases) and
placing it somewhere in your PATH.
