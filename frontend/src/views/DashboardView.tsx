import { RefreshCw, CheckCircle2, AlertCircle, HardDrive, Check, X, GitMerge, Square } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { listJobs, listConflicts } from '../api/client'
import { StatCard } from '../components/StatCard'
import { Card, CardHeader } from '../components/Card'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { ProgressBar } from '../components/ProgressBar'
import type { Job } from '../api/types'
import type { View } from '../components/Sidebar'
import { useWsEvents } from '../hooks/useWsEvents'
import { useCallback, useState } from 'react'
import type { WsEvent } from '../api/types'

interface Props { onNav: (v: View, jobId?: number) => void }

function statusBadge(s: Job['status']) {
  const map: Record<Job['status'], 'running' | 'synced' | 'pending' | 'error' | 'disabled'> = {
    running: 'running', idle: 'synced', paused: 'pending', error: 'error', disabled: 'disabled',
  }
  const labels: Record<Job['status'], string> = {
    running: 'Running', idle: 'Synced', paused: 'Stopped', error: 'Error', disabled: 'Disabled',
  }
  return <Badge variant={map[s]}>{labels[s]}</Badge>
}

function modePill(m: Job['mode']) {
  const labels: Record<Job['mode'], string> = {
    'one-way-backup': 'One-way backup',
    'one-way-mirror': 'One-way mirror',
    'two-way': 'Two-way',
  }
  return <span className="mode-pill">{labels[m]}</span>
}

function fmtDate(d: string | null) {
  if (!d) return '—'
  const dt = new Date(d)
  const diff = Date.now() - dt.getTime()
  const mins = Math.floor(diff / 60000)
  if (mins < 1) return 'Just now'
  if (mins < 60) return `${mins} min ago`
  const hrs = Math.floor(mins / 60)
  if (hrs < 24) return `${hrs} hr ago`
  return dt.toLocaleDateString()
}

