## Learned User Preferences

- When implementing an attached plan, do not edit the plan file itself; follow the plan as reference only.
- During plan execution, use existing todos (do not recreate them); mark each in_progress when starting and completed when finished; finish all todos without stopping early.
- Create git commits only when explicitly asked.
- CI must pass the `gofmt -l .` formatting check; run `gofmt -w` on offending files before pushing.
- GitHub Actions workflows in this repo should use `actions/checkout@v6` and `actions/setup-go@v6` for Node 24 runner compatibility.
- Root `Taskfile.yml` uses 1 space indentation per YAML level.

## Learned Workspace Facts

- TaskOtter is a Docker-based GitHub Action (Go 1.26.5) in `task-otter/Taskotter`; Marketplace display name is **TaskOtter Sync**; container entrypoint is `/taskotter`; `action.yml` requires `branding` (Feather icon + color).
- TaskOtter syncs modules from `task-otter/Taskotter-store` into consumer repos (default `taskfiles`); modules live under `taskfiles/<module>/` with transitive deps from `.deps.yml`; consumer CI needs checkout `fetch-depth: 0` and must not set `persist-credentials: false` on the TaskOtter job.
- Destination folder names are normalized by stripping package-manager and version-manager suffixes (e.g. `eslint-pnpm-fnm` → `taskfiles/eslint`).
- Store directories that contain only subdirectories are namespaces, so their children are modules with slashed names (`internal/skipfiles`); namespaces nest one level deep, namespaced modules keep the namespace segment in the destination path, and they can only be pulled in as dependencies because requested `tasks` reject `/`.
- Managed sync state is tracked in `<target-folder>/.taskotter-lock.yml`; PRs use branch `taskotter/sync-<configuration-hash>`; existing destination dirs without a lock entry are rejected as not managed.
- Sync skips `*_test.*` files; docs (`README.md`, `docs/`) are copied only when `includes-doc: true`; root `Taskfile.yml` includes merge each module's top-level `vars`.
- The action `js` input (YAML: `runtime`, `package-manager`, `version-manager`) replaces separate node-package-manager inputs for Node task resolution.
- Root `Taskfile.yml` defines aggregate `lint`, `lint:fix`, `fmt`, and `fmt:check` tasks delegating to included module taskfiles.
- Core packages: `internal/features/{sync,resolve,store,git,pr,syncrun}` (each with `ports/` and `adapters/`, plus domain/service as needed), `internal/shared/{config,consts,iox,pathutil,logging,yamlfmt,archive,githubapi,repo}`; adapters are wired from `cmd/taskotter-sync`.
- `DefaultBranch()` falls back through symbolic-ref → rev-parse → remote set-head → remote show when `origin/HEAD` is missing (common on GHA Git 2.50+); `Stage()` uses `git add -f` for gitignored `.taskotter/metadata.yml`.
- Docker container actions expose hyphenated `INPUT_*` env vars; `internal/shared/config` reads both hyphen and underscore forms and falls back to `GITHUB_TOKEN`.
- CI in `.github/workflows/test.yml` runs gofmt, `go vet`, `go test -race`, binary/Docker builds, and integration tests under `tests/integration/`; integration sets `setup-go` `cache: false`; the `itself` job sets `fail-on-changes: true` so the action exits non-zero with `::error` when a sync PR is opened and emits `::notice` when taskfiles are up to date.
- Test fixtures mirror the store layout under `tests/fixtures/store/`.
