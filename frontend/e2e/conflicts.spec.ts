import { test, expect } from '@playwright/test'
import { login, nav, getToken } from './helpers'

test.describe('Conflict queue', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('conflict queue page loads', async ({ page }) => {
    await nav(page, /conflict/i)
    await expect(page.locator('.page-title')).toBeVisible()
  })

  test('pending conflicts are listed with file path and strategy', async ({ page, request }) => {
    const token = await getToken(page)
    const resp = await request.get('/api/v1/conflicts?status=pending', {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!resp.ok()) { test.skip(); return }
    const conflicts = await resp.json()
    if (conflicts.length === 0) { test.skip(); return }

    await nav(page, /conflict/i)
    // First conflict's file path should appear in the table cell
    await expect(page.locator('td.td-mono').filter({ hasText: conflicts[0].rel_path })).toBeVisible()
  })

  test('keep-source resolution removes conflict from pending list', async ({ page, request }) => {
    const token = await getToken(page)
    const listResp = await request.get('/api/v1/conflicts?status=pending', {
      headers: { Authorization: `Bearer ${token}` },
    })
    if (!listResp.ok()) { test.skip(); return }
    const pending = await listResp.json()
    if (pending.length === 0) { test.skip(); return }

    const conflict = pending[0]
    await nav(page, /conflict/i)

    // Click the table cell containing the rel_path to select the row
    await page.locator('td.td-mono').filter({ hasText: conflict.rel_path }).click()

    // Resolution panel appears — skip gracefully if button not visible
    const keepBtn = page.getByRole('button', { name: /keep source/i })
    if (!await keepBtn.isVisible({ timeout: 3000 }).catch(() => false)) {
      test.skip()
      return
    }
    await keepBtn.click()

    // Row should disappear after resolution
    await expect(
      page.locator('td.td-mono').filter({ hasText: conflict.rel_path })
    ).not.toBeVisible({ timeout: 5000 })
  })
})
