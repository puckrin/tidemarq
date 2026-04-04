import { useState } from 'react'
import { ScrollText, Trash2 } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { listJobs } from '../api/client'
import { useAuditLog, type LogEntry } from '../store/auditLog'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import type { View } from '../components/Sidebar'

interface Props { onNav: (v: View, id?: number) => void }

function fmtTime(d: Date) {
  return d.toLocaleString(undefined, {
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function typeCls(t: LogEntry['type']) {
  if (t === 'error') return 'lt-error'
  if (t === 'info')  return 'lt-system'
  return 'lt-sync'
}

function typeLabel(t: LogEntry['type']) {
  if (t === 'error') return 'error'
  if (t === 'info')  return 'system'
  return 'sync'
}

// Seed historical entries from job.last_run_at so the log isn't empty on first load
function useHistoricalEntries(filterJobId: number | undefined) {
  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs })
  return jobs
    .filter(j => j.last_run_at && (!filterJobId || j.id === filterJobId))
    .sort((a, b) => new Date(b.last_run_at!).getTime() - new Date(a.last_run_at!).getTime())
    .map((j, i): LogEntry => ({
      id:      -(i + 1),   // negative ids = historical, won't clash with ws entries
      ts:      new Date(j.last_run_at!),
      jobId:   j.id,
      jobName: j.name,
      type:    j.status === 'error' ? 'error' : 'sync',
      message: j.status === 'error' ? 'Job error' : 'Job completed',
      detail:  j.last_error ?? undefined,
    }))
}

export function AuditView({ onNav }: Props) {
  const { entries, clear } = useAuditLog()
  const [filterJobId, setFilterJobId] = useState<number | undefined>()
  const [filterType,  setFilterType]  = useState<'all' | 'sync' | 'error' | 'info'>('all')

  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs })
  const historical = useHistoricalEntries(filterJobId)

  // Merge live WS entries with historical, deduplicate by id, sort newest first
  const wsFiltered = entries.filter(e =>
    (!filterJobId || e.jobId === filterJobId) &&
    (filterType === 'all' || e.type === filterType)
  )

  // Historical entries only shown when no live entries exist for that job+type combo
  // to avoid duplication once a job has run during this session
  const liveJobIds = new Set(entries.map(e => e.jobId))
  const historicalFiltered = historical.filter(e =>
    !liveJobIds.has(e.jobId) &&
    (filterType === 'all' || e.type === filterType)
  )

  const allEntries = [...wsFiltered, ...historicalFiltered]
    .sort((a, b) => b.ts.getTime() - a.ts.getTime())

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Audit Log</div>
        <div className="mla flex gap8">
          {entries.length > 0 && (
            <Button variant="ghost" size="sm" onClick={clear}>
              <Trash2 size={13}/> Clear session log
            </Button>
          )}
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
              <div
                className="log-msg"
                style={{ cursor: 'pointer' }}
                onClick={() => onNav('job-detail', e.jobId)}
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
            {entries.length > 0 ? ' · live session log' : ' · from last known job state'}
          </div>
        )}
      </Card>
    </div>
  )
}
