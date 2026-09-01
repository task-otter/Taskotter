---
number: 7
title: Namespace modules and dep-only slashed names
status: accepted
date: 2026-08-04
links:
  - target: 13
    kind: amendedby
  - target: 5
    kind: relatesto
---

# Namespace modules and dep-only slashed names

> **Amended by [ADR 0013](0013-flattened-node-variants-and-public-install-tasks.md).** The store
> removed its Taskfile-less namespace directories (`taskfiles/internal/`), so a directory is a
> module only when it has its own `Taskfile.yml`. Slashed module names remain, because variant
> leaves such as `eslint/node/pnpm` still use them; the dep-only rule below is unchanged.

## Context and Problem Statement

Some store modules live below a parent module rather than at the top level — variant leaves such
as `eslint/node/pnpm`, and previously namespaced support modules under a directory-only parent.
Consumers need those modules as transitive dependencies without allowing arbitrary path-like
strings in the public `tasks` input.

## Decision Drivers

* Support the store's nested module layout
* Keep requested `tasks` simple and safe (no `/` or `\`)
* Preserve the nested segments in destination paths for slashed modules

## Considered Options

* Catalog nested modules under slashed names; allow those names only via dependency resolution; reject `/` in requested `tasks`
* Allow consumers to request any store-relative path in `tasks`
* Flatten nesting away so all modules are top-level names

## Decision Outcome

Chosen option: "Catalog nested modules under slashed names; allow those names only via dependency
resolution; reject `/` in requested `tasks`", because cataloging walks a module's subdirectories
into slashed child names, destinations keep those segments, and `ValidateTaskName` rejects slashes
in user-requested tasks.

Per ADR 0013, a directory qualifies as a module only when it contains a `Taskfile.yml`; a
directory without one is neither cataloged nor descended into, so it cannot act as a namespace
prefix.

### Consequences

* Good, because support and variant modules can ship without cluttering the top-level task catalog UX
* Good, because path separators in `tasks` cannot be used for traversal-style names
* Bad, because nested modules are not first-class direct requests

### Confirmation

Store catalog walking in `internal/features/store/service/local.go` and its tests;
`pathutil.ValidateTaskName`; README dependency example `eslint/node/pnpm` →
`taskfiles/eslint/node/pnpm`.

## Pros and Cons of the Options

### Catalog nested modules under slashed names; allow those names only via dependency resolution; reject `/` in requested `tasks`

* Good, because matches store layout without exposing path UX in `tasks`
* Neutral, because nesting depth is bounded by what the store publishes

### Allow consumers to request any store-relative path in `tasks`

* Good, because flexible
* Bad, because weakens validation and confuses logical-task UX

### Flatten nesting away so all modules are top-level names

* Good, because simpler destination map
* Bad, because loses organizational grouping and risks name collisions

## More Information

* Code: store catalog walking; [pathutil.ValidateTaskName](../../internal/shared/pathutil/pathutil.go)
* Related: logical resolution ([0005](0005-logical-tasks-and-js-variant-resolution.md))
* Amended by: [0013](0013-flattened-node-variants-and-public-install-tasks.md)
