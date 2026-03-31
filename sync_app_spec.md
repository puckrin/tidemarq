# tidemarq — Formal Product Specification

**Version:** 1.1 (Draft)
**Date:** 31 March 2026
**Status:** Draft

---

## 1. Overview

### 1.1 Purpose

tidemarq is a self-hosted web application that enables users to configure, schedule, and monitor directory synchronisation jobs across local filesystems and network-attached storage. It provides a browser-based interface for managing sync rules, resolving conflicts, and reviewing version history — with no dependency on external cloud services.

### 1.2 Goals

- Provide a clean, intuitive web UI for managing directory sync jobs without requiring command-line access.
- Support both one-way (backup/mirror) and two-way (bidirectional) synchronisation.
- Run entirely within a user's local network with no external data egress unless explicitly configured.
- Be deployable on commodity hardware including low-power ARM devices such as a Raspberry Pi or consumer NAS.

### 1.3 Out of scope

- Local agent software running on remote client machines.
- Mobile native applications.
- Cloud storage destinations (S3, Google Drive, etc.) in v1.0.
- End-to-end encrypted remote sync over the internet in v1.0.

---

## 2. System Architecture

### 2.1 Components

tidemarq consists of the following components, distributed as a single Docker Compose stack:

**Sync server.** The central process exposing a REST API and WebSocket interface. Responsible for job scheduling, filesystem watching, the transfer engine, conflict detection, audit logging, and user authentication.

**Web UI.** A single-page application served by the sync server. Provides the directory browser, job configuration, monitoring dashboard, and conflict resolution interface.

**Database.** An embedded SQLite instance (default) or external PostgreSQL instance (optional, for higher-concurrency deployments). Stores job configuration, file manifests, version metadata, audit logs, and user accounts.

**Metadata store.** A structured file store on local disk holding version history snapshots and soft-deleted files. Managed exclusively by the sync server.

### 2.2 Deployment

- Distributed as a Docker Compose stack with a single `docker-compose.yml`.
- All persistent data stored in a user-defined volume mount.
- ARM64 and AMD64 images provided.
- No outbound internet connectivity required at runtime.
- A configuration file (`tidemarq.yaml` or `tidemarq.toml`) is supported as a full alternative to UI-based configuration, enabling GitOps workflows. UI changes are reflected in the config file and vice versa.

---

## 3. Functional Requirements

### 3.1 Sync Jobs

#### 3.1.1 Job modes

| Mode | Behaviour |
|---|---|
| One-way (mirror) | Changes on the source are applied to the destination. Deletions on the source are replicated. |
| One-way (backup) | Changes on the source are applied to the destination. Deletions on the source are not replicated. |
| Two-way (bidirectional) | Changes on either side are propagated to the other. Conflicts are detected and queued. |

#### 3.1.2 Job configuration

Each sync job shall have the following configurable properties:

- **Name.** A user-defined label.
- **Source path.** A local filesystem path or network mount path.
- **Destination path.** A local filesystem path or network mount path.
- **Mode.** One of the modes defined in §3.1.1.
- **Trigger.** Real-time (filesystem watch), scheduled (cron expression), or manual-only.
- **File filters.** One or more include/exclude rules (see §3.3).
- **Conflict strategy.** The default resolution strategy for this job (see §3.5).
- **Version history settings.** Retention policy for this job (see §3.6).
- **Bandwidth limit.** Optional transfer rate cap in KB/s, with optional time-of-day schedule.
- **Enabled/disabled flag.**

#### 3.1.3 Job lifecycle

- Jobs may be in one of the following states: `idle`, `running`, `paused`, `error`, `disabled`.
- A running job may be paused or cancelled from the UI at any time.
- A paused or cancelled job resumes from a safe checkpoint on next trigger; no data corruption shall result from interruption.
- All sync operations shall be idempotent: running a job twice with no intervening file changes shall produce no mutations.

### 3.2 Filesystem Support

