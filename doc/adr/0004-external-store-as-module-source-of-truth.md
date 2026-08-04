---
number: 4
title: External store as module source of truth
status: accepted
date: 2026-08-04
links:
  - target: 3
    kind: relatesto
  - target: 5
    kind: relatesto
  - target: 8
    kind: relatesto
---

# External store as module source of truth

## Context and Problem Statement

Consumers need curated Taskfile modules (lint, install, tooling) without each repo inventing and maintaining them. TaskOtter must obtain module content safely and keep a clear trust boundary.

## Decision Drivers

* One shared catalog of modules versioned independently of consumer apps
* Transitive dependencies via `.deps.yml`
* Security: never execute downloaded Taskfiles or scripts during sync

## Considered Options

* Sync from an external TaskOtter store repository (download/archive), never execute content
* Vendor modules inside the TaskOtter action image
* Clone consumer-supplied arbitrary git URLs as module sources

## Decision Outcome

Chosen option: "Sync from an external TaskOtter store repository (download/archive), never execute content", because the store is the source of truth; TaskOtter resolves tags or default-branch HEAD, extracts with size/traversal limits, copies files into the consumer repo, and leaves execution to the consumer's own Task/CI.

### Consequences

* Good, because module evolution is centralized and pinable via `store-version`
* Good, because the action's attack surface excludes running untrusted scripts
* Bad, because consumers depend on store availability and tagging discipline

### Confirmation

README security note and features list; store client under `internal/features/store`; archive extraction in `internal/shared/archive`.

## Pros and Cons of the Options

### Sync from an external TaskOtter store repository (download/archive), never execute content

* Good, because clear separation of catalog vs sync engine
* Good, because pinned tags support reproducible consumer CI

### Vendor modules inside the TaskOtter action image

* Good, because offline-friendly
* Bad, because every module change requires an action release

### Clone consumer-supplied arbitrary git URLs as module sources

* Good, because flexible
* Bad, because weak trust model and harder UX/validation

## More Information

* Store: https://github.com/task-otter/store (documented in [README.md](../../README.md))
* Related: resolution ([0005](0005-logical-tasks-and-js-variant-resolution.md)), lockfile ([0008](0008-lockfile-managed-sync.md))
