---
number: 8
title: Lockfile-managed sync
status: accepted
date: 2026-08-04
links:
  - target: 4
    kind: relatesto
  - target: 9
    kind: relatesto
  - target: 10
    kind: relatesto
  - target: 11
    kind: relatesto
---

# Lockfile-managed sync

## Context and Problem Statement

TaskOtter writes modules into a consumer `target-folder`. Existing directories may be hand-maintained. The sync engine needs a clear ownership model so it neither silently overwrites unmanaged trees nor loses track of what it manages.

## Decision Drivers

* Explicit managed vs unmanaged destinations
* Persist sync configuration and managed file inventory across runs
* Safe upgrades/removals of previously synced modules

## Considered Options

* Track state in `<target-folder>/.taskotter-lock.yml` and reject unmanaged existing dirs
* Always overwrite destination trees without a lock
* Require an empty `target-folder` on every sync

## Decision Outcome

Chosen option: "Track state in `<target-folder>/.taskotter-lock.yml` and reject unmanaged existing dirs", because the lock records source metadata, configuration, and managed paths; planning fails when a destination directory exists without a lock entry, protecting local content.

### Consequences

* Good, because accidental clobbering of hand-written taskfiles is avoided
* Good, because subsequent syncs can diff and update managed content deterministically
* Bad, because first-time adoption into a non-empty unmanaged folder requires manual cleanup or relocation

### Confirmation

Lock model under `internal/features/sync/domain/lockmodel`; unmanaged destination tests; README validation table (“Unmanaged existing destination directory”).

## Pros and Cons of the Options

### Track state in `<target-folder>/.taskotter-lock.yml` and reject unmanaged existing dirs

* Good, because clear ownership and auditable managed file lists
* Neutral, because lock YAML becomes part of the consumer repo

### Always overwrite destination trees without a lock

* Good, because simpler
* Bad, because destructive for mixed or hand-edited folders

### Require an empty `target-folder` on every sync

* Good, because no ownership ambiguity
* Bad, because hostile to incremental PR-based workflows

## More Information

* Lock path: `<target-folder>/.taskotter-lock.yml` (also metadata under `<target-folder>/.taskotter/`)
* Related: PR branches ([0009](0009-deterministic-sync-pr-branches.md)), root Taskfile ([0010](0010-optional-root-taskfile-sync.md)), selective copy ([0011](0011-selective-file-copy.md))
