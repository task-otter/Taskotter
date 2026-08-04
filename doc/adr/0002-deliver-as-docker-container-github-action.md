---
number: 2
title: Deliver as Docker container GitHub Action
status: accepted
date: 2026-08-04
links:
  - target: 3
    kind: relatesto
---

# Deliver as Docker container GitHub Action

## Context and Problem Statement

TaskOtter must run inside consumer GitHub Actions workflows with a fixed runtime (Go binary + git) and Marketplace branding. How should the action be packaged and executed on the runner?

## Decision Drivers

* Predictable toolchain (Go build, Alpine, git) independent of runner language setup
* Simple consumer usage (`uses: task-otter/Taskotter@…`) without Node action bundling
* Marketplace requirements (`action.yml` branding)

## Considered Options

* Docker container action (`runs.using: docker`, image from repo `Dockerfile`)
* JavaScript/TypeScript composite or Node action wrapping a downloaded binary
* Composite action that installs Go and builds from source on each run

## Decision Outcome

Chosen option: "Docker container action (`runs.using: docker`, image from repo `Dockerfile`)", because a multi-stage Alpine image builds a static `/taskotter` binary and ships git/ca-certificates with a fixed entrypoint, matching the Marketplace display name **TaskOtter Sync**.

### Consequences

* Good, because consumers get a hermetic runtime without installing Go
* Good, because local `docker build` mirrors CI/action execution
* Bad, because Docker actions are slower to start than pure JS actions on some runners

### Confirmation

[action.yml](../../action.yml) `runs.using: docker` / `image: Dockerfile`; [Dockerfile](../../Dockerfile) entrypoint `/taskotter`; CI Docker build in `.github/workflows/test.yml`.

## Pros and Cons of the Options

### Docker container action (`runs.using: docker`, image from repo `Dockerfile`)

* Good, because Go + git are baked in; no runner setup-go required for the action itself
* Good, because CGO-disabled static binary keeps the runtime image small
* Bad, because image pull/build cost on cold runners

### JavaScript/TypeScript composite or Node action wrapping a downloaded binary

* Good, because often faster cold start
* Bad, because dual-language release surface and platform binary matrix

### Composite action that installs Go and builds from source on each run

* Good, because no image maintenance
* Bad, because slow, network-heavy, and fragile for consumers

## More Information

* [action.yml](../../action.yml), [Dockerfile](../../Dockerfile), [cmd/taskotter-sync](../../cmd/taskotter-sync)
