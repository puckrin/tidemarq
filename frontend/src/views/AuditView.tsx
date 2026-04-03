import { ScrollText } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { listJobs } from '../api/client'
import { Card } from '../components/Card'
import { useState } from 'react'

// The audit log API is in Phase 6; for now we show an informative empty state
// with the full UI chrome ready to be wired up.

function fmtDate(d: string) {
  return new Date(d).toLocaleString()
}

export function AuditView() {
  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs })
  const [filterJob, setFilterJob] = useState<string>('')

  // Placeholder: derive a rudimentary activity list from job metadata
  const entries = jobs
    .filter(j => !filterJob || String(j.id) === filterJob)
    .filter(j => j.last_run_at)
    .sort((a, b) => new Date(b.last_run_at!).getTime() - new Date(a.last_run_at!).getTime())
    .map(j => ({
      time: j.last_run_at!,
      type: j.status === 'error' ? 'error' : 'sync',
      job: j.name,
      msg: j.status === 'error'
        ? `Job failed: ${j.last_error ?? 'unknown error'}`
        : `Job completed successfully`,
    }))

  const typeCls = (t: string) => t === 'error' ? 'lt-error' : t === 'user' ? 'lt-user' : t === 'system' ? 'lt-system' : 'lt-sync'

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Audit Log</div>
        <div className="mla flex gap8">
          <select className="fs" style={{ width: 180, fontSize: 13 }} value={filterJob} onChange={e => setFilterJob(e.target.value)}>
            <option value="">All jobs</option>
            {jobs.map(j => <option key={j.id} value={j.id}>{j.name}</option>)}
          </select>
        </div>
      </div>

      <Card>
        {entries.length === 0 && (
          <div className="empty">
            <ScrollText size={44}/>
            <div className="empty-title">No log entries</div>
            <div className="empty-desc">
              Audit log entries will appear here as jobs run.
              Full audit log search and export is available in Phase 6.
            </div>
          </div>
        )}
        {entries.map((e, i) => (
          <div key={i} className="log-entry">
            <div className="log-time">{fmtDate(e.time)}</div>
            <div className="log-body">
              <span className={`log-type ${typeCls(e.type)}`}>{e.type}</span>
              <div className="log-msg">{e.job}</div>
              <div className="log-detail">{e.msg}</div>
            </div>
          </div>
        ))}
      </Card>
    </div>
  )
}
