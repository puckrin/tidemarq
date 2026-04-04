import { createContext, useContext, useState, useCallback, useEffect, type ReactNode } from 'react'
import { wsClient } from '../api/ws'
import type { WsEvent } from '../api/types'

export interface LogEntry {
  id: number
  ts: Date
  jobId: number
  jobName: string
  type: 'sync' | 'error' | 'info'
  message: string
  detail?: string
}

interface AuditLogContext {
  entries: LogEntry[]
  clear: () => void
}

const Ctx = createContext<AuditLogContext | null>(null)

let nextId = 0

const EVENT_LABELS: Record<WsEvent['event'], string> = {
  started:   'Job started',
  progress:  'Job in progress',
  paused:    'Job stopped',
  completed: 'Job completed',
  error:     'Job error',
}

function wsEventToEntry(e: WsEvent, jobName: string): LogEntry {
  const type = e.event === 'error' ? 'error' : e.event === 'paused' ? 'info' : 'sync'
  let detail: string | undefined
  if (e.event === 'progress' && e.files_total) {
    detail = `${e.files_done ?? 0} / ${e.files_total} files · ${e.rate_kbs ? (e.rate_kbs / 1024).toFixed(1) + ' MB/s' : '—'}`
  }
  if (e.event === 'error' && e.message) detail = e.message
  if (e.event === 'completed' && e.files_done != null) detail = `${e.files_done} files processed`

  return {
    id:      ++nextId,
    ts:      new Date(),
    jobId:   e.job_id,
    jobName,
    type,
    message: EVENT_LABELS[e.event],
    detail,
  }
}

interface Props {
  children: ReactNode
  jobNames: Record<number, string>   // jobId → name, kept up to date by Shell
}

export function AuditLogProvider({ children, jobNames }: Props) {
  const [entries, setEntries] = useState<LogEntry[]>([])

  const add = useCallback((entry: LogEntry) => {
    setEntries(prev => {
      // Deduplicate rapid progress events — keep at most one 'progress' per job
      if (entry.message === 'Job in progress') {
        const filtered = prev.filter(e => !(e.jobId === entry.jobId && e.message === 'Job in progress'))
        return [entry, ...filtered].slice(0, 500)
      }
      return [entry, ...prev].slice(0, 500)
    })
  }, [])

  useEffect(() => {
    const unsub = wsClient.subscribe((e: WsEvent) => {
      const name = jobNames[e.job_id] ?? `Job #${e.job_id}`
      add(wsEventToEntry(e, name))
    })
    return () => { unsub() }
  }, [jobNames, add])

  const clear = useCallback(() => setEntries([]), [])

  return <Ctx.Provider value={{ entries, clear }}>{children}</Ctx.Provider>
}

export function useAuditLog() {
  const ctx = useContext(Ctx)
  if (!ctx) throw new Error('useAuditLog must be used within AuditLogProvider')
  return ctx
}
