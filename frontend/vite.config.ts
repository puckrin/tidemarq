import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import https from 'node:https'

const allowSelfSigned = new https.Agent({ rejectUnauthorized: false })

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api':    { target: 'https://localhost:8717', secure: false, changeOrigin: true },
      '/health': { target: 'https://localhost:8717', secure: false, changeOrigin: true },
      '/ws':     { target: 'wss://localhost:8443',  ws: true, secure: false, changeOrigin: true, agent: allowSelfSigned },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    exclude: ['e2e/**', 'node_modules/**'],
  },
})
