---
number: 12
title: Fail-on-changes for drift CI
status: accepted
date: 2026-08-04
links:
  - target: 9
    kind: relatesto
---

# Fail-on-changes for drift CI

## Context and Problem Statement

Repositories that adopt TaskOtter often want CI to detect when local taskfiles drift from the store and block merge until a sync PR is applied—without making every sync workflow a hard failure by default.

## Decision Drivers

* Opt-in strictness for drift gates
* Clear GitHub Actions annotations when a sync PR is opened
* Default soft behavior for scheduled/manual sync jobs that only open PRs

## Considered Options

* `fail-on-changes` input: non-zero exit and `::error` when a sync PR is opened/updated
* Always fail the action whenever files change
* Never fail; rely only on outputs/`changed`

## Decision Outcome

Chosen option: "`fail-on-changes` input: non-zero exit and `::error` when a sync PR is opened/updated`", because default `false` suits sync bots; `true` (as in this repo’s `itself` job) fails CI with an explicit sync-required error annotation until the PR is merged. Up-to-date runs can emit notice-style success messaging.

### Consequences

* Good, because drift checks compose with `needs:` job graphs
* Good, because annotations surface the PR URL in the Actions UI
* Bad, because jobs must grant PR permissions even when used only as a gate

### Confirmation

`FailOnChanges` in config; `writeSyncRequiredAnnotations` in syncrun result handling; [action.yml](../../action.yml) input; README “CI drift check”; `.github/workflows/test.yml` `itself` job.

## Pros and Cons of the Options

### `fail-on-changes` input: non-zero exit and `::error` when a sync PR is opened/updated

* Good, because one action serves both sync-bot and gate modes
* Neutral, because consumers must wire the flag intentionally

### Always fail the action whenever files change

* Good, because strict by default
* Bad, because breaks scheduled sync workflows that expect success after opening a PR

### Never fail; rely only on outputs/`changed`

* Good, because simplest exit semantics
* Bad, because easy to ignore drift without extra scripting

## More Information

* Code: `internal/features/syncrun/service/result.go` (`::error title=TaskOtter sync required::`)
* Related: deterministic PRs ([0009](0009-deterministic-sync-pr-branches.md))
