import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import { login as apiLogin } from '../api/client'
import { wsClient } from '../api/ws'

interface AuthUser {
  id: number
  username: string
  role: string
}

interface AuthContext {
  user: AuthUser | null
  token: string | null
  login: (username: string, password: string) => Promise<void>
  logout: () => void
}

const Ctx = createContext<AuthContext | null>(null)

function parseToken(token: string): AuthUser | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1] ?? '')) as { user_id: number; username: string; role: string }
    return { id: payload.user_id, username: payload.username, role: payload.role }
  } catch {
    return null
  }
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('token'))
  const [user, setUser] = useState<AuthUser | null>(() => {
    const t = localStorage.getItem('token')
    return t ? parseToken(t) : null
  })

  const logout = useCallback(() => {
    localStorage.removeItem('token')
    setToken(null)
    setUser(null)
    wsClient.disconnect()
  }, [])

  useEffect(() => {
    const handler = () => logout()
    window.addEventListener('auth:expired', handler)
    return () => window.removeEventListener('auth:expired', handler)
  }, [logout])

  useEffect(() => {
    if (token) wsClient.connect()
    else wsClient.disconnect()
    // Return disconnect as cleanup so React StrictMode's double-invoke doesn't
    // leave a zombie WebSocket open alongside the real one. Both connections
    // would share the same listeners Set, causing every WS event to fire twice.
    return () => wsClient.disconnect()
  }, [token])

  const login = async (username: string, password: string) => {
    const { token: t } = await apiLogin(username, password)
    localStorage.setItem('token', t)
    setToken(t)
    setUser(parseToken(t))
  }

  return <Ctx.Provider value={{ user, token, login, logout }}>{children}</Ctx.Provider>
}

export function useAuth() {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
