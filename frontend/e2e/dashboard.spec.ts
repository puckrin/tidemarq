import { test, expect } from '@playwright/test'
import { login, nav } from './helpers'

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('shows summary stat cards', async ({ page }) => {
    await nav(page, /dashboard/i)
    // Stat cards for jobs, conflicts, quarantine
    await expect(page.getByText(/sync jobs/i)).toBeVisible()
    await expect(page.getByText(/conflicts/i)).toBeVisible()
    await expect(page.getByText(/quarantine/i)).toBeVisible()
  })

  test('health endpoint reports ok', async ({ request }) => {
    const resp = await request.get('/health')
    expect(resp.ok()).toBeTruthy()
    const body = await resp.json()
    expect(body.database).toBe('ok')
    expect(body.version).toBeTruthy()
  })
})
