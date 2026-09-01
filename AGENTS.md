## Learned User Preferences

- When implementing an attached plan, do not edit the plan file itself; follow the plan as reference only.
- During plan execution, use existing todos (do not recreate them); mark each in_progress when starting and completed when finished; finish all todos without stopping early.
- Create git commits only when explicitly asked.
- CI must pass the `gofmt -l .` formatting check; run `gofmt -w` on offending files before pushing.
- GitHub Actions workflows in this repo should use `actions/checkout@v6` and `actions/setup-go@v6` for Node 24 runner compatibility.
- Root `Taskfile.yml` uses 1 space indentation per YAML level.

## Learned Workspace Facts

- TaskOtter is a Docker-based GitHub Action (Go 1.26.5) in `task-otter/Taskotter`; Marketplace display name is **TaskOtter Sync**; container entrypoint is `/taskotter`; `action.yml` requires `branding` (Feather icon + color).
- TaskOtter syncs modules from `task-otter/Taskotter-store` into consumer repos (default `taskfiles`); modules live under `taskfiles/<module>/` with transitive deps from `.deps.yml`; consumer CI needs checkout `fetch-depth: 0`; the action configures origin auth from `github-token` (clears checkout’s GitHub extraheader before push).
- Destination folder names are normalized by stripping the node package-manager suffix (e.g. `eslint/node/pnpm` and `eslint/bun` → `taskfiles/eslint`).
- A store directory is a module only when it has its own `Taskfile.yml`; its subdirectories are its variants and carry slashed names (`eslint/node/pnpm`). A directory without a `Taskfile.yml` is not a module and is not descended into, so it cannot act as a namespace prefix. Slashed modules keep their segments in the destination path and can only be pulled in as dependencies, because requested `tasks` reject `/`.
- Managed sync state is tracked in `<target-folder>/.taskotter-lock.yml`; PRs use branch `taskotter/sync-<configuration-hash>`; existing destination dirs without a lock entry are rejected as not managed.
- Sync skips `*_test.*` files; docs (`README.md`, `docs/`) are copied only when `includes-doc: true`; root `Taskfile.yml` includes merge each module's top-level `vars`.
- The action `js` input (YAML: `runtime`, `package-manager`) drives Node task resolution; `version-manager` was removed and is now a validation error.
- Each store module carries a `metadata.yml` pinned to schema `taskotter.dev/taskfile-metadata/v1`; an unrecognized schema fails the sync. Root fan-out tasks are generated from the `exported_tasks` names that at least two modules share (today `ci`, `ci:fix`, `install`, `install:undo`, `upgrade`, `version`), so the generated set follows the store rather than a hardcoded list.
- Core packages: `internal/features/{sync,resolve,store,git,pr,syncrun}` (each with `ports/` and `adapters/`, plus domain/service as needed), `internal/shared/{config,consts,iox,pathutil,logging,yamlfmt,archive,githubapi,repo}`; adapters are wired from `cmd/taskotter-sync`.
- `DefaultBranch()` falls back through symbolic-ref → rev-parse → remote set-head → remote show when `origin/HEAD` is missing (common on GHA Git 2.50+); `Stage()` uses `git add -f` for gitignored `.taskotter/metadata.yml`.
- Docker container actions expose hyphenated `INPUT_*` env vars; `internal/shared/config` reads both hyphen and underscore forms and falls back to `GITHUB_TOKEN`.
- CI in `.github/workflows/test.yml` runs gofmt, `go vet`, `go test -race`, binary/Docker builds, and integration tests under `tests/integration/`; integration sets `setup-go` `cache: false`; the `itself` job sets `fail-on-changes: true` so the action exits non-zero with `::error` when a sync PR is opened and emits `::notice` when taskfiles are up to date.
- Test fixtures mirror the store layout under `tests/fixtures/store/`.
