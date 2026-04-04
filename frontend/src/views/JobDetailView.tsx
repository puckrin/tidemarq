import { useState } from 'react'
import { Square, Play, Trash2, Pencil, FileCheck, FileCog, FileX } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getJob, runJob, pauseJob, resumeJob, deleteJob } from '../api/client'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Card, CardHeader } from '../components/Card'
import { ProgressBar } from '../components/ProgressBar'
import { Modal } from '../components/Modal'
import { useToast } from '../components/Toast'
import { useJobProgress } from '../store/jobProgress'
import type { Job } from '../api/types'
import type { View } from '../components/Sidebar'

interface Props { jobId: number; onNav: (v: View, id?: number) => void }

function statusBadge(s: Job['status']) {
  const map: Record<Job['status'], 'running'|'synced'|'pending'|'error'|'disabled'> = {
    running:'running', idle:'synced', paused:'pending', error:'error', disabled:'disabled',
  }
  const labels: Record<Job['status'], string> = {
    running:'Running', idle:'Synced', paused:'Stopped', error:'Error', disabled:'Disabled',
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

function actionIcon(action: string) {
  if (action === 'copied') return <FileCheck size={12} style={{ color: 'var(--accent)', flexShrink: 0 }}/>
  if (action === 'removing') return <FileX size={12} style={{ color: 'var(--coral-light)', flexShrink: 0 }}/>
  return <FileCog size={12} style={{ color: 'var(--text3)', flexShrink: 0 }}/>
}

function actionLabel(action: string): string {
  const labels: Record<string, string> = {
    copied: 'Copied', skipped: 'Unchanged', removing: 'Removed',
  }
  return labels[action] ?? action
}

export function JobDetailView({ jobId, onNav }: Props) {
  const qc = useQueryClient()
  const toast = useToast()
  const [delModal, setDelModal] = useState(false)

  const { data: job } = useQuery({ queryKey: ['job', jobId], queryFn: () => getJob(jobId), refetchInterval: 5000 })

  // Global progress store — survives navigation
  const progress = useJobProgress(jobId)

  const run    = useMutation({ mutationFn: () => runJob(jobId),    onSuccess: () => { qc.invalidateQueries({queryKey:['job',jobId]}); toast('Job started.','ok') } })
  const pause  = useMutation({ mutationFn: () => pauseJob(jobId),  onSuccess: () => { qc.invalidateQueries({queryKey:['job',jobId]}); toast('Job stopped.','info') } })
  const resume = useMutation({ mutationFn: () => resumeJob(jobId), onSuccess: () => { qc.invalidateQueries({queryKey:['job',jobId]}); toast('Job restarted.','ok') } })
  const del    = useMutation({ mutationFn: () => deleteJob(jobId), onSuccess: () => { qc.invalidateQueries({queryKey:['jobs']}); toast('Job deleted.','ok'); onNav('jobs') } })

  if (!job) return <div className="text3" style={{ padding: 24 }}>Loading…</div>

  const isRunning = job.status === 'running'
  const pct = progress.filesTotal > 0 ? Math.round(progress.filesDone / progress.filesTotal * 100) : 0

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
            <Button variant="secondary" onClick={() => pause.mutate()}><Square size={14}/> Stop</Button>
          )}
          {job.status === 'paused' && (
            <Button variant="secondary" onClick={() => resume.mutate()}><Play size={14}/> Restart</Button>
          )}
          {(job.status === 'idle' || job.status === 'error') && (
            <Button variant="secondary" onClick={() => run.mutate()}><Play size={14}/> Run now</Button>
          )}
          <Button variant="secondary" onClick={() => onNav('edit-job', jobId)}><Pencil size={14}/> Edit</Button>
          <Button variant="ghost" onClick={() => setDelModal(true)}><Trash2 size={14}/></Button>
        </div>
      </div>

      {/* Live progress panel — shown while running, or while progress data exists from this session */}
      {(isRunning || progress.lastEvent !== '') && (
        <div className="run-panel" style={{ marginBottom: 20 }}>
          <div className="run-hd">
            <div className="flex gap8">
              {isRunning
                ? <Badge variant="running">Running</Badge>
                : <Badge variant={progress.lastEvent === 'completed' ? 'synced' : progress.lastEvent === 'paused' ? 'pending' : 'error'}>
                    {progress.lastEvent === 'completed' ? 'Completed' : progress.lastEvent === 'paused' ? 'Stopped' : progress.lastEvent === 'error' ? 'Error' : 'Running'}
                  </Badge>
              }
            </div>
            {isRunning && (
              <Button variant="ghost" size="sm" onClick={() => pause.mutate()}>Stop</Button>
            )}
          </div>

          <ProgressBar pct={pct} height={8} />

          {/* Current file indicator — shows during scanning (evaluating), copying (bytes moving),
              and removing (quarantine/delete). Stays visible between files. */}
          {(progress.currentAction === 'scanning' || progress.currentAction === 'copying' || progress.currentAction === 'removing') && progress.currentFile && (
            <div style={{
              marginTop: 8,
              fontSize: 12,
              color: 'var(--text2)',
              display: 'flex',
              alignItems: 'center',
              gap: 6,
              overflow: 'hidden',
            }}>
              <span style={{ color: progress.currentAction === 'removing' ? 'var(--coral-light)' : 'var(--text3)', flexShrink: 0 }}>
                {progress.currentAction === 'copying' ? 'Copying:' : progress.currentAction === 'removing' ? 'Removing:' : 'Scanning:'}
              </span>
              <span className="mono" style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                {progress.currentFile}
              </span>
            </div>
          )}

          {/* Stats row */}
          <div className="run-stats">
            <div>
              <div className="run-stat-label">Files Done</div>
              <div className="run-stat-val">
                {progress.filesDone} / {progress.filesTotal > 0 ? progress.filesTotal : '?'}
              </div>
            </div>
            <div>
              <div className="run-stat-label">Unchanged</div>
              <div className="run-stat-val">{progress.filesSkipped}</div>
            </div>
            <div>
              <div className="run-stat-label">Transfer Rate</div>
              <div className="run-stat-val">
                {progress.rateKBs > 0 ? `${(progress.rateKBs/1024).toFixed(1)} MB/s` : '—'}
              </div>
            </div>
            <div>
              <div className="run-stat-label">Data Moved</div>
              <div className="run-stat-val">
                {progress.bytesDone > 0 ? fmtBytes(progress.bytesDone) : '—'}
              </div>
            </div>
            <div>
              <div className="run-stat-label">Remaining</div>
              <div className="run-stat-val">
                {progress.etaSecs > 0 ? `~${Math.ceil(progress.etaSecs/60)} min` : '—'}
              </div>
            </div>
          </div>

          {/* Recent file activity */}
          {progress.recentFiles.length > 0 && (
            <div style={{ marginTop: 12 }}>
              <div style={{ fontSize: 11, color: 'var(--text3)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em', marginBottom: 4 }}>
                Recent activity
              </div>
              <div style={{
                maxHeight: 160,
                overflowY: 'auto',
                border: '1px solid var(--border)',
                borderRadius: 'var(--radius)',
                background: 'var(--input-bg)',
              }}>
                {progress.recentFiles.map((f, i) => (
                  <div key={i} style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: 8,
                    padding: '4px 10px',
                    borderBottom: i < progress.recentFiles.length - 1 ? '1px solid var(--border)' : undefined,
                    fontSize: 12,
                  }}>
                    {actionIcon(f.action)}
                    <span className="mono" style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--text2)' }}>
                      {f.relPath}
                    </span>
                    <span style={{ color: 'var(--text3)', flexShrink: 0, fontSize: 11 }}>
                      {actionLabel(f.action)}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      <div className="grid2 sec-gap">
        {/* Configuration */}
        <Card>
          <CardHeader title="Configuration" action={<Button variant="ghost" size="sm" onClick={() => onNav('edit-job', jobId)}><Pencil size={12}/> Edit</Button>}/>
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
              ['Verification', job.full_checksum ? 'Full SHA-256 (all files)' : 'Metadata fast-path'],
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

        {/* Details */}
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
