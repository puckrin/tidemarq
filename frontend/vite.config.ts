import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api':    { target: 'https://localhost:8717', secure: false, changeOrigin: true },
      '/health': { target: 'https://localhost:8717', secure: false, changeOrigin: true },
      '/ws':     { target: 'wss://localhost:8717',   ws: true,      secure: false, changeOrigin: true },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    exclude: ['e2e/**', 'node_modules/**'],
  },
})
