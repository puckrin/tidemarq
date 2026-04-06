// AuditLogProvider is kept for API compatibility but the audit view now
// reads directly from the DB via REST. This file is intentionally minimal.
import { createContext, useContext, type ReactNode } from 'react'

interface AuditLogContext {
  // reserved for future use
}

const Ctx = createContext<AuditLogContext | null>(null)

interface Props {
  children: ReactNode
  jobNames?: Record<number, string>
}

export function AuditLogProvider({ children }: Props) {
  return <Ctx.Provider value={{}}>{children}</Ctx.Provider>
}

export function useAuditLog() {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useAuditLog must be used within AuditLogProvider')
  return ctx
}
