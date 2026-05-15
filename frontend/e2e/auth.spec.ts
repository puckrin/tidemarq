import { test, expect } from '@playwright/test'
import { ADMIN_USER, TEST_PASS } from './helpers'

test.describe('Authentication', () => {
  test('login page is shown when unauthenticated', async ({ page }) => {
    await page.goto('/')
    await expect(page.getByText('tidemarq')).toBeVisible()
    await expect(page.getByLabel(/username/i)).toBeVisible()
    await expect(page.getByLabel(/password/i)).toBeVisible()
  })

  test('invalid credentials show an error', async ({ page }) => {
    await page.goto('/')
    await page.getByLabel(/username/i).fill(ADMIN_USER)
    await page.getByLabel(/password/i).fill('wrongpassword')
    await page.getByRole('button', { name: /sign in/i }).click()
    await expect(page.getByText('Invalid username or password.')).toBeVisible()
  })

  test('valid credentials navigate to dashboard', async ({ page }) => {
    // Uses TEST_PASS: global-setup rotates the seeded admin onto this value
    // (and clears password_change_required) before any test runs, so login
    // here should drop the user straight into the authenticated shell rather
    // than the ChangePasswordView.
    await page.goto('/')
    await page.getByLabel(/username/i).fill(ADMIN_USER)
    await page.getByLabel(/password/i).fill(TEST_PASS)
    await page.getByRole('button', { name: /sign in/i }).click()
    await expect(page).toHaveURL(/\/(dashboard)?$/)
    await expect(page.locator('.page-title').first()).toBeVisible()
  })

  test('protected routes redirect to login when unauthenticated', async ({ page }) => {
    // Navigate directly without a token
    await page.goto('/jobs')
    await expect(page.getByLabel(/username/i)).toBeVisible()
  })
})
