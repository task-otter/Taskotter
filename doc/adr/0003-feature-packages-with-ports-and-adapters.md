---
number: 3
title: Feature packages with ports and adapters
status: accepted
date: 2026-08-04
links:
  - target: 2
    kind: relatesto
  - target: 4
    kind: relatesto
---

# Feature packages with ports and adapters

## Context and Problem Statement

TaskOtter orchestrates store download, resolution, sync planning/apply, git branching, and PR creation. The codebase needs clear package boundaries so domain logic stays testable without binding every feature to GitHub or the filesystem.

## Decision Drivers

* Isolate feature domains (sync, resolve, store, git, pr, syncrun)
* Keep I/O and GitHub clients behind interfaces for unit tests
* Share cross-cutting helpers without circular feature imports

## Considered Options

* Feature packages with `ports/` and `adapters/`, plus `internal/shared/*`, wired in `cmd/taskotter-sync`
* Single flat `internal/` package tree with concrete GitHub/FS calls everywhere
* Hexagonal layout per deployable with heavy DI frameworks

## Decision Outcome

Chosen option: "Feature packages with `ports/` and `adapters/`, plus `internal/shared/*`, wired in `cmd/taskotter-sync`", because each feature owns domain/service logic and ports; adapters implement ports; [wire.go](../../cmd/taskotter-sync/wire.go) composes git CLI, store GitHub, and PR GitHub clients into the syncrun orchestrator.

### Consequences

* Good, because features can be tested with fakes at port boundaries
* Good, because shared concerns (`config`, `consts`, `pathutil`, `logging`, …) stay reusable
* Bad, because wiring and port interfaces add boilerplate for small changes

### Confirmation

Layout under `internal/features/{sync,resolve,store,git,pr,syncrun}` and `internal/shared/*`; composition in [cmd/taskotter-sync/wire.go](../../cmd/taskotter-sync/wire.go).

## Pros and Cons of the Options

### Feature packages with `ports/` and `adapters/`, plus `internal/shared/*`, wired in `cmd/taskotter-sync`

* Good, because matches hexagonal style without framework lock-in
* Good, because orchestrator depends on ports, not adapter packages directly (except at wire time)

### Single flat `internal/` package tree with concrete GitHub/FS calls everywhere

* Good, because fewer packages
* Bad, because hard to unit-test sync/PR flows without network/FS

### Hexagonal layout per deployable with heavy DI frameworks

* Good, because explicit dependency graphs
* Bad, because overkill for a single container entrypoint

## More Information

* Features: `internal/features/{sync,resolve,store,git,pr,syncrun}`
* Shared: `internal/shared/{config,consts,iox,pathutil,logging,yamlfmt,archive,githubapi,repo}`
* Wire: [cmd/taskotter-sync/wire.go](../../cmd/taskotter-sync/wire.go)
