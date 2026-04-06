import { useState } from 'react'
import { Plus, Play, Square, Pencil, Trash2, RefreshCw, Search } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listJobs, runJob, pauseJob, resumeJob, deleteJob } from '../api/client'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Modal } from '../components/Modal'
import { useToast } from '../components/Toast'
import type { Job } from '../api/types'
import type { View } from '../components/Sidebar'

interface Props {
  onNav: (v: View, jobId?: number) => void
}

type Filter = 'all' | 'running' | 'error' | 'disabled'

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

export function JobsView({ onNav }: Props) {
  const qc = useQueryClient()
  const toast = useToast()
  const [filter, setFilter] = useState<Filter>('all')
  const [search, setSearch] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<Job | null>(null)

  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs, refetchInterval: 5000 })

  const run    = useMutation({ mutationFn: runJob,    onSuccess: () => { qc.invalidateQueries({queryKey:['jobs']}); toast('Job queued.','ok') } })
  const pause  = useMutation({ mutationFn: pauseJob,  onSuccess: () => { qc.invalidateQueries({queryKey:['jobs']}); toast('Job stopped.','info') } })
  const resume = useMutation({ mutationFn: resumeJob, onSuccess: () => { qc.invalidateQueries({queryKey:['jobs']}); toast('Job restarted.','ok') } })
  const del    = useMutation({ mutationFn: deleteJob, onSuccess: () => { qc.invalidateQueries({queryKey:['jobs']}); toast('Job deleted.','ok'); setDeleteTarget(null) } })

  const visible = jobs
    .filter(j => {
      if (filter === 'running')  return j.status === 'running'
      if (filter === 'error')    return j.status === 'error'
      if (filter === 'disabled') return j.status === 'disabled'
      return true
    })
    .filter(j => !search || j.name.toLowerCase().includes(search.toLowerCase()))

  const triggerLabel = (j: Job) => {
    if (j.watch_enabled && j.cron_schedule) return `FS watch & ${j.cron_schedule}`
    if (j.watch_enabled) return 'FS watch'
    if (j.cron_schedule) return j.cron_schedule
    return 'Manual'
  }

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Sync Jobs</div>
        <div className="mla flex gap8">
          <Button variant="primary" onClick={() => onNav('new-job')}>
            <Plus size={14}/> New Job
          </Button>
        </div>
      </div>

      <div className="flex gap8 mb16">
        <div style={{ position: 'relative', flex: 1, maxWidth: 300 }}>
          <Search size={13} style={{ position: 'absolute', left: 10, top: '50%', transform: 'translateY(-50%)', color: 'var(--text3)' }}/>
          <input className="fi" style={{ paddingLeft: 30, fontSize: 13 }} placeholder="Search jobs…" value={search} onChange={e => setSearch(e.target.value)}/>
        </div>
        <div className="pills" style={{ marginBottom: 0 }}>
          {(['all','running','error','disabled'] as Filter[]).map(f => (
            <button key={f} className={`pill${filter===f?' on':''}`} onClick={() => setFilter(f)}>
              {f.charAt(0).toUpperCase()+f.slice(1)}
            </button>
          ))}
        </div>
      </div>

      <div className="card p0">
        <div className="tbl-wrap">
          <table>
            <thead><tr>
              <th>Name</th><th>Paths</th><th>Mode</th><th>Trigger</th>
              <th>Status</th><th>Last Run</th><th></th>
            </tr></thead>
            <tbody>
              {visible.map(j => (
                <tr key={j.id} onClick={() => onNav('job-detail', j.id)}>
                  <td>
                    <div className="fw5">{j.name}</div>
                    {j.last_error && <div className="fs11 text3" style={{ color: 'var(--coral-light)' }}>⚠ {j.last_error}</div>}
                    {j.conflict_strategy !== 'ask-user' && !j.last_error && (
                      <div className="fs11 text3">Conflict: {j.conflict_strategy}</div>
                    )}
                  </td>
                  <td>
                    <div className="path-cell">
                      <div className="path-row"><span className="plabel">SRC</span>{j.source_path}</div>
                      <div className="path-row"><span className="plabel">DST</span>{j.destination_path}</div>
                    </div>
                  </td>
                  <td>{modePill(j.mode)}</td>
                  <td className="td-muted fs12">{triggerLabel(j)}</td>
                  <td>{statusBadge(j.status)}</td>
                  <td className="td-muted">{fmtDate(j.last_run_at)}</td>
                  <td>
                    <div className="row-acts" onClick={e => e.stopPropagation()}>
                      {j.status === 'running' && (
                        <button className="icon-btn" title="Stop" onClick={() => pause.mutate(j.id)}><Square size={16}/></button>
                      )}
                      {j.status === 'paused' && (
                        <button className="icon-btn" title="Restart" onClick={() => resume.mutate(j.id)}><Play size={16}/></button>
                      )}
                      {(j.status === 'idle' || j.status === 'error') && (
                        <button className="icon-btn" title="Run now" onClick={() => run.mutate(j.id)}><Play size={16}/></button>
                      )}
                      <button className="icon-btn" title="Edit" onClick={() => onNav('job-detail', j.id)}><Pencil size={16}/></button>
                      <button className="icon-btn" title="Delete" style={{ color: 'var(--coral-light)' }} onClick={() => setDeleteTarget(j)}><Trash2 size={16}/></button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {visible.length === 0 && (
            <div className="empty">
              <RefreshCw size={44}/>
              <div className="empty-title">{search ? 'No matching jobs' : 'No jobs yet'}</div>
              <div className="empty-desc">{search ? 'Try a different search term.' : 'Create your first sync job to get started.'}</div>
              {!search && <Button variant="primary" onClick={() => onNav('new-job')}>New Job</Button>}
            </div>
          )}
        </div>
      </div>

      <Modal
        open={!!deleteTarget}
        title="Delete job?"
        body={`This will permanently delete "${deleteTarget?.name}". Sync history and manifest will be removed. This cannot be undone.`}
        confirmLabel="Delete"
        confirmVariant="danger"
        onConfirm={() => deleteTarget && del.mutate(deleteTarget.id)}
        onClose={() => setDeleteTarget(null)}
      />
    </div>
  )
}
