import { RefreshCw, CheckCircle2, AlertCircle, AlertTriangle, X, GitMerge, Plus, Archive } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { listJobs, listConflicts, listQuarantine } from '../api/client'
import { StatCard } from '../components/StatCard'
import { Card, CardHeader } from '../components/Card'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { ProgressBar } from '../components/ProgressBar'
import type { Job } from '../api/types'
import type { View } from '../components/Sidebar'
import { useWsEvents } from '../hooks/useWsEvents'
import { useCallback, useState, useEffect } from 'react'
import type { WsEvent } from '../api/types'
import { useQueryClient } from '@tanstack/react-query'

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

function timeLeft(expiresAt: string) {
  const diff = new Date(expiresAt).getTime() - Date.now()
  const days = Math.ceil(diff / 86400000)
  if (days <= 0) return 'Expired'
  if (days === 1) return '1 day left'
  return `${days} days left`
}

export function DashboardView({ onNav }: Props) {
  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs, refetchInterval: 10000 })
  const { data: conflicts = [] } = useQuery({ queryKey: ['conflicts'], queryFn: () => listConflicts(), refetchInterval: 15000 })
  const { data: quarantine = [] } = useQuery({ queryKey: ['quarantine'], queryFn: () => listQuarantine(), refetchInterval: 30000 })

  const qc = useQueryClient()

  // Refresh all dashboard data on mount so previously-run jobs are reflected
  // immediately, regardless of cache age or whether any WS events fired.
  useEffect(() => {
    qc.invalidateQueries({ queryKey: ['jobs'] })
    qc.invalidateQueries({ queryKey: ['conflicts'] })
    qc.invalidateQueries({ queryKey: ['quarantine'] })
  }, [qc])

  const [progress, setProgress] = useState<Record<number, WsEvent>>({})
  const handler = useCallback((e: WsEvent) => {
    setProgress(p => ({ ...p, [e.job_id]: e }))
    // When a job finishes, immediately refetch anything that might have changed.
    if (e.event === 'completed' || e.event === 'error') {
      qc.invalidateQueries({ queryKey: ['conflicts'] })
      qc.invalidateQueries({ queryKey: ['quarantine'] })
      qc.invalidateQueries({ queryKey: ['jobs'] })
    }
  }, [qc])
  useWsEvents(handler)

  const running    = jobs.filter(j => j.status === 'running')
  const errorJobs  = jobs.filter(j => j.status === 'error')
  const healthy    = jobs.filter(j => j.status === 'idle').length
  const pending        = conflicts.filter(c => c.status === 'pending')
  const activeQuarantine = quarantine.filter(q => new Date(q.expires_at).getTime() > Date.now())
  const expiringSoon   = activeQuarantine.filter(q => {
    const diff = new Date(q.expires_at).getTime() - Date.now()
    return diff < 7 * 86400000
  })

  const needsAttention = pending.length + activeQuarantine.length
  const attentionSub = [
    pending.length          > 0 ? `${pending.length} conflict${pending.length === 1 ? '' : 's'}` : '',
    activeQuarantine.length > 0 ? `${activeQuarantine.length} quarantined${expiringSoon.length > 0 ? ` (${expiringSoon.length} expiring soon)` : ''}` : '',
  ].filter(Boolean).join(' · ') || 'All clear'

  // Jobs that have pending conflicts — for the detail card
  const conflictJobIds = new Set(pending.map(c => c.job_id))
  const jobMap = Object.fromEntries(jobs.map(j => [j.id, j.name]))

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Dashboard</div>
        <div className="mla">
          <Button variant="primary" onClick={() => onNav('new-job')}>
            <Plus size={14} /> New Job
          </Button>
        </div>
      </div>

      {/* ── Stat cards ── */}
      <div className="stat-grid">
        <StatCard icon={<RefreshCw size={18}/>}    color="teal"  label="Total Jobs"      value={jobs.length}      sub={`${jobs.filter(j => j.status !== 'disabled').length} enabled`} />
        <StatCard icon={<CheckCircle2 size={18}/>} color="grass" label="Healthy"         value={healthy}          sub="Last run succeeded" />
        <StatCard icon={<AlertCircle size={18}/>}  color="coral" label="Errors"          value={errorJobs.length} sub="Require attention"
          valueStyle={errorJobs.length > 0 ? { color: 'var(--coral-light)' } : undefined} />
        <StatCard icon={<AlertTriangle size={18}/>} color="amber" label="Pending Review" value={needsAttention}  sub={attentionSub}
          valueStyle={needsAttention > 0 ? { color: 'var(--amber-light)' } : undefined}
          onClick={needsAttention > 0 ? () => onNav(pending.length > 0 ? 'conflicts' : 'quarantine') : undefined}
          style={needsAttention > 0 ? { cursor: 'pointer' } : undefined} />
      </div>

      {/* ── Two-column cards ── */}
      <div className="grid2 sec-gap" style={{ alignItems: 'stretch' }}>

        {/* Currently running */}
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
                    {p ? `${p.files_done ?? 0} / ${p.files_total ?? '?'} files · ${p.rate_kbs ? (p.rate_kbs / 1024).toFixed(1) + ' MB/s' : '—'}` : 'Starting…'}
                  </div>
                </div>
              </div>
            )
          })}
          {running.length === 0 && (
            <div className="text3 fs12" style={{ padding: '8px 0' }}>No jobs currently running.</div>
          )}
        </Card>

        {/* Needs attention */}
        <Card>
          <CardHeader title="Pending Review" action={
            needsAttention > 0
              ? <span style={{ fontSize: 11, color: 'var(--amber-light)' }}>{needsAttention} item{needsAttention === 1 ? '' : 's'}</span>
              : undefined
          } />

          {needsAttention === 0 && (
            <div className="text3 fs12" style={{ padding: '8px 0' }}>Nothing pending review right now.</div>
          )}

          {/* Pending conflicts grouped by job */}
          {conflictJobIds.size > 0 && (
            <>
              {[...conflictJobIds].map(jobId => {
                const count = pending.filter(c => c.job_id === jobId).length
                return (
                  <div key={jobId} className="act-item" style={{ cursor: 'pointer' }} onClick={() => onNav('conflicts')}>
                    <div className="act-icon ai-warn"><GitMerge size={13}/></div>
                    <div className="act-body">
                      <div className="act-title fw5">{jobMap[jobId] ?? `Job ${jobId}`}</div>
                      <div className="act-detail">{count} pending conflict{count === 1 ? '' : 's'}</div>
                    </div>
                    <Button variant="ghost" size="sm" onClick={e => { e.stopPropagation(); onNav('conflicts') }}>Resolve</Button>
                  </div>
                )
              })}
            </>
          )}

          {/* Quarantine items grouped by job */}
          {activeQuarantine.length > 0 && (
            <>
              {[...new Set(activeQuarantine.map(q => q.job_id))].map(jobId => {
                const items = activeQuarantine.filter(q => q.job_id === jobId)
                const soonest = items.reduce((a, b) =>
                  new Date(a.expires_at) < new Date(b.expires_at) ? a : b
                )
                const hasSoonExpiry = expiringSoon.some(q => q.job_id === jobId)
                return (
                  <div key={jobId} className="act-item" style={{ cursor: 'pointer' }} onClick={() => onNav('quarantine')}>
                    <div className="act-icon ai-warn"><Archive size={13}/></div>
                    <div className="act-body">
                      <div className="act-title fw5">{jobMap[jobId] ?? `Job ${jobId}`}</div>
                      <div className="act-detail">
                        {items.length} file{items.length === 1 ? '' : 's'} quarantined
                        {hasSoonExpiry ? ` · ${timeLeft(soonest.expires_at)}` : ''}
                      </div>
                    </div>
                    <Button variant="ghost" size="sm" onClick={e => { e.stopPropagation(); onNav('quarantine') }}>Review</Button>
                  </div>
                )
              })}
            </>
          )}

          {/* Error jobs */}
          {errorJobs.length > 0 && (
            <>
              {errorJobs.map(j => (
                <div key={j.id} className="act-item" style={{ cursor: 'pointer' }} onClick={() => onNav('job-detail', j.id)}>
                  <div className="act-icon ai-err"><X size={13}/></div>
                  <div className="act-body">
                    <div className="act-title fw5">{j.name}</div>
                    <div className="act-detail">{j.last_error ?? 'Job failed'}</div>
                  </div>
                  <Button variant="ghost" size="sm" onClick={e => { e.stopPropagation(); onNav('job-detail', j.id) }}>View</Button>
                </div>
              ))}
            </>
          )}
        </Card>
      </div>

      {/* ── Jobs table ── */}
      <Card noPad>
        <div style={{ padding: 16 }}>
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div className="card-title">All Jobs</div>
            <Button variant="ghost" size="sm" onClick={() => onNav('jobs')}>Manage jobs</Button>
          </div>
        </div>
        <div className="tbl-wrap">
          <table>
            <thead><tr>
              <th>Job Name</th><th>Mode</th><th>Status</th><th>Last Run</th><th>Trigger</th>
            </tr></thead>
            <tbody>
              {jobs.map(j => (
                <tr key={j.id} onClick={() => onNav('job-detail', j.id)}>
                  <td className="fw5">{j.name}</td>
                  <td>{modePill(j.mode)}</td>
                  <td>{statusBadge(j.status)}</td>
                  <td className="td-muted">{fmtDate(j.last_run_at)}</td>
                  <td className="td-muted">{j.cron_schedule || (j.watch_enabled ? 'On change' : 'Manual')}</td>
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
