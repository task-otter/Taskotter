---
number: 5
title: Logical tasks and JS variant resolution
status: accepted
date: 2026-08-04
links:
  - target: 13
    kind: amendedby
  - target: 4
    kind: relatesto
  - target: 6
    kind: relatesto
  - target: 7
    kind: relatesto
---

# Logical tasks and JS variant resolution

> **Amended by [ADR 0013](0013-flattened-node-variants-and-public-install-tasks.md).** The store
> dropped its version-manager dimension, so `js.version-manager` was removed and Node variant
> paths are now `<task>/node/<pkg>`. The logical-task and unified-`js`-input decision below still
> holds; only the version-manager dimension is superseded.

## Context and Problem Statement

The store holds many Node variants (package manager × runtime) plus non-Node tools. Consumers should request logical names like `eslint` or `go` without encoding full store paths, while Node choices must still select the correct variant.

## Decision Drivers

* Simple `tasks` input for humans and CI YAML
* Correct selection among store paths such as `eslint/node/pnpm`
* One structured `js` input instead of several separate Node-related action inputs

## Considered Options

* Logical task names plus unified `js` YAML (`runtime`, `package-manager`)
* Require full store module paths in `tasks`
* Separate action inputs per Node dimension (legacy-style)

## Decision Outcome

Chosen option: "Logical task names plus unified `js` YAML (`runtime`, `package-manager`)", because non-Node tasks resolve by name; Node tasks combine the logical name with `js` settings (bun ignores the package manager). Invalid combinations fail before download.

### Consequences

* Good, because consumer YAML stays short and readable
* Good, because validation can reject unsafe or inconsistent Node settings early
* Bad, because Node tasks require remembering to supply `js`

### Confirmation

[action.yml](../../action.yml) `tasks` / `js` inputs; resolve logic in `internal/features/resolve/service` (e.g. variants); README “Module resolution”.

## Pros and Cons of the Options

### Logical task names plus unified `js` YAML (`runtime`, `package-manager`)

* Good, because one YAML block scales as Node dimensions grow
* Good, because defaults (`nodejs`/`npm`) cover common cases

### Require full store module paths in `tasks`

* Good, because explicit
* Bad, because couples consumers to store layout and suffix conventions

### Separate action inputs per Node dimension (legacy-style)

* Good, because flat env mapping
* Bad, because noisier Marketplace UI and harder grouped validation

## More Information

* [action.yml](../../action.yml), [README.md](../../README.md) (`js` input)
* Code: `internal/features/resolve/service/variants.go`
* Related: destination normalization ([0006](0006-normalize-destination-folder-names.md)), slashed variant names ([0007](0007-namespace-modules-and-dep-only-slashed-names.md))
* Amended by: [0013](0013-flattened-node-variants-and-public-install-tasks.md)