#### 3.2.1 Local filesystems

The server process must be able to access any path that is mounted into its Docker container. The user is responsible for bind-mounting the relevant paths in `docker-compose.yml`. The UI will present a directory browser scoped to the paths made available via configuration.

#### 3.2.2 Network sources and destinations

The following network protocols shall be supported as sync sources or destinations:

- **SMB/CIFS** — via credentials (username/password) or guest access.
- **NFS** — via mount, exposed as a mounted path within the container.
- **SFTP** — via password or SSH private key authentication.

Network locations are configured as named mounts in the application and are then available for selection in the job directory browser as first-class paths.

### 3.3 File Filtering

Each sync job may define one or more filter rules applied in order. Filter rules support:

- **Include/exclude by glob pattern** — e.g. `*.log`, `**/node_modules/**`.
- **Include/exclude by file extension** — e.g. exclude `.tmp`, `.part`.
- **Include/exclude by file size** — e.g. exclude files larger than 4 GB.
- **Include/exclude by last modified date** — e.g. only include files modified in the last 30 days.
- **Hidden file handling** — configurable flag to include or exclude dot-files and system hidden files.

Rules are evaluated top-to-bottom; the first matching rule wins. If no rule matches, the file is included by default.

### 3.4 Transfer Engine

- Only the changed portions of a file (byte-level delta) shall be transferred where the protocol supports it, minimising bandwidth consumption.
- Large files shall be transferred in chunks to allow resumable transfers.
- File integrity shall be verified via checksum (SHA-256) after transfer.
- File metadata shall be preserved on transfer: last modified timestamp, POSIX permissions, and ownership where the destination filesystem supports it.
- Transfers shall respect the per-job bandwidth limit configuration.

### 3.5 Conflict Detection and Resolution

A conflict is defined as a file that has been modified on both the source and destination since the last successful sync of that file.

#### 3.5.1 Detection

Conflicts are detected by comparing the stored file manifest (hash + modification time at last sync) against the current state of both sides.

#### 3.5.2 Resolution strategies

The following strategies are configurable per job and may be overridden per conflict in the UI:

| Strategy | Behaviour |
|---|---|
| Newest wins | The file with the more recent modification timestamp is kept. |
| Largest wins | The file with the greater size is kept. |
| Source wins | The source side always takes precedence. |
| Destination wins | The destination side always takes precedence. |
| Ask user | The conflict is queued and held until manually resolved. |

#### 3.5.3 Conflict quarantine

When a conflict cannot be automatically resolved (strategy is "ask user", or no strategy matches), both versions of the file are preserved. The destination copy is renamed with a `.conflict.<timestamp>` suffix. The conflict is recorded in the conflict queue and surfaced in the UI for manual review.

Conflicts not resolved within a configurable grace period (default: 30 days) shall trigger a notification.

### 3.6 Versioning and History

- Each sync job may be configured with a version retention policy:
  - **By count** — retain the last N versions of each file (default: 10).
  - **By age** — retain versions modified within the last X days.
  - **Disabled** — no version history; latest state only.
- Previous versions are stored in the metadata store outside the destination path.
- Any previous version may be restored to the destination path via the UI with a single action.
- Version history is browsable per file via the UI.

### 3.7 Soft Deletion

When a file is deleted from the source in one-way mirror mode, or from either side in two-way mode, the file is not immediately purged from the destination. Instead:

- The file is moved to a per-job quarantine area within the metadata store.
- It remains there for a configurable retention period (default: 14 days).
- It may be restored via the UI at any time during the retention period.
- After the retention period expires, it is permanently deleted.
- Permanent deletion may also be triggered manually from the UI.

### 3.8 Scheduling

