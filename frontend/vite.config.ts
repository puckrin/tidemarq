import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      '/api':    { target: 'https://localhost:8443', secure: false, changeOrigin: true },
      '/health': { target: 'https://localhost:8443', secure: false, changeOrigin: true },
      '/ws':     { target: 'wss://localhost:8443',   ws: true,      secure: false, changeOrigin: true,
                   configure: (proxy) => {
                     // configure() runs before Vite registers its own proxyReqWs listener,
                     // so we defer with setImmediate to replace its socket error handler
                     // after it has been added — suppressing ECONNRESET noise from the
                     // backend closing idle WebSocket connections.
                     proxy.on('proxyReqWs', (_proxyReq, _req, socket: NodeJS.EventEmitter) => {
                       setImmediate(() => {
                         socket.removeAllListeners('error')
                         socket.on('error', (err: NodeJS.ErrnoException) => {
                           if (err.code !== 'ECONNRESET') console.error('[ws proxy]', err)
                         })
                       })
                     })
                   } },
    },
  },
  test: {
    environment: 'jsdom',
    globals: true,
    setupFiles: ['./src/test/setup.ts'],
    exclude: ['e2e/**', 'node_modules/**'],
  },
})
