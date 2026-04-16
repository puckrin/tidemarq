# tidemarq

A self-hosted directory synchronisation tool with a browser-based UI. Runs as a Docker Compose stack with no external cloud dependencies.

---

## What tidemarq does

tidemarq lets you define sync jobs between directories — local folders, network shares (SMB/CIFS), or SFTP servers — and manage them from a browser.

**Sync jobs** are the core concept. Each job has a source, a destination, a mode, and a schedule:

- **One-way backup** — copies new and changed files from source to destination. Files deleted from the source are left in place at the destination.
- **One-way mirror** — keeps the destination an exact mirror of the source. Files removed from the source are soft-deleted (moved to quarantine, not permanently deleted).
- **Two-way sync** — propagates changes in both directions. Files changed on both sides since the last sync are flagged as conflicts for manual review.

**Scheduling** is flexible: jobs can run on a cron schedule, trigger automatically when the source directory changes (filesystem watch), or be kicked off manually from the UI.

**Conflict management** is built in. When two-way sync detects a file changed on both sides, it queues a conflict for you to resolve — keep the source version, keep the destination version, or keep both. Auto-resolution strategies (newest wins, largest wins, source wins, destination wins) are also available per job.

**Quarantine** replaces permanent deletion. When a mirror or two-way job would delete a file, it moves it to a quarantine store with a configurable retention period. Quarantined files can be restored or permanently removed from the UI.

**Audit log** records every job run, file operation, conflict, and user action with timestamps, so you always have a record of what changed and when.

**Mounts** let you configure named SMB and SFTP connections once and reuse them across jobs. Credentials are encrypted at rest.

**Notifications** send alerts (email, webhook, Gotify) when jobs fail, conflicts are detected, or quarantine entries are about to expire.

---

## How tidemarq transfers files

**Tools like rsync** use an efficient delta algorithm — only changed bytes travel the wire. But they require their own software running on both machines. Sync to an SFTP server or a shared SMB volume and you need rsync installed on the remote too.

**Tools like restic** need nothing on the remote, doing all the work on the client. But the destination is an opaque repository format. You can't mount your NAS and open a synced file directly — you have to restore it through restic first. It's a backup tool, not a sync tool.

**tidemarq** takes a different position. The destination is always a plain directory — open any file directly, mount it from another machine, copy files out manually. No proprietary format, no remote agent to install.

For local transfers, tidemarq uses a rolling-checksum delta algorithm: it signatures the existing destination file, diffs the source against it, and writes only the changed regions. Large files with small modifications transfer in seconds.

For network destinations (SFTP, SMB), tidemarq performs a full streaming copy. Without software on the remote there is no way to discover what has changed without reading the destination file first — downloading it to diff it would save bytes in one direction at the cost of the other, a poor trade on most connections.

Because tidemarq always reads what is actually on the destination before writing, it also handles the case where someone else modified a file on the NAS between syncs. There is no local cache of assumed state that can drift from reality.

Simple to deploy. Nothing to install on the remote. Always correct.

---

## Running tidemarq

```bash
# Full stack — UI available at https://localhost:8717
docker compose up --build

# Backend only (from backend/) — uses tidemarq.dev.yaml if present
go run ./cmd/tidemarq

# Frontend dev server (from frontend/) — proxies API to https://localhost:8443
npm install && npm run dev
```

### Ports

#### Docker / production (`tidemarq.example.yaml`)

| Port | Protocol | Purpose |
|------|----------|---------|
| 8716 | HTTP | Redirects to HTTPS |
| 8717 | HTTPS | Main UI and API |

#### Local development (`tidemarq.dev.yaml`)

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | Redirects to HTTPS |
| 8443 | HTTPS | Backend API |
| 5173 | HTTP | Vite dev server (frontend) |

### First start

On first start tidemarq will:
- Create a default admin account (username `admin`, password from config or `TIDEMARQ_ADMIN_PASSWORD`)
- Auto-generate a TLS certificate if none is configured
- Auto-generate a JWT signing secret stored in `<data_dir>/.jwt_secret`

No manual token generation is required.

---

## Documentation

- `sync_app_spec.md` — full functional specification
- `CLAUDE.md` — developer guide and architectural rules
