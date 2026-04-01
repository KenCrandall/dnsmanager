# dnsmanager

`dnsmanager` is a planned control plane for `dnsmasq` that combines a web UI and a Go-based CLI for DNS, DHCP, TFTP, PXE, and operational visibility.

The target deployment is a two-container Docker Compose application:
- A `dnsmanager` app container running the backend API, embedded web UI, metrics collection, and config orchestration.
- A companion `dnsmasq` container that reads runtime configuration and related content from shared volumes managed by the app.

## Planned product surfaces

The web UI is intended to provide:
- A Pi-hole-inspired dashboard with DNS, DHCP, and service health graphs.
- A staged config editor with validation, diff preview, apply, rollback, and drift detection.
- Editors for DNS zones, DHCP settings, TFTP configuration, and PXE configuration.
- A DHCP lease manager and live log viewer.

The CLI is intended to provide:
- Remote task-oriented administration for common DNS and DHCP workflows.
- Resource CRUD for managed records, reservations, options, and revisions.
- Human-readable and automation-friendly output formats.

## Planned architecture

- Backend: Go service exposing REST APIs plus live event/log streams.
- Frontend: Svelte single-page application served by the Go backend.
- CLI: Go utility built with `spf13/cobra`.
- State: SQLite for users, tokens, audit events, staged revisions, managed objects, and short-retention metrics.
- Runtime: Docker Compose with shared config, data, and content volumes.

## Configuration model

The app is planned around a hybrid config ownership model:
- `managed/` contains structured config rendered from app-managed data.
- `manual/` preserves hand-managed or unsupported snippets.
- `generated/` contains rendered outputs and staging artifacts.

Changes will follow a staged apply pipeline:
1. Save edits as a staged revision.
2. Render config into staging.
3. Validate with `dnsmasq --test`.
4. Show a diff and validation result.
5. Publish to the shared volume atomically.
6. Trigger a controlled reload/restart of the companion `dnsmasq` container.

## Roadmap

The canonical living roadmap is maintained in [docs/plan.md](docs/plan.md).

The first repository slice is intentionally documentation-first:
- Initialize the repository.
- Capture the product vision and default architecture.
- Establish a living plan that can evolve as implementation begins.

## Current status

This repository is currently bootstrapped with project documentation only. Code scaffolding, container definitions, and application implementation will follow in later milestones.
