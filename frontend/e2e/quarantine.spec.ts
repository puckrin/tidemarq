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
    // `.first()` because the same filename may also appear in "Recently
    // Removed" rows from prior runs — both are valid evidence that the
    // entry has rendered.
    await expect(page.getByText(entries[0].rel_path).first()).toBeVisible()
    // Expiry label should be visible (Nd or Expired)
    await expect(page.getByText(/\d+d|\d+h|expired/i).first()).toBeVisible()
  })

  test('restore button calls API and removes entry from list', async ({ page, request }) => {
    const token = await getToken(page)
    const headers = { Authorization: `Bearer ${token}` }
    const resp = await request.get('/api/v1/quarantine?status=active', { headers })
    if (!resp.ok()) { test.skip(); return }
    const entries = await resp.json()
    if (entries.length === 0) { test.skip(); return }

    const entry = entries[0]
    await nav(page, /quarantine/i)

    // Scope to the ACTIVE row by requiring the row to contain a Restore
    // button. After restore, the filename also appears in "Recently
    // Removed" — a plain `getByRole('row')` would match both and stay
    // visible even after the operation succeeds.
    const activeRow = page.getByRole('row').filter({ hasText: entry.rel_path })
      .filter({ has: page.getByRole('button', { name: 'Restore', exact: true }) })
    await expect(activeRow).toBeVisible({ timeout: 8000 })
    await activeRow.getByRole('button', { name: 'Restore', exact: true }).click()

    // Restore is "done" when the active row disappears. The recently-removed
    // entry with the same filename does NOT have a Restore button, so the
    // scoped locator no longer matches.
    await expect(activeRow).not.toBeVisible({ timeout: 8000 })

    // Cross-check the API: the backend marks the entry status='restored',
    // so a subsequent active fetch must not return this id.
    const after = await (await request.get('/api/v1/quarantine?status=active', { headers })).json() as Array<{ id: number }>
    expect(after.find(e => e.id === entry.id)).toBeUndefined()
  })
})
