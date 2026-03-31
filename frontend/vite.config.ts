import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api': 'https://localhost:8443',
      '/health': 'https://localhost:8443',
      '/ws': { target: 'wss://localhost:8443', ws: true },
    },
  },
})
