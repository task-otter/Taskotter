---
num: 14
title: Split oversized feature packages by capability
status: accepted
date: 2026-09-27
links:
- target: 3
  kind: refines
---

# Split oversized feature packages by capability

## Context and Problem Statement

Feature-level packages had grown to combine orchestration, result presentation, YAML transformation,
filesystem state, and several infrastructure capabilities. The broad packages made dependency needs
unclear and forced tests for unrelated behavior to share package internals.

## Decision Outcome

Keep the feature-oriented ports-and-adapters layout from ADR 3, but split implementations into
capability packages. Domain result types live independently of orchestration and presentation;
adapters implement narrow consumer-owned ports; shared packages contain one reusable concern.

Packages that have no implementation or contract are removed instead of being retained as empty
`doc.go` placeholders. New placeholder packages must not be added in anticipation of future work.

### Consequences

* Imports communicate the specific capability a caller uses.
* Tests and failure seams live beside the capability they exercise.
* Composition wiring is more explicit.
* Internal import paths may change during the migration; action-level contracts remain stable.

## Confirmation

`go list ./...` contains no documentation-only placeholder packages, and CI verifies every concrete
package independently.
