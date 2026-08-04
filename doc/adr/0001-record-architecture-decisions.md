---
number: 1
title: Record architecture decisions
status: accepted
date: 2026-08-04
---

# Record architecture decisions

## Context and Problem Statement

TaskOtter has accumulated product and architecture choices that are only implicit in code, README, and agent notes. New contributors need a durable, reviewable record of *why* the system is shaped the way it is, without turning every implementation quirk into documentation.

## Decision Drivers

* Significant product/architecture decisions should be findable without reading the full codebase
* Records should capture options and consequences, not only the chosen outcome
* Authoring and TOC generation should fit the project's existing Taskfile automation

## Considered Options

* Architecture Decision Records with MADR 4.0 (`full`) under `doc/adr`, managed by `adrs`
* Informal README-only architecture sections
* Nygard-style one-page ADRs without structured options/drivers

## Decision Outcome

Chosen option: "Architecture Decision Records with MADR 4.0 (`full`) under `doc/adr`, managed by `adrs`", because MADR captures drivers, options, and consequences while `adrs` (via `taskotter:adrs`) installs, creates, lists, links, and generates a TOC consistently.

### Consequences

* Good, because architecture intent is versioned next to the code
* Good, because related decisions can be linked and indexed
* Bad, because ADRs require maintenance when major design changes land

### Confirmation

Presence of `adrs.toml` (`mode = "nextgen"`, MADR `full`), `doc/adr/` records, and `task taskotter:adrs:list` / `doctor` succeeding.

## Pros and Cons of the Options

### Architecture Decision Records with MADR 4.0 (`full`) under `doc/adr`, managed by `adrs`

* Good, because structured sections fit multi-option product decisions
* Good, because the repo already vendors `taskfiles/adrs`
* Neutral, because tooling adds a Rust/`cargo` dependency for contributors who author ADRs

### Informal README-only architecture sections

* Good, because no extra tooling
* Bad, because README mixes user docs with decision history and lacks status/links

### Nygard-style one-page ADRs without structured options/drivers

* Good, because short and familiar
* Bad, because weaker capture of rejected alternatives for a product with many trade-offs

## More Information

* Tooling: `taskfiles/adrs/README.md`, root `adrs.toml`
* Format: [MADR](https://adr.github.io/madr/)
