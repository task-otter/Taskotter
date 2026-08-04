---
number: 10
title: Optional root Taskfile sync
status: accepted
date: 2026-08-04
links:
  - target: 8
    kind: relatesto
---

# Optional root Taskfile sync

## Context and Problem Statement

Synced modules are most useful when the consumer root `Taskfile.yml` includes them and exposes aggregate tasks/vars. Some repos already manage their root Taskfile manually and must opt out of automatic edits.

## Decision Drivers

* Default to a working root Taskfile for new adopters
* Preserve unmanaged root content when a file already exists
* Allow module-only sync via `sync-root: false`

## Considered Options

* Optional `sync-root` (default true) merging managed includes and top-level module `vars`
* Always rewrite the entire root Taskfile
* Never touch the root Taskfile (documentation-only includes)

## Decision Outcome

Chosen option: "Optional `sync-root` (default true) merging managed includes and top-level module `vars`", because TaskOtter creates a minimal root Taskfile when missing, updates managed includes (and shared aggregate tasks/vars from module metadata) when present, and skips all root Taskfile I/O when `sync-root` is false.

### Consequences

* Good, because `task lint` / `fmt` style aggregates can be generated for shared exported tasks
* Good, because opt-out supports advanced layouts
* Bad, because partial root ownership can surprise users who edit managed include blocks by hand

### Confirmation

`sync-root` in [action.yml](../../action.yml); sync apply/plan paths gated on `Config.SyncRoot`; README “Prerequisites”.

## Pros and Cons of the Options

### Optional `sync-root` (default true) merging managed includes and top-level module `vars`

* Good, because balances onboarding and escape hatch
* Neutral, because root task generation depends on store metadata quality

### Always rewrite the entire root Taskfile

* Good, because simpler code
* Bad, because destroys consumer custom tasks

### Never touch the root Taskfile (documentation-only includes)

* Good, because zero surprise edits
* Bad, because weaker out-of-the-box experience

## More Information

* Related: lockfile ([0008](0008-lockfile-managed-sync.md)); root aggregate tasks noted in [README.md](../../README.md) and AGENTS.md
