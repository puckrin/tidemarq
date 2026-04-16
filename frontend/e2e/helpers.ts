import { Page } from '@playwright/test'

const ADMIN_USER = process.env.TIDEMARQ_ADMIN_USER ?? 'admin'
const ADMIN_PASS = process.env.TIDEMARQ_ADMIN_PASSWORD ?? 'admin'

/**
 * Log in and wait for the sidebar (navigation) to confirm auth is complete.
 */
export async function login(page: Page) {
  await page.goto('/')
  await page.getByLabel(/username/i).fill(ADMIN_USER)
  await page.getByLabel(/password/i).fill(ADMIN_PASS)
  await page.getByRole('button', { name: /sign in/i }).click()
  // Wait for the sidebar to appear — confirms the login API call succeeded
  // and React has re-rendered the authenticated shell.
  await page.getByRole('navigation').waitFor({ state: 'visible', timeout: 8000 })
}

/**
 * Read the JWT from the page's localStorage so request fixtures can auth.
 */
export async function getToken(page: Page): Promise<string> {
  return (await page.evaluate(() => localStorage.getItem('token'))) ?? ''
}

/**
 * Navigate to a named view via the sidebar.
 */
export async function nav(page: Page, label: RegExp | string) {
  await page.getByRole('navigation').getByText(label).click()
}

export { ADMIN_USER, ADMIN_PASS }
