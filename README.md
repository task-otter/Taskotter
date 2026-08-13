<p align="center">
  <img src="logo/image.png" alt="TaskOtter logo" width="240">
</p>

# TaskOtter

[![LICENSE](https://img.shields.io/github/license/task-otter/Taskotter)](/LICENSE) [![codecov](https://codecov.io/gh/task-otter/Taskotter/graph/badge.svg)](https://codecov.io/gh/task-otter/Taskotter)

Docker-based GitHub Action that synchronizes task modules from the [TaskOtter store](https://github.com/task-otter/store) into your repository, resolves transitive dependencies, normalizes destination folder names, optionally updates your root `Taskfile.yml`, and opens or updates a deterministic pull request when changes exist.

Sync pull requests target the branch that invoked TaskOtter. For pull-request workflows, they target that workflow's base branch.

## Features

- Validates all inputs before modifying files
- Resolves logical task names to store variants (`eslint` + `pnpm` + `fnm` → `eslint/node/fnm/pnpm`)
- Recursively resolves dependencies from `.deps.yml`, including namespaced support modules (`internal/skipfiles` → `taskfiles/internal/skipfiles`)
- Supports latest default branch or a pinned store Git tag
- Normalizes destination directories (`eslint/node/fnm/pnpm` → `taskfiles/eslint`)
- Rewrites internal Taskfile dependency includes to normalized paths
- Tracks managed files in `<target-folder>/.taskotter-lock.yml`
- Creates or updates a deterministic PR branch `taskotter/sync-<configuration-hash>`
- Never executes downloaded Taskfiles or scripts

## Permissions

```yaml
permissions:
  contents: write
  pull-requests: write
```

## Inputs

| Input | Required | Default | Description |
|-------|----------|---------|-------------|
| `tasks` | yes | — | Comma-separated or multiline list of logical task names (`eslint`, `go`, …) |
| `github-token` | yes | — | Token used to authenticate git push (origin URL) and manage pull requests |
| `js` | no | empty | YAML block for Node task resolution (see below) |
| `includes-doc` | no | `true` | Copy `README.md` and `docs/` when `true` |
| `sync-root` | no | `true` | Create or update the repository root `Taskfile.yml` when `true` |
| `store-version` | no | empty | Store Git tag (for example `v1.4.0`); empty uses default branch HEAD |
| `target-folder` | no | `taskfiles` | Repository-relative destination directory |
| `fail-on-changes` | no | `false` | Exit with failure after opening or updating a sync pull request (for CI drift checks) |

### `js` input

Optional YAML block for resolving Node.js task variants. Omit when syncing non-Node tasks only.

| Field | When | Default | Allowed values |
|-------|------|---------|----------------|
| `runtime` | always | `nodejs` | `nodejs`, `bun` |
| `package-manager` | `runtime: nodejs` | `npm` | `npm`, `yarn`, `pnpm` |
| `version-manager` | `runtime: nodejs` | `fnm` | `fnm`, `nvm` |

```yaml
js: |
  runtime: nodejs
  package-manager: pnpm
  version-manager: fnm
```

```yaml
js: |
  runtime: bun
```

## Outputs

| Output | Description |
|--------|-------------|
| `changed` | `true` when changes were generated |
| `store-version` | Requested store tag, or empty for latest default branch |
| `source-ref` | Resolved source reference (`refs/tags/...` or `refs/heads/...`) |
| `source-sha` | Immutable source commit SHA |
| `target-folder` | Normalized target folder |
| `resolved-tasks` | JSON map of requested tasks with source/destination modules |
| `resolved-dependencies` | JSON array of dependency modules |
| `pull-request-number` | Created or updated PR number |
| `pull-request-url` | Created or updated PR URL |

## Usage

### Latest store version

```yaml
name: TaskOtter

on:
  workflow_dispatch:
  schedule:
    - cron: "0 6 * * 1"

permissions:
  contents: write
  pull-requests: write

jobs:
  sync:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: TaskOtter
        id: taskotter
        uses: task-otter/Taskotter@v1
        with:
          github-token: ${{ github.token }}
          tasks: |
            eslint
            prettier
            typescript
            go
          js: |
            runtime: nodejs
            package-manager: pnpm
            version-manager: fnm
          includes-doc: true
          target-folder: taskfiles
```

### CI drift check

Use `fail-on-changes: true` in jobs that should block when TaskOtter opens a sync pull request. The action exits with a clear `::error` annotation and fails the job until the PR is merged.

```yaml
jobs:
  taskotter:
    runs-on: ubuntu-latest
    permissions:
      contents: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v6
        with:
          fetch-depth: 0

      - name: TaskOtter
        uses: task-otter/Taskotter@v1
        with:
          github-token: ${{ github.token }}
          fail-on-changes: true
          tasks: |
            go
            actionlint
          target-folder: taskfiles

  test:
    needs: taskotter
    runs-on: ubuntu-latest
    steps:
      - run: echo "runs only when taskfiles are in sync"
```

### Pinned store tag

```yaml
- uses: task-otter/Taskotter@v1
  with:
    github-token: ${{ github.token }}
    store-version: v1.4.0
    target-folder: automation/taskfiles
    tasks: |
      eslint
      prettier
      go
    js: |
      runtime: nodejs
      package-manager: pnpm
      version-manager: fnm
```

### Bun runtime

```yaml
- uses: task-otter/Taskotter@v1
  with:
    github-token: ${{ github.token }}
    tasks: |
      eslint
      prettier
      typescript
    js: |
      runtime: bun
    includes-doc: false
```

### Non-Node tasks only

```yaml
- uses: task-otter/Taskotter@v1
  with:
    github-token: ${{ github.token }}
    target-folder: build/taskfiles
    tasks: |
      actionlint
      shellcheck
      docker
      go
```

## Prerequisites

With the default `sync-root: true`, TaskOtter creates a root `Taskfile.yml` when one is missing, then adds managed `includes` entries for synced modules. If a root Taskfile already exists, TaskOtter updates only its managed includes and leaves other content unchanged. Set `sync-root: false` to synchronize modules and TaskOtter state without reading, creating, or modifying the root `Taskfile.yml`.

When synced modules share exported task names in store metadata, TaskOtter also generates root tasks that fan out to those modules. For example, syncing `go` and `eslint` can generate root `lint`, `lint:fix`, `install`, and `version` tasks that call `go:lint`, `eslint:lint`, and so on. Shared module variables are promoted to editable root `vars`, while include-level vars reference those root values.

## Behavior

### Module resolution

- Non-Node tasks (`go`, `docker`, `shellcheck`, …) resolve directly by name
- Node tasks require the `js` input
- `runtime: nodejs` uses `package-manager` (default `npm`) and `version-manager` (default `fnm`)
- `runtime: bun` ignores `package-manager` and `version-manager`

### Destination layout

Modules copy to `<target-folder>/<normalized-name>/`:

```text
taskfiles/eslint/Taskfile.yml      ← eslint/node/fnm/pnpm
taskfiles/pnpm/Taskfile.yml        ← pnpm/fnm
taskfiles/corepack/Taskfile.yml    ← corepack/fnm
taskfiles/fnm/Taskfile.yml         ← fnm
```

### Lock file and metadata

- Lock file: `<target-folder>/.taskotter-lock.yml`
- Metadata: `<target-folder>/.taskotter/metadata.yml`

### Pull requests

- Branch: `taskotter/sync-<configuration-hash>`
- Title: `chore(taskotter): sync taskfiles`
- Updates an existing open PR for the same branch instead of creating duplicates
- No commit or PR when synchronized content is unchanged

## Validation failures

| Condition | Result |
|-----------|--------|
| Empty or invalid task names | Fail before download |
| Missing store task or variant | Fail with close matches |
| Bun + version manager | Fail |
| npm/yarn/pnpm without version manager | Fail |
| Node task without package manager | Fail |
| Invalid or unsafe `store-version` | Fail |
| Unsafe `target-folder` | Fail |
| Missing root `Taskfile.yml` with `sync-root: true` | Create minimal root Taskfile and include in PR |
| Unmanaged existing destination directory | Fail |
| Destination name collision | Fail |

## Architecture decisions

Product and architecture decisions are recorded as ADRs under [`doc/adr/`](doc/adr/) (see the [TOC](doc/adr/README.md)).

## Development

```bash
go vet ./...
go test -race ./...
go build ./cmd/taskotter-sync
docker build -t taskotter:local .
```

## Security

- Input validation before any filesystem writes
- Secure tar.gz extraction with size limits and traversal protection
- Git invoked with argument arrays (no shell interpolation)
- GitHub token is never logged
- Downloaded Taskfiles and scripts are never executed

## License

MIT
