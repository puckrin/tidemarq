import { createContext, useContext, useState, useEffect, useCallback, type ReactNode } from 'react'
import { login as apiLogin, changePassword as apiChangePassword } from '../api/client'
import { wsClient } from '../api/ws'

interface AuthUser {
  id: number
  username: string
  role: string
  // True when the backend has flagged the account as needing to change its
  // password before doing anything else (e.g. the seeded default admin).
  // The forced-change view in App.tsx renders while this is true; all API
  // calls except /auth/change-password will 403 with code
  // "password_change_required" until it is cleared.
  passwordChangeRequired: boolean
}

interface AuthContext {
  user: AuthUser | null
  token: string | null
  login: (username: string, password: string) => Promise<void>
  changePassword: (currentPassword: string, newPassword: string) => Promise<void>
  logout: () => void
}

const Ctx = createContext<AuthContext | null>(null)

function parseToken(token: string): AuthUser | null {
  try {
    const payload = JSON.parse(atob(token.split('.')[1] ?? '')) as {
      user_id: number
      username: string
      role: string
      pwd_change_required?: boolean
    }
    return {
      id: payload.user_id,
      username: payload.username,
      role: payload.role,
      passwordChangeRequired: !!payload.pwd_change_required,
    }
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
    // Don't open the WebSocket while the user is on the forced-change screen.
    // The ws-token endpoint is gated by the same middleware and will 403, so
    // connecting would just spin a retry loop.
    if (token && !user?.passwordChangeRequired) wsClient.connect()
    else wsClient.disconnect()
    // Return disconnect as cleanup so React StrictMode's double-invoke doesn't
    // leave a zombie WebSocket open alongside the real one. Both connections
    // would share the same listeners Set, causing every WS event to fire twice.
    return () => wsClient.disconnect()
  }, [token, user?.passwordChangeRequired])

  const login = async (username: string, password: string) => {
    const { token: t } = await apiLogin(username, password)
    localStorage.setItem('token', t)
    setToken(t)
    setUser(parseToken(t))
  }

  const changePassword = async (currentPassword: string, newPassword: string) => {
    const { token: t } = await apiChangePassword(currentPassword, newPassword)
    localStorage.setItem('token', t)
    setToken(t)
    setUser(parseToken(t))
  }

  return <Ctx.Provider value={{ user, token, login, changePassword, logout }}>{children}</Ctx.Provider>
}

export function useAuth() {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useAuth must be used within AuthProvider')
  return ctx
}
