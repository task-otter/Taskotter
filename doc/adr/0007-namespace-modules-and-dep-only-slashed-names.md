---
number: 7
title: Namespace modules and dep-only slashed names
status: accepted
date: 2026-08-04
links:
  - target: 5
    kind: relatesto
---

# Namespace modules and dep-only slashed names

## Context and Problem Statement

The store groups some modules under directory-only parents (namespaces) such as `internal/skipfiles`. Consumers need those modules as transitive dependencies without allowing arbitrary path-like strings in the public `tasks` input.

## Decision Drivers

* Support nested store layout one level of namespace deep
* Keep requested `tasks` simple and safe (no `/` or `\`)
* Preserve namespace segments in destination paths for namespaced modules

## Considered Options

* Treat dir-only parents as namespaces; allow slashed names only via dependency resolution; reject `/` in requested `tasks`
* Allow consumers to request any store-relative path in `tasks`
* Flatten namespaces away so all modules are top-level names

## Decision Outcome

Chosen option: "Treat dir-only parents as namespaces; allow slashed names only via dependency resolution; reject `/` in requested `tasks`", because cataloging treats subdirectory-only parents as namespaces, children become modules with slashed names, destinations keep the namespace segment, and `ValidateTaskName` rejects slashes in user-requested tasks.

### Consequences

* Good, because support modules can ship without cluttering the top-level task catalog UX
* Good, because path separators in `tasks` cannot be used for traversal-style names
* Bad, because namespaced modules are not first-class direct requests

### Confirmation

Store catalog namespace behavior and tests; `pathutil.ValidateTaskName`; README dependency example `internal/skipfiles` → `taskfiles/internal/skipfiles`.

## Pros and Cons of the Options

### Treat dir-only parents as namespaces; allow slashed names only via dependency resolution; reject `/` in requested `tasks`

* Good, because matches store layout without exposing path UX in `tasks`
* Neutral, because namespaces nest only one level deep by design

### Allow consumers to request any store-relative path in `tasks`

* Good, because flexible
* Bad, because weakens validation and confuses logical-task UX

### Flatten namespaces away so all modules are top-level names

* Good, because simpler destination map
* Bad, because loses organizational grouping and risks name collisions

## More Information

* Code: store catalog/namespace handling; [pathutil.ValidateTaskName](../../internal/shared/pathutil/pathutil.go)
* Related: logical resolution ([0005](0005-logical-tasks-and-js-variant-resolution.md))
