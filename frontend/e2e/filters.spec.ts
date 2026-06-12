import { test, expect } from '@playwright/test'
import { login, getToken } from './helpers'
import { fileURLToPath } from 'url'
import * as path from 'path'

const __dirname = fileURLToPath(new URL('.', import.meta.url))

// Reuse the same fixture root as jobs.spec.ts so we don't duplicate paths.
const FIXTURES = process.env.TIDEMARQ_FIXTURES_DIR
  ?? path.resolve(__dirname, '../../backend/dev-data/test-fixtures')

/**
 * Click Next n times. The wizard renders one Next button at a time so we
 * grab a fresh handle each click — the previous one detaches after the
 * step transition.
 */
async function next(page: any, n = 1) {
  for (let i = 0; i < n; i++) {
    await page.getByRole('button', { name: /^next/i }).click()
  }
}

/**
 * Walk through steps 1–3 with sane defaults so the test for steps 4+ starts
 * from the Filters step. Optionally override the job name.
 */
async function openWizardToFilters(page: any, name = 'E2E - Filtered') {
  await page.getByRole('button', { name: /new job/i }).click()

  // Step 1
  await page.getByLabel(/job name/i).fill(name)
  await page.locator('input[placeholder="/local/path"]').fill(`${FIXTURES}/01-backup-simple/source`)
  await next(page)

  // Step 2
  await page.locator('input[placeholder="/local/path"]').fill(`${FIXTURES}/01-backup-simple/destination-filters-e2e`)
  await next(page)

  // Step 3 — keep default one-way-backup
  await next(page)

  await expect(page.getByText(/Step 4.*File Filters/i)).toBeVisible()
}

