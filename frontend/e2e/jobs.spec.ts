import { test, expect } from '@playwright/test'
import { login, nav } from './helpers'
import { fileURLToPath } from 'url'
import * as path from 'path'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

// Adjust these paths to wherever dev-data/test-fixtures lives on the test machine.
// Override via env var for CI.
const FIXTURES = process.env.TIDEMARQ_FIXTURES_DIR
  ?? path.resolve(__dirname, '../../backend/dev-data/test-fixtures')

test.describe('Job management', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await nav(page, /sync jobs/i)
  })

  test('jobs list page loads', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /sync jobs/i })).toBeVisible()
  })

  test('new job wizard — creates a one-way-backup job', async ({ page }) => {
    await page.getByRole('button', { name: /new job/i }).click()

    // Step 1: name and mode
    await page.getByLabel(/job name/i).fill('E2E - Simple Backup')
    await page.getByLabel(/one.way backup/i).check()
    await page.getByRole('button', { name: /next/i }).click()

    // Step 2: source path
    await page.getByLabel(/source/i).fill(`${FIXTURES}/01-backup-simple/source`)
    await page.getByRole('button', { name: /next/i }).click()

    // Step 3: destination path
    await page.getByLabel(/destination/i).fill(`${FIXTURES}/01-backup-simple/destination-e2e`)
    await page.getByRole('button', { name: /next/i }).click()

    // Step 4: trigger (skip — leave as manual)
    await page.getByRole('button', { name: /next/i }).click()

    // Step 5: review and create
    await expect(page.getByText('E2E - Simple Backup')).toBeVisible()
    await page.getByRole('button', { name: /create job/i }).click()

    // Should return to jobs list with new job visible
    await expect(page.getByText('E2E - Simple Backup')).toBeVisible()
  })

  test('job detail page is reachable', async ({ page }) => {
    // Assumes at least one job exists (created by setup.sh or the wizard test above)
    const firstJob = page.getByRole('table').getByRole('row').nth(1)
    await firstJob.click()
    await expect(page.getByRole('heading')).toBeVisible()
  })
})

test.describe('Job execution — backup mode (Job 01)', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('running a backup job shows progress and completes', async ({ page }) => {
    await nav(page, /sync jobs/i)

    // Find "01 - Simple Backup" if it exists; skip if the fixture jobs aren't loaded
    const row = page.getByRole('row', { name: /01.*simple backup/i })
    if (await row.count() === 0) {
      test.skip()
      return
    }

    await row.getByRole('button', { name: /run/i }).click()

    // Progress should become visible, then job returns to idle
    await expect(page.getByText(/running/i)).toBeVisible({ timeout: 10000 })
    await expect(page.getByText(/idle/i)).toBeVisible({ timeout: 30000 })
  })
})

test.describe('Job execution — idempotency (Job 11)', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('idempotency job transfers zero files on second run', async ({ page, request }) => {
    // Find the job via API so we can run it directly
    const listResp = await request.get('/api/v1/jobs')
    if (!listResp.ok()) { test.skip(); return }

    const jobs: Array<{ id: number; name: string }> = await listResp.json()
    const job = jobs.find(j => /11.*idempoten/i.test(j.name))
    if (!job) { test.skip(); return }

    // Run the job twice via API
    for (let run = 1; run <= 2; run++) {
      const runResp = await request.post(`/api/v1/jobs/${job.id}/run`)
      expect(runResp.ok()).toBeTruthy()

      // Poll until idle
      await page.waitForTimeout(1000)
      for (let i = 0; i < 30; i++) {
        const s = await request.get(`/api/v1/jobs/${job.id}`)
        const data = await s.json()
        if (data.status === 'idle') break
        await page.waitForTimeout(1000)
      }

      // On the second run, audit log should show 0 files changed
      if (run === 2) {
        const auditResp = await request.get(`/api/v1/audit?job_id=${job.id}`)
        const audit: Array<{ event: string; detail: string }> = await auditResp.json()
        const lastRun = audit.find(e => e.event === 'job.completed')
        expect(lastRun?.detail).toMatch(/files_copied.*0|0.*files/i)
      }
    }
  })
})