export function DashboardView({ onNav }: Props) {
  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs, refetchInterval: 10000 })
  const { data: conflicts = [] } = useQuery({ queryKey: ['conflicts'], queryFn: () => listConflicts(), refetchInterval: 15000 })

  const [progress, setProgress] = useState<Record<number, WsEvent>>({})

  const handler = useCallback((e: WsEvent) => {
    setProgress(p => ({ ...p, [e.job_id]: e }))
  }, [])
  useWsEvents(handler)

  const total     = jobs.length
  const running   = jobs.filter(j => j.status === 'running')
  const errorJobs = jobs.filter(j => j.status === 'error')
  const healthy   = jobs.filter(j => j.status === 'idle').length
  const pending   = conflicts.filter(c => c.status === 'pending').length

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Dashboard</div>
        <div className="mla">
          <Button variant="primary" onClick={() => onNav('new-job')}>
            <RefreshCw size={14} /> New Job
          </Button>
        </div>
      </div>

      <div className="stat-grid">
        <StatCard icon={<RefreshCw size={18}/>} color="teal"  label="Total Jobs"      value={total}   sub={`${jobs.filter(j=>j.status!=='disabled').length} enabled`} />
        <StatCard icon={<CheckCircle2 size={18}/>} color="grass" label="Healthy"      value={healthy} sub="Last run succeeded" />
        <StatCard icon={<AlertCircle size={18}/>}  color="coral" label="Errors"       value={errorJobs.length} sub="Require attention" valueStyle={errorJobs.length > 0 ? { color: 'var(--coral-light)' } : undefined} />
        <StatCard icon={<HardDrive size={18}/>}   color="amber" label="Conflicts"     value={pending} sub="Pending resolution" />
      </div>

      <div className="grid2 sec-gap" style={{ alignItems: 'stretch' }}>
        {/* Recent activity — derived from last_run_at */}
        <Card>
          <CardHeader title="All Jobs — Recent" action={<Button variant="ghost" size="sm" onClick={() => onNav('jobs')}>View all</Button>} />
          {jobs.slice(0, 5).map(j => (
            <div key={j.id} className="act-item" style={{ cursor: 'pointer' }} onClick={() => onNav('job-detail', j.id)}>
              <div className={`act-icon ${j.status === 'error' ? 'ai-err' : j.status === 'paused' ? 'ai-info' : 'ai-ok'}`}>
                {j.status === 'error'  ? <X size={13}/> :
                 j.status === 'paused' ? <Square size={13}/> :
                 <Check size={13}/>}
              </div>
              <div className="act-body">
                <div className="act-title fw5">{j.name}</div>
                <div className="act-detail">{j.source_path} → {j.destination_path}</div>
              </div>
              <div className="act-time">{fmtDate(j.last_run_at)}</div>
            </div>
          ))}
          {jobs.length === 0 && (
            <div className="empty">
              <div className="empty-title">No jobs yet</div>
              <div className="empty-desc">Create your first sync job to get started.</div>
            </div>
          )}
        </Card>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
          {/* Running jobs */}
          <Card style={{ flex: 1 }}>
            <CardHeader title="Currently Running" action={<Badge variant="running">{running.length} active</Badge>} />
            {running.map(j => {
              const p = progress[j.id]
              const pct = p?.files_total ? Math.round((p.files_done ?? 0) / p.files_total * 100) : 0
              return (
                <div key={j.id} className="act-item" style={{ cursor: 'pointer' }} onClick={() => onNav('job-detail', j.id)}>
                  <div className="act-icon ai-info"><RefreshCw size={13}/></div>
                  <div className="act-body">
                    <div className="act-title fw5">{j.name}</div>
                    <div style={{ margin: '5px 0 3px' }}>
                      <ProgressBar pct={pct} />
                    </div>
                    <div className="act-detail">
                      {p ? `${p.files_done ?? 0} / ${p.files_total ?? '?'} files · ${p.rate_kbs ? (p.rate_kbs/1024).toFixed(1)+' MB/s' : '—'}` : 'Starting…'}
                    </div>
                  </div>
                </div>
              )
            })}
            {running.length === 0 && (
              <div className="text3 fs12" style={{ padding: '8px 0' }}>No jobs currently running.</div>
            )}
          </Card>

          {/* Conflicts summary */}
          <Card>
            <CardHeader title="Unresolved Conflicts" action={<Button variant="ghost" size="sm" onClick={() => onNav('conflicts')}>Resolve</Button>} />
            <div className="flex gap8" style={{ alignItems: 'flex-end' }}>
              <div style={{ fontSize: 32, fontWeight: 700, color: pending > 0 ? 'var(--amber-light)' : 'var(--text2)' }}>{pending}</div>
              <div className="fs12 text2 mb4">
                {pending === 0 ? 'All clear' : `across ${new Set(conflicts.filter(c=>c.status==='pending').map(c=>c.job_id)).size} job(s)`}
              </div>
            </div>
          </Card>
        </div>
      </div>

      {/* Jobs overview table */}
      <Card noPad>
        <div style={{ padding: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div className="card-title">All Jobs — Status Overview</div>
            <Button variant="ghost" size="sm" onClick={() => onNav('jobs')}>Manage jobs</Button>
          </div>
        </div>
        <div className="tbl-wrap">
          <table>
            <thead><tr>
              <th>Job Name</th><th>Mode</th><th>Status</th><th>Last Run</th><th>Next Run</th>
            </tr></thead>
            <tbody>
              {jobs.map(j => (
                <tr key={j.id} onClick={() => onNav('job-detail', j.id)}>
                  <td className="fw5">{j.name}</td>
                  <td>{modePill(j.mode)}</td>
                  <td>{statusBadge(j.status)}</td>
                  <td className="td-muted">{fmtDate(j.last_run_at)}</td>
                  <td className="td-muted">{j.cron_schedule || (j.watch_enabled ? 'On change' : '—')}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {jobs.length === 0 && (
            <div className="empty">
              <GitMerge size={44}/>
              <div className="empty-title">No sync jobs</div>
              <div className="empty-desc">Create a job to start syncing directories.</div>
              <Button variant="primary" onClick={() => onNav('new-job')}>New Job</Button>
            </div>
          )}
        </div>
      </Card>
    </div>
  )
}
