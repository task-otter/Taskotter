---
number: 11
title: Selective file copy
status: accepted
date: 2026-08-04
links:
  - target: 8
    kind: relatesto
---

# Selective file copy

## Context and Problem Statement

Store modules may include tests, docs, and store-only metadata that consumers do not need in their synced trees. Copying everything increases noise and review surface in sync PRs.

## Decision Drivers

* Keep consumer trees focused on runnable Taskfiles and supporting assets
* Make documentation opt-in/out via `includes-doc`
* Never sync store module `metadata.yml` consumed only by the catalog

## Considered Options

* Skip `*_test.*` and module metadata; copy `README.md`/`docs/` only when `includes-doc: true`
* Always copy the entire module directory
* Never copy documentation

## Decision Outcome

Chosen option: "Skip `*_test.*` and module metadata; copy `README.md`/`docs/` only when `includes-doc: true`", because planning filters test paths and module metadata; documentation inclusion follows the action input (default true).

### Consequences

* Good, because sync PRs stay smaller and more relevant
* Good, because docs can be enabled for discoverability or disabled for minimal trees
* Bad, because consumers cannot rely on store tests being present locally after sync

### Confirmation

`pathutil.IsTestPath` / `IsModuleMetadataPath`; sync plan filtering; README `includes-doc` input.

## Pros and Cons of the Options

### Skip `*_test.*` and module metadata; copy `README.md`/`docs/` only when `includes-doc: true`

* Good, because tunable docs without always shipping tests
* Neutral, because skip rules must stay documented for store authors

### Always copy the entire module directory

* Good, because lossless
* Bad, because noisy PRs and unused test files in consumers

### Never copy documentation

* Good, because minimal
* Bad, because hurts discoverability of module usage

## More Information

* Code: [pathutil.go](../../internal/shared/pathutil/pathutil.go) (`IsTestPath`, docs helpers); sync plan skip in `internal/features/sync/service/plan.go`
* Related: lockfile-managed inventory ([0008](0008-lockfile-managed-sync.md))
