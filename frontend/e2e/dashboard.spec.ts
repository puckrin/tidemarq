import { test, expect } from '@playwright/test'
import { login, nav } from './helpers'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('shows summary stat cards', async ({ page }) => {
    await nav(page, /dashboard/i)
    // Stat cards: Total Jobs, Healthy, Errors, Pending Review
    await expect(page.locator('.stat-card').filter({ hasText: /total jobs/i })).toBeVisible()
    await expect(page.locator('.stat-card').filter({ hasText: /healthy/i })).toBeVisible()
    await expect(page.locator('.stat-card').filter({ hasText: /errors/i })).toBeVisible()
  })

  test('health endpoint reports ok', async ({ request }) => {
    const resp = await request.get('/health')
    expect(resp.ok()).toBeTruthy()
    const body = await resp.json()
    expect(body.database).toBe('ok')
    expect(body.version).toBeTruthy()
  })
})
