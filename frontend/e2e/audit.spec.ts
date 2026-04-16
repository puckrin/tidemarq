import { test, expect } from '@playwright/test'
import { login, nav } from './helpers'

test.describe('Audit log', () => {
  test.beforeEach(async ({ page }) => {
    await login(page)
  })

  test('audit log page loads and shows entries', async ({ page }) => {
    await nav(page, /audit/i)
    await expect(page.getByRole('heading', { name: /audit/i })).toBeVisible()
    // At minimum a login event should exist
    await expect(page.getByRole('table')).toBeVisible()
  })

  test('audit log API returns entries filterable by event', async ({ request }) => {
    const resp = await request.get('/api/v1/audit?event=auth.login')
    expect(resp.ok()).toBeTruthy()
    const entries = await resp.json()
    expect(Array.isArray(entries)).toBeTruthy()
    // Should have at least the login we just performed
    expect(entries.length).toBeGreaterThan(0)
  })

  test('audit log CSV export returns valid CSV', async ({ request }) => {
    const resp = await request.get('/api/v1/audit/export?format=csv')
    expect(resp.ok()).toBeTruthy()
    const text = await resp.text()
    // First line should be a header row
    const firstLine = text.split('\n')[0]
    expect(firstLine).toMatch(/id|event|created/i)
  })

  test('audit log JSON export returns valid JSON array', async ({ request }) => {
    const resp = await request.get('/api/v1/audit/export?format=json')
    expect(resp.ok()).toBeTruthy()
    const data = await resp.json()
    expect(Array.isArray(data)).toBeTruthy()
  })
})
