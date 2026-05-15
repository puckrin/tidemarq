import { Page } from '@playwright/test'

// Two distinct credentials need to exist for the e2e suite:
//
//   ADMIN_PASS — the password the backend was seeded with (default "admin123").
//                Only used by global-setup, and only on the very first run
//                against a fresh backend that has the seeded admin's
//                password_change_required flag still set to true.
//
//   TEST_PASS  — the password every test logs in with. Global-setup rotates
//                the admin from ADMIN_PASS to TEST_PASS on first run (which
//                clears the flag at the same time). Subsequent runs find the
//                admin already on TEST_PASS and skip the rotation.
//
// Both must be at least 8 characters so the backend's password-policy check
// in handleChangePassword accepts them. They must also differ from each
// other — the backend rejects new == current.
export const ADMIN_USER = process.env.TIDEMARQ_ADMIN_USER     ?? 'admin'
export const ADMIN_PASS = process.env.TIDEMARQ_ADMIN_PASSWORD ?? 'admin123'
export const TEST_PASS  = process.env.TIDEMARQ_TEST_PASSWORD  ?? 'tidemarq-test-12345'

/**
 * Log in and wait for the sidebar (navigation) to confirm auth is complete.
 * Uses TEST_PASS, which is what the backend admin's password becomes after
 * global-setup runs.
 */
export async function login(page: Page) {
  await page.goto('/')
  await page.getByLabel(/username/i).fill(ADMIN_USER)
  await page.getByLabel(/password/i).fill(TEST_PASS)
  await page.getByRole('button', { name: /sign in/i }).click()
  // Wait for the sidebar to appear — confirms the login API call succeeded
  // and React has re-rendered the authenticated shell. If the admin is
  // somehow still flagged for forced-change, the navigation will never appear
  // and this will time out with a clear failure.
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
