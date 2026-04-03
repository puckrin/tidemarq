import { useCallback, useState } from 'react'
import { Pause, Play, Trash2, Pencil } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getJob, runJob, pauseJob, resumeJob, deleteJob } from '../api/client'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Card, CardHeader } from '../components/Card'
import { ProgressBar } from '../components/ProgressBar'
import { Modal } from '../components/Modal'
import { useToast } from '../components/Toast'
import { useWsEvents } from '../hooks/useWsEvents'
import type { WsEvent, Job } from '../api/types'
import type { View } from '../components/Sidebar'

interface Props { jobId: number; onNav: (v: View, id?: number) => void }

function statusBadge(s: Job['status']) {
  const map: Record<Job['status'], 'running'|'synced'|'pending'|'error'|'disabled'> = {
    running:'running', idle:'synced', paused:'pending', error:'error', disabled:'disabled',
  }
  const labels: Record<Job['status'], string> = {
    running:'Running', idle:'Synced', paused:'Paused', error:'Error', disabled:'Disabled',
  }
  return <Badge variant={map[s]}>{labels[s]}</Badge>
}

function modePill(m: Job['mode']) {
  const labels: Record<Job['mode'], string> = {
    'one-way-backup':'One-way backup','one-way-mirror':'One-way mirror','two-way':'Two-way',
  }
  return <span className="mode-pill">{labels[m]}</span>
}

function fmtDate(d: string | null) {
  if (!d) return '—'
  return new Date(d).toLocaleString()
}

function fmtBytes(b: number) {
  if (b < 1024) return `${b} B`
  if (b < 1048576) return `${(b/1024).toFixed(1)} KB`
  if (b < 1073741824) return `${(b/1048576).toFixed(1)} MB`
  return `${(b/1073741824).toFixed(2)} GB`
}

