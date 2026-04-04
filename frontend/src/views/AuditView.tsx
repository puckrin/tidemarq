import { useState } from 'react'
import { ScrollText, Trash2, Download } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { listJobs, listAuditLog } from '../api/client'
import { useAuditLog, type LogEntry } from '../store/auditLog'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import type { View } from '../components/Sidebar'
import type { AuditEntry } from '../api/types'

interface Props { onNav: (v: View, id?: number) => void }

function fmtTime(d: Date) {
  return d.toLocaleString(undefined, {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

// Map API event names to display types used by the log entry style
function eventType(event: string): 'sync' | 'error' | 'info' {
  if (event === 'job_failed') return 'error'
  if (event === 'conflict_detected') return 'info'
  return 'sync'
}

function typeCls(t: 'sync' | 'error' | 'info') {
  if (t === 'error') return 'lt-error'
  if (t === 'info')  return 'lt-system'
  return 'lt-sync'
}

function typeLabel(t: 'sync' | 'error' | 'info') {
  if (t === 'error') return 'error'
  if (t === 'info')  return 'system'
  return 'sync'
}

// A unified entry row for both live WS entries and DB-persisted entries
interface DisplayEntry {
  id: string       // "ws-{id}" or "db-{id}" — unique key
  ts: Date
  jobId?: number
  jobName: string
  type: 'sync' | 'error' | 'info'
  message: string
  detail?: string
  source: 'live' | 'db'
}

function wsToDisplay(e: LogEntry): DisplayEntry {
  return {
    id:      `ws-${e.id}`,
    ts:      e.ts,
    jobId:   e.jobId,
    jobName: e.jobName,
    type:    e.type,
    message: e.message,
    detail:  e.detail,
    source:  'live',
  }
}

function dbToDisplay(e: AuditEntry): DisplayEntry {
  return {
    id:      `db-${e.id}`,
    ts:      new Date(e.created_at),
    jobId:   e.job_id,
    jobName: e.job_name || `Job #${e.job_id}`,
    type:    eventType(e.event),
    message: e.message,
    detail:  e.detail || undefined,
    source:  'db',
  }
}

export function AuditView({ onNav }: Props) {
  const { entries: wsEntries, clear } = useAuditLog()
  const [filterJobId, setFilterJobId] = useState<number | undefined>()
  const [filterType,  setFilterType]  = useState<'all' | 'sync' | 'error' | 'info'>('all')

  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs })

  // DB-persisted entries — refetch every 15s while the view is open
  const { data: dbEntries = [] } = useQuery({
    queryKey: ['audit-log', filterJobId],
    queryFn: () => listAuditLog({ job_id: filterJobId, limit: 500 }),
    refetchInterval: 15000,
    staleTime: 5000,
  })

  // Convert both sources to a common shape
  const liveJobIds = new Set(wsEntries.map(e => e.jobId))

  const liveDisplay: DisplayEntry[] = wsEntries
    .filter(e =>
      (!filterJobId || e.jobId === filterJobId) &&
      (filterType === 'all' || e.type === filterType)
    )
    .map(wsToDisplay)

  // Show DB entries only for jobs not already represented in the live session log
  const dbDisplay: DisplayEntry[] = dbEntries
    .filter(e =>
      !liveJobIds.has(e.job_id ?? -1) &&
      (filterType === 'all' || eventType(e.event) === filterType)
    )
    .map(dbToDisplay)

  const allEntries = [...liveDisplay, ...dbDisplay]
    .sort((a, b) => b.ts.getTime() - a.ts.getTime())
    .slice(0, 500)

  const handleExport = (fmt: 'csv' | 'json') => {
    const qs = filterJobId ? `?job_id=${filterJobId}&format=${fmt}` : `?format=${fmt}`
    window.open(`/api/v1/audit/export${qs}`, '_blank')
  }

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Audit Log</div>
        <div className="mla flex gap8">
          {wsEntries.length > 0 && (
            <Button variant="ghost" size="sm" onClick={clear}>
              <Trash2 size={13}/> Clear session log
            </Button>
          )}
          <Button variant="ghost" size="sm" onClick={() => handleExport('csv')}>
            <Download size={13}/> CSV
          </Button>
          <Button variant="ghost" size="sm" onClick={() => handleExport('json')}>
            <Download size={13}/> JSON
          </Button>
        </div>
      </div>

      {/* Filters */}
      <div className="flex gap8 mb16" style={{ flexWrap: 'wrap' }}>
        <select
          className="fs"
          style={{ width: 200, fontSize: 13 }}
          value={filterJobId ?? ''}
          onChange={e => setFilterJobId(e.target.value ? Number(e.target.value) : undefined)}
        >
          <option value="">All jobs</option>
          {jobs.map(j => <option key={j.id} value={j.id}>{j.name}</option>)}
        </select>

        <div className="pills" style={{ marginBottom: 0 }}>
          {(['all', 'sync', 'error', 'info'] as const).map(f => (
            <button
              key={f}
              className={`pill${filterType === f ? ' on' : ''}`}
              onClick={() => setFilterType(f)}
            >
              {f.charAt(0).toUpperCase() + f.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <Card>
        {allEntries.length === 0 && (
          <div className="empty">
            <ScrollText size={44}/>
            <div className="empty-title">No log entries</div>
            <div className="empty-desc">
              Entries appear here as jobs run. Run a sync job to see live events.
            </div>
            <Button variant="primary" onClick={() => onNav('jobs')}>Go to Sync Jobs</Button>
          </div>
        )}

        {allEntries.map(e => (
          <div key={e.id} className="log-entry">
            <div className="log-time">{fmtTime(e.ts)}</div>
            <div className="log-body">
              <span className={`log-type ${typeCls(e.type)}`}>{typeLabel(e.type)}</span>
              {e.source === 'live' && (
                <span className="badge b-tag" style={{ marginLeft: 4, fontSize: 10 }}>live</span>
              )}
              <div
                className="log-msg"
                style={{ cursor: e.jobId ? 'pointer' : 'default' }}
                onClick={() => e.jobId && onNav('job-detail', e.jobId)}
              >
                {e.jobName} — {e.message}
              </div>
              {e.detail && <div className="log-detail">{e.detail}</div>}
            </div>
          </div>
        ))}

        {allEntries.length > 0 && (
          <div className="text3 fs12" style={{ paddingTop: 12, textAlign: 'center' }}>
            {allEntries.length} entr{allEntries.length === 1 ? 'y' : 'ies'}
            {wsEntries.length > 0 ? ' · includes live session events' : ' · from database'}
          </div>
        )}
      </Card>
    </div>
  )
}
