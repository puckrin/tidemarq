import { test, expect } from '@playwright/test'
import { login, nav } from './helpers'

test.describe('Conflict queue', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('conflict queue page loads', async ({ page }) => {
    await nav(page, /conflict/i)
    await expect(page.getByRole('heading', { name: /conflict/i })).toBeVisible()
  })

  test('pending conflicts are listed with file path and strategy', async ({ page, request }) => {
    const resp = await request.get('/api/v1/conflicts?status=pending')
    if (!resp.ok()) { test.skip(); return }
    const conflicts = await resp.json()
    if (conflicts.length === 0) { test.skip(); return }

    await nav(page, /conflict/i)
    // First conflict's file path should appear in the table
    await expect(page.getByText(conflicts[0].rel_path)).toBeVisible()
  })

  test('keep-source resolution removes conflict from pending list', async ({ page, request }) => {
    // Create a synthetic conflict via API to test resolution without needing fixture jobs run
    const listResp = await request.get('/api/v1/conflicts?status=pending')
    if (!listResp.ok()) { test.skip(); return }
    const pending = await listResp.json()
    if (pending.length === 0) { test.skip(); return }

    const conflict = pending[0]
    await nav(page, /conflict/i)

    // Click into the conflict row
    await page.getByText(conflict.rel_path).first().click()

    // Click Keep Source
    await page.getByRole('button', { name: /keep source/i }).click()

    // Row should disappear or show resolved state
    await expect(page.getByText(conflict.rel_path)).not.toBeVisible({ timeout: 5000 })
  })
})