export function JobDetailView({ jobId, onNav }: Props) {
  const qc = useQueryClient()
  const toast = useToast()
  const [delModal, setDelModal] = useState(false)
  const [wsData, setWsData] = useState<WsEvent | null>(null)

  const { data: job } = useQuery({ queryKey: ['job', jobId], queryFn: () => getJob(jobId), refetchInterval: 5000 })

  const handler = useCallback((e: WsEvent) => {
    if (e.job_id === jobId) setWsData(e)
  }, [jobId])
  useWsEvents(handler)

  const run    = useMutation({ mutationFn: () => runJob(jobId),    onSuccess: () => { qc.invalidateQueries({queryKey:['job',jobId]}); toast('Job started.','ok') } })
  const pause  = useMutation({ mutationFn: () => pauseJob(jobId),  onSuccess: () => { qc.invalidateQueries({queryKey:['job',jobId]}); toast('Job paused.','info') } })
  const resume = useMutation({ mutationFn: () => resumeJob(jobId), onSuccess: () => { qc.invalidateQueries({queryKey:['job',jobId]}); toast('Job resumed.','ok') } })
  const del    = useMutation({ mutationFn: () => deleteJob(jobId), onSuccess: () => { qc.invalidateQueries({queryKey:['jobs']}); toast('Job deleted.','ok'); onNav('jobs') } })

  if (!job) return <div className="text3" style={{ padding: 24 }}>Loading…</div>

  const pct = wsData?.files_total ? Math.round((wsData.files_done ?? 0) / wsData.files_total * 100) : 0

  return (
    <div>
      {/* Breadcrumb */}
      <div className="bc">
        <a onClick={() => onNav('jobs')}>Sync Jobs</a>
        <span className="bc-sep">/</span>
        <span>{job.name}</span>
      </div>

      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16, marginBottom: 24 }}>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 22, fontWeight: 700, marginBottom: 6 }}>{job.name}</div>
          <div className="flex gap12 fs12 text2" style={{ flexWrap: 'wrap' }}>
            {statusBadge(job.status)}
            {modePill(job.mode)}
            {job.watch_enabled && <span>FS watch</span>}
            {job.cron_schedule && <span>{job.cron_schedule}</span>}
            {job.bandwidth_limit_kb > 0 && <span>BW: {(job.bandwidth_limit_kb/1024).toFixed(1)} MB/s</span>}
          </div>
        </div>
        <div className="flex gap8">
          {job.status === 'running' && (
            <Button variant="secondary" onClick={() => pause.mutate()}><Pause size={14}/> Pause</Button>
          )}
          {job.status === 'paused' && (
            <Button variant="secondary" onClick={() => resume.mutate()}><Play size={14}/> Resume</Button>
          )}
          {(job.status === 'idle' || job.status === 'error') && (
            <Button variant="secondary" onClick={() => run.mutate()}><Play size={14}/> Run now</Button>
          )}
          <Button variant="ghost" onClick={() => setDelModal(true)}><Trash2 size={14}/></Button>
        </div>
      </div>

      {/* Live progress panel */}
      {job.status === 'running' && (
        <div className="run-panel">
          <div className="run-hd">
            <div className="flex gap8">
              <Badge variant="running">Running</Badge>
            </div>
            <Button variant="ghost" size="sm" onClick={() => pause.mutate()}>Pause</Button>
          </div>
          <ProgressBar pct={pct} height={8} />
          <div className="run-stats">
            <div>
              <div className="run-stat-label">Files Done</div>
              <div className="run-stat-val">{wsData?.files_done ?? 0} / {wsData?.files_total ?? '?'}</div>
            </div>
            <div>
              <div className="run-stat-label">Transfer Rate</div>
              <div className="run-stat-val">
                {wsData?.rate_kbs ? `${(wsData.rate_kbs/1024).toFixed(1)} MB/s` : '—'}
              </div>
            </div>
            <div>
              <div className="run-stat-label">Data Moved</div>
              <div className="run-stat-val">
                {wsData?.bytes_done ? fmtBytes(wsData.bytes_done) : '—'}
              </div>
            </div>
            <div>
              <div className="run-stat-label">Remaining</div>
              <div className="run-stat-val">
                {wsData?.eta_secs != null ? `~${Math.ceil(wsData.eta_secs/60)} min` : '—'}
              </div>
            </div>
          </div>
        </div>
      )}

      <div className="grid2 sec-gap">
        {/* Configuration */}
        <Card>
          <CardHeader title="Configuration" action={<Button variant="ghost" size="sm"><Pencil size={12}/> Edit</Button>}/>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13 }}>
            {[
              ['Source', <span className="mono fs12">{job.source_path}</span>],
              ['Destination', <span className="mono fs12">{job.destination_path}</span>],
              ['Mode', modePill(job.mode)],
              ['Trigger', job.watch_enabled && job.cron_schedule
                ? `FS watch & ${job.cron_schedule}`
                : job.watch_enabled ? 'FS watch' : job.cron_schedule || 'Manual'],
              ['Conflict strategy', job.conflict_strategy.replace(/-/g,' ')],
              ['Bandwidth limit', job.bandwidth_limit_kb > 0 ? `${(job.bandwidth_limit_kb/1024).toFixed(1)} MB/s` : 'None'],
            ].map(([label, val]) => (
              <div key={String(label)} className="flex gap8">
                <span className="text3 fw5" style={{ minWidth: 130 }}>{label}</span>
                <span>{val}</span>
              </div>
            ))}
            {job.last_error && (
              <div className="flex gap8">
                <span className="text3 fw5" style={{ minWidth: 130 }}>Last error</span>
                <span style={{ color: 'var(--coral-light)' }}>{job.last_error}</span>
              </div>
            )}
          </div>
        </Card>

        {/* Run history (static, API doesn't expose this yet) */}
        <Card>
          <CardHeader title="Details"/>
          <div style={{ display: 'flex', flexDirection: 'column', gap: 10, fontSize: 13 }}>
            {[
              ['Created',     fmtDate(job.created_at)],
              ['Last run',    fmtDate(job.last_run_at)],
              ['Status',      job.status],
            ].map(([label, val]) => (
              <div key={label} className="flex gap8">
                <span className="text3 fw5" style={{ minWidth: 130 }}>{label}</span>
                <span>{val}</span>
              </div>
            ))}
          </div>
        </Card>
      </div>

      <Modal
        open={delModal}
        title="Delete job?"
        body={`This will permanently delete "${job.name}". This cannot be undone.`}
        confirmLabel="Delete"
        confirmVariant="danger"
        onConfirm={() => del.mutate()}
        onClose={() => setDelModal(false)}
      />
    </div>
  )
}
