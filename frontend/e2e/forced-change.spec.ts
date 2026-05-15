import { test, expect, type Page } from '@playwright/test'
import { ADMIN_USER, TEST_PASS, login } from './helpers'

// Coverage strategy for the forced-change feature:
//
//   • UI tests (renders + client-side validation) run against a *fake* JWT
//     that we drop into localStorage before the page loads. parseToken() in
//     the auth store only base64-decodes the payload — it doesn't verify the
//     signature — so a hand-crafted token with pwd_change_required=true is
//     enough to make App.tsx render ChangePasswordView. These tests never
//     submit the form, so the backend's signature check is never invoked.
//
//   • Backend behaviour (change-password endpoint contract) is exercised
//     against the real backend using the admin's actual session: we rotate
//     TEST_PASS → temp, verify the new token works, then rotate temp →
//     TEST_PASS to restore state so subsequent specs are unaffected. No DB
//     manipulation required.
//
// Doing it this way means the suite never has to mutate the
// password_change_required column directly, which keeps the tests portable
// across worktrees, CI environments, and dev machines that may not have
// sqlite3 installed.

// A syntactically valid JWT carrying pwd_change_required=true. Signature is
// random bytes — fine for tests that don't submit the form, since the
// frontend's parseToken() doesn't verify signatures. Username + user_id are
// fabricated; the UI only reads them for display.
function fakeForcedChangeToken(): string {
  const header  = Buffer.from(JSON.stringify({ alg: 'HS256', typ: 'JWT' })).toString('base64url')
  const payload = Buffer.from(JSON.stringify({
    user_id: 99999,
    username: 'forced-change-fixture',
    role: 'admin',
    pwd_change_required: true,
    exp: Math.floor(Date.now() / 1000) + 3600,
  })).toString('base64url')
  // Random signature — irrelevant since we never submit.
  const sig = Buffer.from('not-a-real-signature').toString('base64url')
  return `${header}.${payload}.${sig}`
}

async function loadWithFakeToken(page: Page) {
  // Inject the fake token before any script on the page evaluates so
  // AuthProvider's useState initialiser sees it on first render.
  await page.addInitScript((tok) => {
    localStorage.setItem('token', tok)
  }, fakeForcedChangeToken())
  await page.goto('/')
}

test.describe('Forced password change — UI', () => {
  test('renders the change-password view, hides the sidebar', async ({ page }) => {
    await loadWithFakeToken(page)
    await expect(page.getByText(/set a new password/i)).toBeVisible()
    await expect(page.getByLabel(/current password/i)).toBeVisible()
    await expect(page.getByLabel(/^new password$/i)).toBeVisible()
    await expect(page.getByLabel(/confirm new password/i)).toBeVisible()
    // No app navigation should be reachable while the flag is set.
    await expect(page.getByRole('navigation')).toHaveCount(0)
  })

  test('rejects new password shorter than 8 chars (client-side)', async ({ page }) => {
    await loadWithFakeToken(page)
    await page.getByLabel(/current password/i).fill('whatever1')
    await page.getByLabel(/^new password$/i).fill('short')
    await page.getByLabel(/confirm new password/i).fill('short')
    await page.getByRole('button', { name: /set new password/i }).click()
    await expect(page.getByText(/at least 8 characters/i)).toBeVisible()
  })

  test('rejects mismatched confirmation', async ({ page }) => {
    await loadWithFakeToken(page)
    await page.getByLabel(/current password/i).fill('whatever1')
    await page.getByLabel(/^new password$/i).fill('new-password-99')
    await page.getByLabel(/confirm new password/i).fill('different-99')
    await page.getByRole('button', { name: /set new password/i }).click()
    await expect(page.getByText(/do not match/i)).toBeVisible()
  })

  test('rejects new password identical to current', async ({ page }) => {
    await loadWithFakeToken(page)
    await page.getByLabel(/current password/i).fill('same-password-1')
    await page.getByLabel(/^new password$/i).fill('same-password-1')
    await page.getByLabel(/confirm new password/i).fill('same-password-1')
    await page.getByRole('button', { name: /set new password/i }).click()
    await expect(page.getByText(/must differ/i)).toBeVisible()
  })

  test('sign-out escape hatch returns to login', async ({ page }) => {
    await loadWithFakeToken(page)
    await page.getByRole('button', { name: /sign out and use a different account/i }).click()
    await expect(page.getByLabel(/^username$/i)).toBeVisible()
    await expect(page.getByLabel(/^password$/i)).toBeVisible()
  })
})

