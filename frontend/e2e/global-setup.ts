/**
 * Playwright global setup — runs once before any test worker starts.
 *
 * Creates the four fixture jobs in the DB (if they don't already exist) and
 * runs them to produce the DB state that the skipped E2E tests depend on:
 *   01 - Simple Backup   → needed by job-execution progress test
 *   11 - Idempotency     → needed by idempotency test (test runs the job itself)
 *   02 - Mirror + Quarantine → needed by quarantine tests
 *   08 - Conflict Ask User   → needed by conflict resolution test
 *
 * If the backend is unreachable the setup logs a warning and exits cleanly;
 * the individual tests will skip themselves via their own guard clauses.
 */

import { request as playwrightRequest } from '@playwright/test'
import * as fs from 'node:fs'
import * as path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

const BACKEND_URL = process.env.TIDEMARQ_URL ?? 'https://localhost:8443'
const ADMIN_USER  = process.env.TIDEMARQ_ADMIN_USER ?? 'admin'
const ADMIN_PASS  = process.env.TIDEMARQ_ADMIN_PASSWORD ?? 'admin'

// Fixture source directories — already on disk, not modified by setup.
const FIXTURES = path.resolve(__dirname, '../../backend/dev-data/test-fixtures')

// Writable destinations — created fresh by this script, gitignored with dev-data.
const E2E_DEST = path.resolve(__dirname, '../../backend/dev-data/e2e-dest')

function sleep(ms: number) { return new Promise(r => setTimeout(r, ms)) }

async function waitIdle(ctx: Awaited<ReturnType<typeof playwrightRequest.newContext>>, token: string, jobId: number, timeoutMs = 60_000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    await sleep(1000)
    const r = await ctx.get(`${BACKEND_URL}/api/v1/jobs/${jobId}`, {
      headers: { Authorization: `Bearer ${token}` },
    })
    const j = await r.json()
    if (j.status === 'idle' || j.status === 'error') return j.status
  }
  return 'timeout'
}

async function findOrCreate(
  ctx: Awaited<ReturnType<typeof playwrightRequest.newContext>>,
  token: string,
  name: string,
  payload: Record<string, unknown>,
): Promise<number> {
  const headers = { Authorization: `Bearer ${token}` }
  const list = await (await ctx.get(`${BACKEND_URL}/api/v1/jobs`, { headers })).json() as Array<{ id: number; name: string }>
  const existing = list.find(j => j.name === name)
  if (existing) return existing.id
  const r = await ctx.post(`${BACKEND_URL}/api/v1/jobs`, { headers, data: payload })
  return ((await r.json()) as { id: number }).id
}

async function runAndWait(
  ctx: Awaited<ReturnType<typeof playwrightRequest.newContext>>,
  token: string,
  jobId: number,
) {
  await ctx.post(`${BACKEND_URL}/api/v1/jobs/${jobId}/run`, {
    headers: { Authorization: `Bearer ${token}` },
  })
  return waitIdle(ctx, token, jobId)
}

function jobPayload(name: string, src: string, dst: string, mode: string, strategy = 'ask-user'): Record<string, unknown> {
  return {
    name,
    source_path:        src,
    destination_path:   dst,
    mode,
    conflict_strategy:  strategy,
    watch_enabled:      false,
    cron_schedule:      '',
    bandwidth_limit_kb: 0,
    full_checksum:      false,
    hash_algo:          'blake3',
    use_delta:          false,
  }
}

