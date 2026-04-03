import type { WsEvent } from './types'
import { getWsToken } from './client'

type Listener = (event: WsEvent) => void

class WsClient {
  private ws: WebSocket | null = null
  private listeners = new Set<Listener>()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private stopped = false

  async connect() {
    this.stopped = false
    try {
      const { token } = await getWsToken()
      const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
      const host = window.location.host
      this.ws = new WebSocket(`${proto}://${host}/ws?token=${token}`)

      this.ws.onmessage = (e) => {
        try {
          const event: WsEvent = JSON.parse(e.data)
          this.listeners.forEach(fn => fn(event))
        } catch { /* ignore malformed messages */ }
      }

      this.ws.onclose = () => {
        if (!this.stopped) this.scheduleReconnect()
      }

      this.ws.onerror = () => {
        this.ws?.close()
      }
    } catch {
      if (!this.stopped) this.scheduleReconnect()
    }
  }

  private scheduleReconnect() {
    if (this.reconnectTimer) return
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null
      this.connect()
    }, 3000)
  }

  disconnect() {
    this.stopped = true
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.ws?.close()
    this.ws = null
  }

  subscribe(fn: Listener) {
    this.listeners.add(fn)
    return () => this.listeners.delete(fn)
  }
}

export const wsClient = new WsClient()