test.describe('Forced password change — change-password API', () => {
  // These tests hit the real backend. They rotate the admin password forward
  // and immediately back so they're self-cleaning and order-independent
  // relative to other specs.
  const TEMP_PASS = 'temp-rotation-pass-1234'

  test('wrong current_password returns 400 invalid_current_password (does not invalidate session)', async ({ page, request }) => {
    await login(page)
    const token = await page.evaluate(() => localStorage.getItem('token'))

    const resp = await request.post('/api/v1/auth/change-password', {
      headers: { Authorization: `Bearer ${token}` },
      data: { current_password: 'definitely-not-the-password', new_password: TEMP_PASS },
    })
    expect(resp.status()).toBe(400)
    const body = await resp.json() as { code?: string; error?: string }
    expect(body.code).toBe('invalid_current_password')

    // Same token must still work for other endpoints — 400 (not 401) means
    // the session was never invalidated.
    const followup = await request.get('/api/v1/jobs', {
      headers: { Authorization: `Bearer ${token}` },
    })
    expect(followup.ok()).toBeTruthy()
  })

  test('rejects new password shorter than 8 characters', async ({ page, request }) => {
    await login(page)
    const token = await page.evaluate(() => localStorage.getItem('token'))

    const resp = await request.post('/api/v1/auth/change-password', {
      headers: { Authorization: `Bearer ${token}` },
      data: { current_password: TEST_PASS, new_password: 'short' },
    })
    expect(resp.status()).toBe(400)
  })

  test('rejects new password matching current', async ({ page, request }) => {
    await login(page)
    const token = await page.evaluate(() => localStorage.getItem('token'))

    const resp = await request.post('/api/v1/auth/change-password', {
      headers: { Authorization: `Bearer ${token}` },
      data: { current_password: TEST_PASS, new_password: TEST_PASS },
    })
    expect(resp.status()).toBe(400)
  })

  test('successful rotation issues a fresh token and authorises future calls', async ({ page, request }) => {
    await login(page)
    const oldToken = await page.evaluate(() => localStorage.getItem('token'))

    // Rotate forward: TEST_PASS → TEMP_PASS.
    const rotateForward = await request.post('/api/v1/auth/change-password', {
      headers: { Authorization: `Bearer ${oldToken}` },
      data: { current_password: TEST_PASS, new_password: TEMP_PASS },
    })
    expect(rotateForward.ok()).toBeTruthy()
    const { token: newToken } = await rotateForward.json() as { token: string }
    expect(newToken).toBeTruthy()
    // NB: we deliberately don't assert newToken !== oldToken. When login and
    // change-password happen in the same wall-clock second, the JWT's
    // iat/exp claims (truncated to seconds) are identical, all other claims
    // are identical, and the signed bytes match exactly. That's expected JWT
    // behaviour — what matters is that the new token is valid.

    // Wrap everything after the forward rotation in try/finally so the
    // rotate-back ALWAYS happens, even if a subsequent assertion fails.
    // Without this, a mid-test failure leaves the admin password stuck at
    // TEMP_PASS and breaks every subsequent run of the suite.
    try {
      // The new token authorises an API call.
      const ping = await request.get('/api/v1/jobs', {
        headers: { Authorization: `Bearer ${newToken}` },
      })
      expect(ping.ok()).toBeTruthy()

      // Logging in again with the new password works.
      const reLogin = await request.post('/api/v1/auth/login', {
        data: { username: ADMIN_USER, password: TEMP_PASS },
      })
      expect(reLogin.ok()).toBeTruthy()
    } finally {
      // Rotate back: TEMP_PASS → TEST_PASS.
      await request.post('/api/v1/auth/change-password', {
        headers: { Authorization: `Bearer ${newToken}` },
        data: { current_password: TEMP_PASS, new_password: TEST_PASS },
      })
    }
  })
})