- Jobs may be triggered by any combination of: filesystem event, cron schedule, or manual invocation.
- Filesystem watching uses the native OS mechanism: inotify (Linux), FSEvents (macOS), ReadDirectoryChangesW (Windows).
- Polling fallback is used where native watching is unavailable (e.g. network mounts).
- The polling interval for network mounts is configurable (default: 60 seconds).
- Cron expressions follow standard 5-field syntax (`minute hour day month weekday`).
- Bandwidth limits may include a time-of-day schedule, e.g. limit to 1 MB/s between 09:00–18:00, unlimited otherwise.

---

## 4. User Interface Requirements

### 4.1 General

- The UI is a responsive single-page web application, targeting modern desktop and tablet browsers.
- Dark mode and light mode are supported, respecting the OS preference by default with a manual override.
- All actions that modify state (create job, delete job, restore file, resolve conflict) require explicit confirmation.
- The UI exposes the following top-level sections via the navigation rail: **Dashboard**, **Sync Jobs**, **Conflicts**, **Audit Log**, **Mounts**, and **Settings**.
- The Settings UI is intentionally scoped to end-user concerns: **Users** (account management and role assignments) and **Sync Defaults** (default conflict strategy, retention periods, bandwidth, and polling interval). All other configuration (TLS, database, auth/OIDC, notifications, bind address) is managed via the `tidemarq.yaml` config file or environment variables and does not require a UI surface.

### 4.2 Directory Browser

- A graphical directory browser component allows users to navigate and select directories on any filesystem accessible to the server (local mounts and configured network mounts).
- The browser displays directory name, item count, and total size per node.
- The user may type a path directly as an alternative to navigation.

### 4.3 Job Management

- A job list view displays all jobs with their current state, last run time, last run outcome, and next scheduled run.
- Job creation follows a guided flow: select source → select destination → choose mode → configure filters and schedule → review and save.
- Individual jobs may be run manually, paused, resumed, edited, or deleted from the job detail view.
- A live progress view shows files transferred, current transfer rate, estimated time remaining, and any errors for a running job.

### 4.4 Monitoring Dashboard

- A summary dashboard displays aggregate statistics: total jobs, jobs in error state, total data transferred in the last 24 hours and 7 days, and storage used by version history.
- Each job has a run history view showing a timeline of past runs with outcome, duration, files changed, and any errors.

### 4.5 Conflict Resolution UI

- A dedicated conflict queue view lists all unresolved conflicts across all jobs, sortable by job, file path, and age.
- Each conflict displays: file path, last modified timestamp and size for both versions, and a diff summary where applicable.
- The user may resolve a conflict by choosing one version, keeping both (with rename), or discarding both.
- Bulk resolution is supported: apply a strategy to all selected conflicts.

### 4.6 Audit Log

- A searchable audit log is accessible from the UI, recording all sync events, user actions, and system events with timestamp, user, and detail.
- Log entries are filterable by job, user, event type, and date range.
- Logs may be exported as CSV or JSON.

---

## 5. Security Requirements

### 5.1 Authentication

- The application requires authentication to access any page or API endpoint.
- Username and password authentication is supported, with passwords stored as bcrypt hashes.
- Session tokens are issued as signed JWTs with a configurable expiry (default: 8 hours).
- An optional OIDC/OAuth2 provider integration allows SSO login (e.g. Authelia, Authentik).

### 5.2 Authorisation

Three roles are defined:

| Role | Permissions |
|---|---|
| Admin | Full access: manage users, configure mounts, create/edit/delete any job, access all logs. |
| Operator | Create and manage own jobs, run any job, resolve conflicts, view logs. |
| Viewer | Read-only access to job status, run history, and logs. No ability to trigger or modify. |

### 5.3 Transport Security

