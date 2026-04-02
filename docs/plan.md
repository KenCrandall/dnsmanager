# dnsmanager Living Plan

## Summary

`dnsmanager` will be a Docker Compose-based control plane for `dnsmasq` with:

- A `dnsmanager` application container for API, web UI, metrics, and orchestration.
- A companion `dnsmasq` container that reads configuration and related content from shared volumes.
- Two operator surfaces: a web UI and a Go CLI built with `spf13/cobra`.

The web UI will provide a dashboard, staged configuration management, DNS/DHCP/TFTP/PXE editing, DHCP lease management, and log viewing. The CLI will provide remote task-oriented administration and CRUD access over the same backend API.

## Success Criteria

- Operators can manage `dnsmasq` safely through staged changes rather than live file editing.
- The app owns managed config generation while preserving manual/legacy snippets cleanly.
- The companion `dnsmasq` container can consume config from shared volumes without manual intervention.
- The dashboard provides Pi-hole-inspired operational visibility with short-retention analytics.
- The CLI and web UI share the same backend semantics for validation, apply, audit, and visibility.

## Current Defaults

### Deployment model

- Docker Compose with a `dnsmanager` app container and a companion `dnsmasq` container.
- Shared volumes:
  - Config volume for rendered dnsmasq config tree.
  - Data volume for SQLite, backups, staged revisions, and import metadata.
  - Content volume for TFTP/PXE files and future asset lifecycle management.
- `dnsmanager` is the writer for managed outputs.
- `dnsmasq` is the reader of published config and content.

### Planned stack

- Backend: Go
- Frontend: Svelte SPA
- CLI: Go with `spf13/cobra`
- Persistence: SQLite
- Streaming: SSE and/or WebSockets for live events and logs

### Config ownership

- Hybrid model with `managed/`, `manual/`, and `generated/` areas.
- Conservative first-run import with backups before any rewrite.
- Staged apply flow with validation, diff preview, atomic publish, and controlled reload/restart.

### Product defaults

- Dashboard inspired by Pi-hole graph layouts and operational summaries.
- DNS editor support for `A`, `AAAA`, `CNAME`, `PTR`, `TXT`, `SRV`, and host overrides.
- PXE v1 includes boot directives, boot entry management, and simple iPXE file creation.
- Full PXE/TFTP boot-asset lifecycle management is a later milestone and should not be blocked by v1 schema choices.
- CLI is API-backed only in v1 and authenticates with API tokens.
- Metrics retention target is 24 hours in v1.

## Milestones

| Milestone | Status | Notes |
| --- | --- | --- |
| 1. Repository bootstrap | done | Git repo initialized, README added, living plan established, minimal `.gitignore` added. |
| 2. Foundation | done | Compose scaffold, shared volumes, backend shell, Svelte shell, SQLite schema, base CLI client, and Compose/runtime verification completed. |
| 3. Controlled config lifecycle | planned | Import wizard, backups, managed/manual/generated tree, validation, diff, apply, rollback, raw snippet editing. |
| 4. Managed editors and APIs | planned | DNS, DHCP, TFTP, PXE/iPXE object model, validation, rendering, REST endpoints. |
| 5. CLI v1 | planned | Cobra command tree, token config, task commands, CRUD commands, output formatting. |
| 6. Operations views | planned | Lease manager, live logs, apply status, drift warnings. |
| 7. Dashboard and analytics | planned | Query-log ingestion, 24h roll-ups, Pi-hole-style graphs, CLI dashboard summary. |
| 8. Future PXE/TFTP expansion | planned | Full boot-asset lifecycle management, upload, versioning, organization, validation, reference tracking. |

## Current Implementation Focus

The next implementation slice should establish the controlled config lifecycle:

- Add a first-run initialization flow that prepares or imports the managed/manual/generated tree.
- Record staged config revisions in SQLite rather than only exposing filesystem paths.
- Implement config rendering and placeholder diff generation for managed fragments.
- Add `dnsmasq --test`-based validation and persist validation results.
- Introduce explicit apply and rollback primitives for the shared config volume.

## Acceptance Criteria For Next Slice

- The backend can create and persist draft config revisions.
- The shared config tree can be rendered from application state into a staging area.
- Validation can run against staged config and report pass/fail details.
- An apply action can atomically publish staged config into the shared volume.
- A rollback path exists for at least the most recent applied revision.
- The CLI and API both expose the current revision and validation/apply state.

## Open Questions

- Which live-update mechanism should be preferred first: SSE everywhere, or WebSockets only where needed?
- What initial token and local-auth bootstrap flow should be used for the first operator account?
- Which config diff representation will be most useful across both UI and CLI?
- How much existing dnsmasq config should the first import pass promote versus preserve as manual snippets?

## Deferred Scope

- Full PXE/TFTP boot-asset lifecycle management.
- SSO/OIDC auth providers.
- Multi-host or centrally managed remote `dnsmasq` fleets.
- Long-term analytics retention beyond the initial 24-hour target.
- Direct host-management mode in the CLI.

## Decision Log

| Date | Decision |
| --- | --- |
| 2026-04-01 | Use Docker Compose with separate `dnsmanager` and companion `dnsmasq` containers. |
| 2026-04-01 | Use a shared-volume model where `dnsmanager` writes managed config and `dnsmasq` reads it. |
| 2026-04-01 | Use Go for the backend and CLI, and Svelte for the web UI. |
| 2026-04-01 | Keep the initial repository slice documentation-first. |
| 2026-04-01 | Treat `docs/plan.md` as the canonical living roadmap. |
| 2026-04-01 | Use a custom Alpine-based companion `dnsmasq` image for the initial Compose scaffold. |
| 2026-04-01 | Verify the foundation slice with local Go/CLI/frontend builds plus a successful Compose stack startup. |

## Test Checklist

- [x] The repository bootstrap docs reflect the agreed architecture and roadmap.
- [x] The living plan records milestones, current defaults, open questions, and the next slice.
- [x] Foundation scaffold exists and can be executed locally.
- [x] Companion container/shared-volume integration is verified.
- [x] API, CLI, and frontend foundation pieces are connected.