test.describe('File-filter wizard (§3.3)', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
    await page.getByRole('navigation').getByText(/sync jobs/i).click()
  })

  test('empty filter step shows the no-rules hint', async ({ page }) => {
    await openWizardToFilters(page, 'E2E - No Filters')
    await expect(page.getByText(/No rules — every file/i)).toBeVisible()
    await expect(page.getByRole('button', { name: /add rule/i })).toBeVisible()
    // Cancel out so we don't pollute the jobs list.
    await page.getByRole('button', { name: /cancel/i }).click()
  })

  test('add rule reveals an Exclude + Glob row with a pattern input', async ({ page }) => {
    await openWizardToFilters(page, 'E2E - Add Rule')
    await page.getByRole('button', { name: /add rule/i }).click()
    await expect(page.getByLabel('Action').first()).toHaveValue('exclude')
    await expect(page.getByLabel('Rule type').first()).toHaveValue('glob')
    await expect(page.getByLabel('Glob pattern')).toBeVisible()
    // The no-rules hint must disappear once a rule exists.
    await expect(page.getByText(/No rules — every file/i)).not.toBeVisible()
    await page.getByRole('button', { name: /cancel/i }).click()
  })

  test('switching rule type swaps the input fields', async ({ page }) => {
    await openWizardToFilters(page, 'E2E - Switch Type')
    await page.getByRole('button', { name: /add rule/i }).click()
    await page.getByLabel('Rule type').first().selectOption('size')
    await expect(page.getByLabel('Size above (bytes)')).toBeVisible()
    await expect(page.getByLabel('Size below (bytes)')).toBeVisible()
    await expect(page.getByLabel('Glob pattern')).not.toBeVisible()

    await page.getByLabel('Rule type').first().selectOption('modified')
    await expect(page.getByLabel('Modified before (days)')).toBeVisible()
    await expect(page.getByLabel('Modified within (days)')).toBeVisible()
    await expect(page.getByLabel('Size above (bytes)')).not.toBeVisible()

    await page.getByRole('button', { name: /cancel/i }).click()
  })

  test('Review step summarises the filter selection', async ({ page }) => {
    await openWizardToFilters(page, 'E2E - Review Summary')
    // Build a small ruleset: exclude *.log, exclude hidden, no other rules.
    await page.getByLabel(/Exclude hidden files/i).check()
    await page.getByRole('button', { name: /add rule/i }).click()
    await page.getByLabel('Rule type').first().selectOption('extension')
    await page.getByLabel('Extension').fill('.log')

    await next(page) // 4 -> 5 (Schedule)
    await next(page) // 5 -> 6 (Review)

    // The summary row reads "Filters: exclude hidden · 1 rule"
    await expect(page.getByText(/exclude hidden/i)).toBeVisible()
    await expect(page.getByText(/1 rule/i)).toBeVisible()
    await page.getByRole('button', { name: /cancel/i }).click()
  })

  // End-to-end create with filters, then verify via API that the persisted
  // ruleset matches the wizard inputs. This is the highest-value test in
  // this file: it exercises the wire format and the persistence path
  // through every layer.
  test('creating a job with filters persists them via the API', async ({ page, request }) => {
    await openWizardToFilters(page, 'E2E - Filter Persist')

    // Exclude *.log and node_modules.
    await page.getByRole('button', { name: /add rule/i }).click()
    await page.getByLabel('Rule type').first().selectOption('extension')
    await page.getByLabel('Extension').fill('.log')

    await page.getByRole('button', { name: /add rule/i }).click()
    // Second rule defaults to a glob; just fill in the pattern.
    await page.getByLabel('Glob pattern').fill('**/node_modules/**')

    await next(page) // -> 5
    await next(page) // -> 6
    await page.getByRole('button', { name: /create job/i }).click()
    await expect(page.locator('.bc')).toBeVisible({ timeout: 8000 })

    // Read back via API and confirm the ruleset round-tripped intact.
    const token = await getToken(page)
    const listResp = await request.get('/api/v1/jobs', { headers: { Authorization: `Bearer ${token}` } })
    expect(listResp.ok()).toBeTruthy()
    const jobs: Array<{ id: number; name: string; filters?: any }> = await listResp.json()
    const created = jobs.find(j => j.name === 'E2E - Filter Persist')
    expect(created).toBeDefined()
    expect(created?.filters?.rules).toHaveLength(2)
    expect(created?.filters?.rules?.[0]).toMatchObject({
      type: 'extension', action: 'exclude', pattern: '.log',
    })
    expect(created?.filters?.rules?.[1]).toMatchObject({
      type: 'glob', action: 'exclude', pattern: '**/node_modules/**',
    })

    // Clean up so the test is rerunnable.
    await request.delete(`/api/v1/jobs/${created!.id}`, { headers: { Authorization: `Bearer ${token}` } })
  })

  // Invalid rule (modified-date with no fields) must surface a 400 the
  // wizard treats as a save failure — the user must not be navigated to
  // the job detail page on a 400, and the persisted state must be empty.
  test('saving a malformed rule shows the failure toast and does not persist', async ({ page, request }) => {
    await openWizardToFilters(page, 'E2E - Bad Filter')

    await page.getByRole('button', { name: /add rule/i }).click()
    await page.getByLabel('Rule type').first().selectOption('modified')
    // Leave both day fields blank — this is the invalid case.

    await next(page) // -> 5
    await next(page) // -> 6
    await page.getByRole('button', { name: /create job/i }).click()

    // The Review panel surfaces the failure inline; a toast also fires.
    // Use `.first()` because both appear simultaneously and strict-mode
    // would otherwise treat the duplicate as a locator error.
    await expect(page.getByText(/Failed to create job/i).first()).toBeVisible({ timeout: 8000 })

    // Confirm nothing was persisted.
    const token = await getToken(page)
    const listResp = await request.get('/api/v1/jobs', { headers: { Authorization: `Bearer ${token}` } })
    const jobs: Array<{ name: string }> = await listResp.json()
    expect(jobs.find(j => j.name === 'E2E - Bad Filter')).toBeUndefined()
  })
})
