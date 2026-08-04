---
number: 9
title: Deterministic sync PR branches
status: accepted
date: 2026-08-04
links:
  - target: 8
    kind: relatesto
  - target: 12
    kind: relatesto
---

# Deterministic sync PR branches

## Context and Problem Statement

When sync produces changes, TaskOtter should open reviewable pull requests without creating duplicate PRs on every scheduled run for the same consumer configuration.

## Decision Drivers

* Idempotent PR updates for the same configuration
* Branch names that encode configuration identity
* No commit/PR when content is unchanged

## Considered Options

* Branch `taskotter/sync-<configuration-hash>` with update-in-place PR
* Timestamped branch per run (`taskotter/sync-YYYYMMDD…`)
* Commit directly to the default branch without a PR

## Decision Outcome

Chosen option: "Branch `taskotter/sync-<configuration-hash>` with update-in-place PR", because configuration inputs hash to a stable branch prefix; an open PR for that branch is updated rather than duplicated; title remains `chore(taskotter): sync taskfiles`.

### Consequences

* Good, because scheduled syncs converge on one PR per configuration
* Good, because hash changes when configuration changes, separating concerns
* Bad, because force-push/update semantics require contents:write and pull-requests:write

### Confirmation

`computeConfigurationHash` in [config.go](../../internal/shared/config/config.go); PR find/update in `internal/features/pr`; README “Pull requests”.

## Pros and Cons of the Options

### Branch `taskotter/sync-<configuration-hash>` with update-in-place PR

* Good, because deterministic and review-friendly
* Neutral, because hash opacity requires PR body/metadata for human context

### Timestamped branch per run (`taskotter/sync-YYYYMMDD…`)

* Good, because history of attempts
* Bad, because PR spam and harder automation

### Commit directly to the default branch without a PR

* Good, because fewer steps
* Bad, because bypasses review and is unsafe for many orgs

## More Information

* Branch pattern from configuration hash; related lockfile ([0008](0008-lockfile-managed-sync.md)), fail-on-changes ([0012](0012-fail-on-changes-for-drift-ci.md))
