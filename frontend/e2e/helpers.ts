import { Page } from '@playwright/test'

const BASE_URL = process.env.TIDEMARQ_URL ?? 'https://localhost:8717'
const ADMIN_USER = process.env.TIDEMARQ_ADMIN_USER ?? 'admin'
const ADMIN_PASS = process.env.TIDEMARQ_ADMIN_PASSWORD ?? 'admin'

/**
 * Log in and store auth state for the session.
 * Call this in test.beforeEach or test.use({ storageState }) for speed.
 */
export async function login(page: Page) {
  await page.goto('/')
  await page.getByLabel(/username/i).fill(ADMIN_USER)
  await page.getByLabel(/password/i).fill(ADMIN_PASS)
  await page.getByRole('button', { name: /sign in/i }).click()
  // Wait for redirect away from login
  await page.waitForURL((url) => !url.pathname.includes('login'), { timeout: 5000 })
}

/**
 * Navigate to a named view via the sidebar.
 */
export async function nav(page: Page, label: RegExp | string) {
  await page.getByRole('navigation').getByText(label).click()
}

export { BASE_URL, ADMIN_USER, ADMIN_PASS }
