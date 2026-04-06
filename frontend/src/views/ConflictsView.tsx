import { GitMerge } from 'lucide-react'
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query'
import { listConflicts, listJobs, clearResolvedConflicts } from '../api/client'
import { Card } from '../components/Card'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { ConflictCard } from '../components/ConflictCard'
import { StatusBadge, ModePill } from '../components/JobFormatters'
import { useToast } from '../components/Toast'
import type { Conflict } from '../api/types'
import type { View } from '../components/Sidebar'

interface Props { onNav: (v: View, id?: number) => void }

function fmtDate(d: string) {
  return new Date(d).toLocaleString()
}

export function ConflictsView({ onNav }: Props) {
  const qc = useQueryClient()
  const toast = useToast()

  const { data: conflicts = [] } = useQuery({
    queryKey: ['conflicts'],
    queryFn: () => listConflicts(),
    refetchInterval: 10000,
  })

  const { data: jobs = [] } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
    staleTime: 30000,
  })

  const jobMap = new Map(jobs.map(j => [j.id, j]))

  // Group by job, preserving order of first appearance.
  type Group = { jobId: number; pending: Conflict[]; resolved: Conflict[] }
  const groups: Group[] = []
  const seen = new Map<number, Group>()

  for (const c of conflicts) {
    if (!seen.has(c.job_id)) {
      const g: Group = { jobId: c.job_id, pending: [], resolved: [] }
      seen.set(c.job_id, g)
      groups.push(g)
    }
    const g = seen.get(c.job_id)!
    if (c.status === 'pending') g.pending.push(c)
    else g.resolved.push(c)
  }

  // Jobs with pending conflicts first.
  groups.sort((a, b) => b.pending.length - a.pending.length)

  const clearResolved = useMutation({
    mutationFn: (jobId: number | undefined) => clearResolvedConflicts(jobId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['conflicts'] })
      toast('Resolved conflicts cleared.', 'ok')
    },
    onError: () => toast('Failed to clear resolved conflicts.', 'err'),
  })

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Conflicts</div>
      </div>

      {groups.length === 0 && (
        <Card>
          <div className="empty">
            <GitMerge size={44}/>
            <div className="empty-title">No conflicts</div>
            <div className="empty-desc">
              All conflicts have been resolved. Two-way sync jobs will surface new ones here as they occur.
            </div>
          </div>
        </Card>
      )}

      {groups.map(({ jobId, pending, resolved }) => {
        const job = jobMap.get(jobId)
        const hasBoth = pending.length > 0 && resolved.length > 0
        return (
          <Card key={jobId} noPad style={{ marginBottom: 16 }}>

            {/* Job header */}
            <div style={{
              padding: '14px 16px',
              borderBottom: '1px solid var(--border)',
              display: 'flex',
              alignItems: 'center',
              gap: 8,
              flexWrap: 'wrap',
            }}>
              <span
                className="card-title"
                style={{ cursor: 'pointer', marginBottom: 0 }}
                onClick={() => onNav('job-detail', jobId)}
              >
                {job?.name ?? `Job #${jobId}`}
              </span>
              {job && <ModePill mode={job.mode} />}
              {job && <StatusBadge status={job.status} />}
              {pending.length > 0 && (
                <Badge variant="pending">{pending.length} pending</Badge>
              )}
            </div>

            {/* Pending conflicts */}
            {pending.length > 0 && (
              <div style={{
                padding: '12px 16px',
                borderBottom: hasBoth ? '1px solid var(--border)' : undefined,
              }}>
                <ConflictCard
                  conflicts={pending}
                  onChanged={() => qc.invalidateQueries({ queryKey: ['conflicts'] })}
                />
              </div>
            )}

            {/* Recently resolved */}
            {resolved.length > 0 && (
              <div style={{ padding: '12px 16px' }}>
                <div style={{ display: 'flex', alignItems: 'center', marginBottom: 10 }}>
                  <span className="fs11 text3" style={{ fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em', flex: 1 }}>
                    Recently Resolved
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => clearResolved.mutate(jobId)}
                    disabled={clearResolved.isPending}
                  >
                    Clear
                  </Button>
                </div>
                <div className="tbl-wrap">
                  <table>
                    <thead><tr>
                      <th>File</th><th>Resolution</th><th>Resolved at</th>
                    </tr></thead>
                    <tbody>
                      {resolved.slice(0, 10).map(c => (
                        <tr key={c.id}>
                          <td className="td-mono">{c.rel_path}</td>
                          <td><span className="badge b-synced">{c.resolution ?? c.status}</span></td>
                          <td className="td-muted">{c.resolved_at ? fmtDate(c.resolved_at) : '—'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>
            )}
          </Card>
        )
      })}
    </div>
  )
}
