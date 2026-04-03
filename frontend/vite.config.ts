import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api':    { target: 'https://localhost:8443', secure: false },
      '/health': { target: 'https://localhost:8443', secure: false },
      '/ws':     { target: 'wss://localhost:8443',  ws: true, secure: false },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
  },
})
