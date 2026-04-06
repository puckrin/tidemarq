import { useState } from 'react'
import { GitMerge } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listConflicts, resolveConflict, listJobs } from '../api/client'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { useToast } from '../components/Toast'
import type { Conflict } from '../api/types'

function fmtDate(d: string) {
  return new Date(d).toLocaleString()
}

function fmtBytes(b: number) {
  if (b < 1024) return `${b} B`
  if (b < 1048576) return `${(b/1024).toFixed(1)} KB`
  return `${(b/1048576).toFixed(1)} MB`
}

export function ConflictsView() {
  const qc = useQueryClient()
  const toast = useToast()
  const [selected, setSelected] = useState<Conflict | null>(null)
  const [filterJob, setFilterJob] = useState<number | undefined>()

  const { data: jobs = [] } = useQuery({ queryKey: ['jobs'], queryFn: listJobs })
  const { data: conflicts = [] } = useQuery({
    queryKey: ['conflicts', filterJob],
    queryFn: () => listConflicts(filterJob),
    refetchInterval: 10000,
  })

  const resolve = useMutation({
    mutationFn: ({ id, action }: { id: number; action: string }) => resolveConflict(id, action),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['conflicts'] })
      toast('Conflict resolved.', 'ok')
      setSelected(null)
    },
    onError: () => toast('Failed to resolve conflict.', 'err'),
  })

  const pending  = conflicts.filter(c => c.status === 'pending')
  const resolved = conflicts.filter(c => c.status !== 'pending')

  const jobName = (id: number) => jobs.find(j => j.id === id)?.name ?? `Job #${id}`

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <div className="page-hd">
        <div className="page-title">Conflicts</div>
        <div className="mla flex gap8">
          <select className="fs" style={{ width: 180, fontSize: 13 }} value={filterJob ?? ''} onChange={e => setFilterJob(e.target.value ? Number(e.target.value) : undefined)}>
            <option value="">All jobs</option>
            {jobs.map(j => <option key={j.id} value={j.id}>{j.name}</option>)}
          </select>
        </div>
      </div>

      <div style={{ flex: 1, display: 'flex', gap: 0, overflow: 'hidden', minHeight: 0 }}>
        {/* List column */}
        <div style={{ flex: 1, overflowY: 'auto', minWidth: 0 }}>
          {pending.length === 0 && (
            <div className="empty">
              <GitMerge size={44}/>
              <div className="empty-title">No pending conflicts</div>
              <div className="empty-desc">All conflicts have been resolved. Two-way sync jobs will surface new ones here as they occur.</div>
            </div>
          )}

          {pending.length > 0 && (
            <Card noPad style={{ marginBottom: 16 }}>
              <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)', display: 'flex', alignItems: 'center', gap: 8 }}>
                <div className="card-title" style={{ marginBottom: 0 }}>Pending</div>
                <Badge variant="pending">{pending.length}</Badge>
              </div>
              <div className="tbl-wrap">
                <table>
                  <thead><tr>
                    <th>File</th><th>Job</th><th>Detected</th><th>Src size</th><th>Dst size</th><th>Strategy</th>
                  </tr></thead>
                  <tbody>
                    {pending.map(c => (
                      <tr key={c.id} onClick={() => setSelected(c)} style={{ background: selected?.id===c.id ? 'var(--active)' : '' }}>
                        <td className="td-mono">{c.rel_path}</td>
                        <td className="td-muted">{jobName(c.job_id)}</td>
                        <td className="td-muted">{fmtDate(c.created_at)}</td>
                        <td className="td-muted">{fmtBytes(c.src_size)}</td>
                        <td className="td-muted">{fmtBytes(c.dest_size)}</td>
                        <td><span className="badge b-pending">{c.strategy}</span></td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
          )}

          {resolved.length > 0 && (
            <Card noPad>
              <div style={{ padding: '12px 16px', borderBottom: '1px solid var(--border)' }}>
                <div className="card-title" style={{ marginBottom: 0 }}>Recently Resolved</div>
              </div>
              <div className="tbl-wrap">
                <table>
                  <thead><tr>
                    <th>File</th><th>Job</th><th>Resolution</th><th>Resolved at</th>
                  </tr></thead>
                  <tbody>
                    {resolved.slice(0, 20).map(c => (
                      <tr key={c.id}>
                        <td className="td-mono">{c.rel_path}</td>
                        <td className="td-muted">{jobName(c.job_id)}</td>
                        <td><span className="badge b-synced">{c.resolution ?? c.status}</span></td>
                        <td className="td-muted">{c.resolved_at ? fmtDate(c.resolved_at) : '—'}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </Card>
          )}
        </div>

        {/* Detail panel */}
        {selected && (
          <div className="conflict-detail" style={{ display: 'block' }}>
            <div style={{ fontWeight: 600, fontSize: 14, marginBottom: 4 }}>Resolve Conflict</div>
            <div className="mono fs12 text2 mb16" style={{ wordBreak: 'break-all' }}>{selected.rel_path}</div>

            <div className="diff-cols">
              <div className="diff-side">
                <div className="diff-hd src">Source</div>
                <div className="diff-body">
                  <div className="diff-row"><strong>Size</strong>: {fmtBytes(selected.src_size)}</div>
                  <div className="diff-row"><strong>Modified</strong>: {fmtDate(selected.src_mod_time)}</div>
                  <div className="diff-row"><strong>SHA256</strong>: <span className="mono" style={{ fontSize: 10 }}>{selected.src_sha256.slice(0,16)}…</span></div>
                </div>
              </div>
              <div className="diff-side">
                <div className="diff-hd dst">Destination</div>
                <div className="diff-body">
                  <div className="diff-row"><strong>Size</strong>: {fmtBytes(selected.dest_size)}</div>
                  <div className="diff-row"><strong>Modified</strong>: {fmtDate(selected.dest_mod_time)}</div>
                  <div className="diff-row"><strong>SHA256</strong>: <span className="mono" style={{ fontSize: 10 }}>{selected.dest_sha256.slice(0,16)}…</span></div>
                </div>
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              <Button variant="primary" style={{ justifyContent: 'center' }} onClick={() => resolve.mutate({ id: selected.id, action: 'keep-source' })} disabled={resolve.isPending}>
                Keep source version
              </Button>
              <Button variant="secondary" style={{ justifyContent: 'center' }} onClick={() => resolve.mutate({ id: selected.id, action: 'keep-dest' })} disabled={resolve.isPending}>
                Keep destination version
              </Button>
              <Button variant="ghost" style={{ justifyContent: 'center' }} onClick={() => resolve.mutate({ id: selected.id, action: 'keep-both' })} disabled={resolve.isPending}>
                Keep both (rename conflict copy)
              </Button>
              <div className="divider"/>
              <Button variant="ghost" onClick={() => setSelected(null)}>Dismiss</Button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
