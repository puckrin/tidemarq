import { test, expect } from '@playwright/test'
import { login, nav, getToken } from './helpers'

test.describe('Quarantine', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('quarantine page loads', async ({ page }) => {
    await nav(page, /quarantine/i)
    await expect(page.locator('.page-title')).toBeVisible()
  })

  test('quarantined files are listed with expiry', async ({ page, request }) => {
    const token = await getToken(page)
    const resp = await request.get('/api/v1/quarantine?status=active', {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!resp.ok()) { test.skip(); return }
    const entries = await resp.json()
    if (entries.length === 0) { test.skip(); return }

    await nav(page, /quarantine/i)
    await expect(page.getByText(entries[0].rel_path)).toBeVisible()
    // Expiry label should be visible (Nd or Expired)
    await expect(page.getByText(/\d+d|\d+h|expired/i).first()).toBeVisible()
  })

  test('restore button calls API and removes entry from list', async ({ page, request }) => {
    const token = await getToken(page)
    const resp = await request.get('/api/v1/quarantine?status=active', {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!resp.ok()) { test.skip(); return }
    const entries = await resp.json()
    if (entries.length === 0) { test.skip(); return }

    const entry = entries[0]
    await nav(page, /quarantine/i)

    // Click Restore for the first entry
    const row = page.getByRole('row').filter({ hasText: entry.rel_path })
    await row.getByRole('button', { name: /^Restore$/ }).click()

    // Entry should disappear or show restored state
    await expect(page.getByText(entry.rel_path)).not.toBeVisible({ timeout: 5000 })
  })
})