- All web traffic must be served over HTTPS.
- The server will auto-generate a self-signed certificate on first start if none is provided.
- ACME (Let's Encrypt) certificate provisioning and renewal is supported via configuration.
- HTTP requests are permanently redirected to HTTPS.

### 5.4 Data Locality

- No telemetry, usage data, or file metadata is transmitted to any external service.
- All configuration, file manifests, version history, and audit logs remain within the user's deployment.

---

## 6. Notifications

### 6.1 Notification events

The following events shall be configurable to trigger notifications:

- Job completed successfully.
- Job completed with errors.
- Job failed (no files transferred due to error).
- Conflict detected and queued.
- Conflict not resolved within grace period.
- Soft-deleted file approaching permanent deletion.
- Disk space for version history below configured threshold.

### 6.2 Notification channels

| Channel | Notes |
|---|---|
| Email (SMTP) | Configurable SMTP server, sender address, and recipient list. |
| Webhook | HTTP POST to a configurable URL with a JSON payload. Compatible with Slack and Microsoft Teams incoming webhooks. |
| Gotify | Native Gotify push notification support for self-hosted alerting. |
| Pushover | Pushover API integration. |

Notification channels are configured globally and may be enabled or disabled per event type and per job.

---

## 7. Non-Functional Requirements

### 7.1 Performance

- The server must be able to index and maintain a file manifest for a directory tree of 1,000,000+ files without degradation in UI responsiveness.
- Directory scanning shall be parallelised across subdirectories.
- The web UI must remain responsive during active sync jobs — progress updates are streamed via WebSocket and must not block the UI thread.

### 7.2 Reliability

- All sync operations must be safe to interrupt at any point without data corruption or inconsistent state.
- The file manifest and job state must be updated transactionally; a crash during a sync run must leave the system in a consistent, resumable state.
- The application must start cleanly and resume all scheduled jobs automatically after a container restart.

### 7.3 Compatibility

- Server images must be published for `linux/amd64` and `linux/arm64`.
- The web UI must support the current and previous major version of Chrome, Firefox, Safari, and Edge.

### 7.4 Observability

- A `/health` HTTP endpoint returns application status, database connectivity, and version, suitable for use with uptime monitors (e.g. Uptime Kuma, Healthchecks.io).
- Structured JSON logs are written to stdout for consumption by log aggregators.
- Optional Prometheus metrics endpoint (`/metrics`) for integration with Grafana dashboards.

### 7.5 Upgrades

- Database schema migrations run automatically on startup.
- The upgrade path between any two consecutive releases must not require manual intervention beyond updating the Docker image tag.
- Breaking configuration changes between major versions must be documented with a migration guide.

---

## 8. Configuration Reference

All settings are configurable via environment variables or the `tidemarq.yaml` config file. The config file takes precedence over environment variables. A subset of settings is also manageable through the UI; UI changes are persisted to the config file.

| Category | Key settings | UI-exposed |
|---|---|---|
| Server | Bind address, port, TLS cert/key paths, ACME domain | No — config file / env only |
| Database | Type (sqlite/postgres), connection string, backup schedule | No — config file / env only |
| Storage | Metadata store path, max version history size | No — config file / env only |
| Auth | Session expiry, OIDC provider URL, client ID/secret | No — config file / env only |
| Notifications | SMTP settings, webhook URLs, Gotify/Pushover credentials | No — config file / env only |
| Users | User accounts, passwords, role assignments | Yes — Settings › Users |
| Sync | Default polling interval, default bandwidth limit, default conflict strategy, retention periods | Yes — Settings › Sync Defaults |

---

## 9. Glossary

| Term | Definition |
|---|---|
| Sync job | A configured pairing of a source and destination directory with associated rules, schedule, and settings. |
| File manifest | The stored record of each file's path, size, hash, and modification time at the point of the last successful sync. |
| Conflict | A state where a file has been modified on both sides since the last sync. |
| Soft delete | The act of moving a deleted file to quarantine rather than permanently removing it. |
| Delta sync | A transfer method that sends only the changed portions of a file rather than the entire file. |
| Network mount | A named remote filesystem location (SMB, NFS, or SFTP) configured in tidemarq and available as a path in job configuration. |
| Metadata store | The on-disk area managed by tidemarq for version history snapshots and soft-deleted files. |
