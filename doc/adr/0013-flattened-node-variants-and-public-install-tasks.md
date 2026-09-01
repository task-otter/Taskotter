---
number: 13
title: Flattened node variants and public install tasks
status: accepted
date: 2026-09-01
links:
  - target: 4
    kind: relatesto
  - target: 5
    kind: amends
  - target: 7
    kind: amends
---

# Flattened node variants and public install tasks

## Context and Problem Statement

The store reshaped its module layout. Node variants lost their version-manager segment
(`eslint/node/fnm/pnpm` became `eslint/node/pnpm`), the `fnm` and `nvm` modules were deleted
along with every `{npm,pnpm,yarn}/{fnm,nvm}` variant, the whole `taskfiles/internal/` tree was
removed, module `_ensure` helpers became public `install` and `version` tasks, and each module
now declares a `metadata.yml` pinned to `taskotter.dev/taskfile-metadata/v1`.

TaskOtter still built the old paths, so resolving a Node task produced a module name that no
longer exists in the store. This ADR records how the sync CLI follows the new contract.

## Decision Drivers

* Resolution must name modules that actually exist in the store
* A stale store assumption should fail loudly rather than silently degrade
* Consumer-facing inputs should not survive as no-ops once they stop affecting resolution

## Considered Options

* Follow the store: drop the version-manager dimension and pin the metadata schema
* Keep `js.version-manager` as an accepted no-op for backward compatibility
* Map old variant names onto new ones inside the resolver

## Decision Outcome

Chosen option: "Follow the store: drop the version-manager dimension and pin the metadata
schema", because the version-manager choice no longer selects anything, and a compatibility
shim would preserve a knob that cannot affect the result.

Concretely:

* `BuildSourceModule` takes only a package manager and builds `<task>/node/<pkg>` or
  `<task>/bun`; destination normalization strips the matching suffixes.
* `js.version-manager` is removed from `action.yml` and the config model. Supplying it is a
  validation error rather than a silent no-op, and `node_version_manager` is gone from the
  lock file. This is a breaking change to the action input surface.
* Catalog walking treats a directory as a module only when it has its own `Taskfile.yml`.
  A directory without one is neither cataloged nor descended into, so Taskfile-less namespace
  directories are no longer a supported store shape. Slashed module names remain, because
  variant leaves still use them.
* `metadata.yml` must declare `taskotter.dev/taskfile-metadata/v1`; any other value fails the
  sync.

### Consequences

* Good, because a Node task resolves to a module the store actually publishes
* Good, because a future store contract change surfaces as an explicit schema error
* Bad, because consumers passing `js.version-manager` must edit their workflow before upgrading
* Bad, because a store that reintroduces Taskfile-less grouping directories would need a code
  change rather than working by default

### Confirmation

`internal/features/resolve/service/variants.go`; catalog walking in
`internal/features/store/service/local.go` (the GitHub adapter delegates to it); schema check in
`internal/features/sync/service/store_metadata.go`; fixtures under `tests/fixtures/store/` and
the contract tests in `internal/features/store/service/local_test.go`.

## Pros and Cons of the Options

### Follow the store: drop the version-manager dimension and pin the metadata schema

* Good, because the code models exactly one store layout
* Bad, because it breaks existing workflow YAML

### Keep `js.version-manager` as an accepted no-op

* Good, because existing workflows keep running unchanged
* Bad, because the input would quietly stop meaning anything, which is worse than a clear failure

### Map old variant names onto new ones inside the resolver

* Good, because both old and new spellings resolve
* Bad, because it carries the deleted layout forward indefinitely in translation tables

## More Information

* [action.yml](../../action.yml), [README.md](../../README.md) (`js` input)
* Amends: logical resolution ([0005](0005-logical-tasks-and-js-variant-resolution.md)),
  namespaces ([0007](0007-namespace-modules-and-dep-only-slashed-names.md))
* Related: external store as source of truth
  ([0004](0004-external-store-as-module-source-of-truth.md))