export default async function globalSetup() {
  const ctx = await playwrightRequest.newContext({ ignoreHTTPSErrors: true })

  // ── login ──────────────────────────────────────────────────────────────────
  const loginResp = await ctx.post(`${BACKEND_URL}/api/v1/auth/login`, {
    data: { username: ADMIN_USER, password: ADMIN_PASS },
  })
  if (!loginResp.ok()) {
    console.warn('\n[global-setup] Backend unreachable — fixture jobs not seeded; data-dependent tests will skip.\n')
    await ctx.dispose()
    return
  }
  const token = ((await loginResp.json()) as { token: string }).token
  const headers = { Authorization: `Bearer ${token}` }

  // ── 01 Simple Backup ──────────────────────────────────────────────────────
  // Source files already exist; destination is an empty dir the engine writes into.
  const dst01 = path.join(E2E_DEST, '01')
  fs.mkdirSync(dst01, { recursive: true })

  const id01 = await findOrCreate(ctx, token, '01 - Simple Backup',
    jobPayload('01 - Simple Backup',
      path.join(FIXTURES, '01-backup-simple', 'source'), dst01,
      'one-way-backup'))
  await runAndWait(ctx, token, id01)
  console.log('[global-setup] 01 - Simple Backup: done')

  // ── 11 Idempotency ────────────────────────────────────────────────────────
  // The idempotency test triggers the job itself — we only need the job row to exist.
  const dst11 = path.join(E2E_DEST, '11')
  fs.mkdirSync(dst11, { recursive: true })

  await findOrCreate(ctx, token, '11 - Idempotency',
    jobPayload('11 - Idempotency',
      path.join(FIXTURES, '11-idempotency', 'source'), dst11,
      'one-way-backup'))
  console.log('[global-setup] 11 - Idempotency: job created (test will run it)')

  // ── 02 Mirror → Quarantine ────────────────────────────────────────────────
  // Only run if the quarantine table is currently empty.
  const quarResp = await ctx.get(`${BACKEND_URL}/api/v1/quarantine?status=active`, { headers })
  const quarantine = quarResp.ok() ? (await quarResp.json() as unknown[]) : []

  if (quarantine.length === 0) {
    // Destination has one extra file the source doesn't — it will be quarantined.
    const dst02 = path.join(E2E_DEST, '02')
    fs.rmSync(dst02, { recursive: true, force: true })   // fresh state
    fs.mkdirSync(dst02, { recursive: true })

    // Destination starts with only the orphan file.
    // The mirror job will copy source files across AND quarantine orphan-e2e.txt
    // because it exists in dest but not in source.
    const src02 = path.join(FIXTURES, '02-mirror-soft-delete', 'source')
    fs.writeFileSync(path.join(dst02, 'orphan-e2e.txt'), 'not in source — will be quarantined')

    const id02 = await findOrCreate(ctx, token, '02 - Mirror Soft Delete',
      jobPayload('02 - Mirror Soft Delete', src02, dst02, 'one-way-mirror'))
    await runAndWait(ctx, token, id02)
    console.log('[global-setup] 02 - Mirror Soft Delete: quarantine entries created')
  } else {
    console.log(`[global-setup] Quarantine already has ${quarantine.length} entries — skipping 02`)
  }

  // ── 08 Two-way → Conflict ─────────────────────────────────────────────────
  // Only run if there are no pending conflicts.
  const confResp = await ctx.get(`${BACKEND_URL}/api/v1/conflicts?status=pending`, { headers })
  const pending = confResp.ok() ? (await confResp.json() as unknown[]) : []

  if (pending.length === 0) {
    const src08 = path.join(E2E_DEST, '08-src')
    const dst08 = path.join(E2E_DEST, '08-dst')
    fs.rmSync(src08, { recursive: true, force: true })
    fs.rmSync(dst08, { recursive: true, force: true })
    fs.mkdirSync(src08, { recursive: true })
    fs.mkdirSync(dst08, { recursive: true })

    // Seed with a shared file on the source.
    fs.writeFileSync(path.join(src08, 'shared.txt'), 'initial content — baseline sync')

    const id08 = await findOrCreate(ctx, token, '08 - Conflict Ask User',
      jobPayload('08 - Conflict Ask User', src08, dst08, 'two-way', 'ask-user'))

    // Run 1: establishes the manifest (last-synced hash for shared.txt).
    await runAndWait(ctx, token, id08)

    // Modify shared.txt on BOTH sides with different content.
    // The engine will see: src_hash ≠ last_synced AND dst_hash ≠ last_synced → conflict.
    await sleep(200) // ensure distinct mtimes
    fs.writeFileSync(path.join(src08, 'shared.txt'), `source edit — ${Date.now()}`)
    await sleep(200)
    fs.writeFileSync(path.join(dst08, 'shared.txt'), `destination edit — ${Date.now()}`)

    // Run 2: detects the conflict and queues it for manual resolution.
    await runAndWait(ctx, token, id08)
    console.log('[global-setup] 08 - Conflict Ask User: pending conflict created')
  } else {
    console.log(`[global-setup] Conflicts already has ${pending.length} pending — skipping 08`)
  }

  await ctx.dispose()
  console.log('[global-setup] Done.\n')
}
