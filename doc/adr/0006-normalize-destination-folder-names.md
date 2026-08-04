---
number: 6
title: Normalize destination folder names
status: accepted
date: 2026-08-04
links:
  - target: 5
    kind: relatesto
---

# Normalize destination folder names

## Context and Problem Statement

Store modules often include package-manager and version-manager path segments (for example `eslint/node/fnm/pnpm`). Copying that layout into consumer repos would churn directories whenever JS tooling preferences change and complicate stable includes.

## Decision Drivers

* Stable consumer paths under `target-folder` across variant changes
* Predictable Taskfile include keys (`eslint`, not `eslint-pnpm-fnm`)
* Detect collisions when two sources normalize to the same destination

## Considered Options

* Strip package/version-manager (and related) suffixes to a logical destination name
* Mirror full store paths under `target-folder`
* Hash or encode full source paths into opaque folder names

## Decision Outcome

Chosen option: "Strip package/version-manager (and related) suffixes to a logical destination name", because `Normalize` repeatedly strips known suffixes so `eslint/node/fnm/pnpm` lands at `taskfiles/eslint` (default target), and `BuildDestinationMap` fails on collisions.

### Consequences

* Good, because includes and local paths stay stable when switching pnpm/npm or fnm/nvm
* Good, because root Taskfile includes stay human-readable
* Bad, because colliding variants cannot coexist as separate destination trees

### Confirmation

`internal/features/resolve/service/normalizer.go` and tests; README “Destination layout”.

## Pros and Cons of the Options

### Strip package/version-manager (and related) suffixes to a logical destination name

* Good, because matches how users think about tools (`eslint`, `pnpm`, `fnm`)
* Bad, because encoding of “known suffixes” must stay aligned with the store

### Mirror full store paths under `target-folder`

* Good, because lossless mapping
* Bad, because path churn and deep trees when JS settings change

### Hash or encode full source paths into opaque folder names

* Good, because collision-free
* Bad, because opaque includes hurt usability

## More Information

* Code: [normalizer.go](../../internal/features/resolve/service/normalizer.go)
* Related: logical resolution ([0005](0005-logical-tasks-and-js-variant-resolution.md))
