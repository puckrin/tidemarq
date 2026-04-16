import { test, expect } from '@playwright/test'

test.describe('Authentication', () => {
  test('login page is shown when unauthenticated', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByRole('heading', { name: /tidemarq/i })).toBeVisible()
    await expect(page.getByLabel(/username/i)).toBeVisible()
    await expect(page.getByLabel(/password/i)).toBeVisible()
  })

  test('invalid credentials show an error', async ({ page }) => {
    await page.goto('/')
    await page.getByLabel(/username/i).fill('admin')
    await page.getByLabel(/password/i).fill('wrongpassword')
    await page.getByRole('button', { name: /sign in/i }).click()
    await expect(page.getByText(/invalid|incorrect|unauthorized/i)).toBeVisible()
  })

  test('valid credentials navigate to dashboard', async ({ page }) => {
    await page.goto('/')
    await page.getByLabel(/username/i).fill('admin')
    await page.getByLabel(/password/i).fill(process.env.TIDEMARQ_ADMIN_PASSWORD ?? 'admin')
    await page.getByRole('button', { name: /sign in/i }).click()
    await expect(page).toHaveURL(/\/(dashboard)?$/)
    await expect(page.getByText(/dashboard/i)).toBeVisible()
  })

  test('protected routes redirect to login when unauthenticated', async ({ page }) => {
    // Navigate directly without a token
    await page.goto('/jobs')
    await expect(page.getByLabel(/username/i)).toBeVisible()
  })
})
