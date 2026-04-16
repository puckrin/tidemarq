# tidemarq

A self-hosted directory synchronisation tool with a browser-based UI. Runs as a Docker Compose stack with no external cloud dependencies.

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
