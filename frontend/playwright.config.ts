import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './e2e',
  globalSetup: './e2e/global-setup.ts',
  fullyParallel: false,
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 1,
  reporter: 'html',

  use: {
    // In dev, tests hit the Vite dev server which proxies /api and /ws to the backend.
    // In CI (or when TIDEMARQ_URL is set), tests hit the backend directly.
    baseURL: process.env.TIDEMARQ_URL ?? 'http://localhost:5173',
    ignoreHTTPSErrors: true,
    screenshot: 'only-on-failure',
    video: 'retain-on-failure',
  },

  // Start the Vite dev server automatically before running tests (dev only).
  // Requires the backend to already be running at https://localhost:8717.
  // Skip this block when TIDEMARQ_URL is set (CI / built frontend).
  webServer: process.env.TIDEMARQ_URL ? undefined : {
    command: 'npx vite',
    url: 'http://localhost:5173',
    reuseExistingServer: true,
    timeout: 60000,
    stdout: 'pipe',
    stderr: 'pipe',
  },

  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
  ],
})
