# Hadolint Taskfile Public Tasks

## What is this Taskfile?

A cross-platform Taskfile for installing and running
[hadolint](https://github.com/hadolint/hadolint), the Dockerfile linter.

hadolint is installed globally via a pinned GitHub release binary to
`/usr/local/bin` on macOS and Linux, and Scoop on Windows. The install task is
skipped automatically when hadolint is already present in PATH at the pinned
version.

## Usage

### Standalone

```sh
task -t taskfiles/hadolint/Taskfile.yml install
task -t taskfiles/hadolint/Taskfile.yml ci
task -t taskfiles/hadolint/Taskfile.yml version
```

Lint a specific Dockerfile:

```sh
task -t taskfiles/hadolint/Taskfile.yml ci HADOLINT_DOCKERFILE=path/to/Dockerfile
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
task hadolint:version
```

## Public Tasks

| Task           | Description                                       | Key variables                        |
| -------------- | ------------------------------------------------- | ------------------------------------ |
| `install`      | Install hadolint on the current operating system  | none                                 |
| `install:undo` | Remove hadolint from the current operating system | none                                 |
| `ci`         | Lint a Dockerfile with hadolint                   | `HADOLINT_DOCKERFILE`, `HADOLINT_CONFIG`, `HADOLINT_EXTRA_ARGS` |
| `upgrade`      | Upgrade hadolint to the latest release            | none                                 |
| `version`      | Show the installed hadolint version               | none                                 |

## Variables

| Variable     | Default      | Description                                            |
| ------------ | ------------ | ------------------------------------------------------ |
| `HADOLINT_DOCKERFILE` | `Dockerfile` | Path to the Dockerfile to lint                         |
| `HADOLINT_CONFIG`     | empty        | Path to a hadolint config file passed via `--config`   |
| `HADOLINT_EXTRA_ARGS` | empty        | Extra arguments appended when CLI_ARGS is not provided |
| `HADOLINT_LINT_SKIP_PATTERN` | _(empty)_ | Forward-slash path glob for files skipped by lint checks and fixes |

Skip patterns support `*` within one path segment, `**` across directories, and `?` for one character. Paths are matched relative to the task working directory; for example, `**/generated/**`.

## Notes

On macOS and Linux, hadolint is installed from the pinned `HADOLINT_VERSION`
GitHub release binary into `/usr/local/bin` (`x86_64` and `arm64`). Other
architectures require a manual install from the
[hadolint releases page](https://github.com/hadolint/hadolint/releases).
macOS `upgrade` still uses Homebrew (`brew upgrade hadolint`).
