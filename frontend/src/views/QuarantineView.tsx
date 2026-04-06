import { Archive } from 'lucide-react'
import { useQuery, useQueryClient, useMutation } from '@tanstack/react-query'
import { listQuarantine, listRemovedQuarantine, listJobs, clearRemovedQuarantine } from '../api/client'
import { Card } from '../components/Card'
import { Badge } from '../components/Badge'
import { Button } from '../components/Button'
import { QuarantineCard } from '../components/QuarantineCard'
import { StatusBadge, ModePill } from '../components/JobFormatters'
import { useToast } from '../components/Toast'
import type { QuarantineEntry } from '../api/types'
import type { View } from '../components/Sidebar'

interface Props { onNav: (v: View, id?: number) => void }

function fmtDate(d: string) {
  return new Date(d).toLocaleString()
}

function fmtBytes(b: number) {
  if (b < 1024) return `${b} B`
  if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1073741824) return `${(b / 1048576).toFixed(1)} MB`
  return `${(b / 1073741824).toFixed(2)} GB`
}

export function QuarantineView({ onNav }: Props) {
  const qc = useQueryClient()
  const toast = useToast()

  const { data: active = [] } = useQuery({
    queryKey: ['quarantine'],
    queryFn: () => listQuarantine(),
    refetchInterval: 30000,
    staleTime: 10000,
  })

  const { data: removed = [] } = useQuery({
    queryKey: ['quarantine-removed'],
    queryFn: () => listRemovedQuarantine(),
    refetchInterval: 30000,
    staleTime: 10000,
  })

  const { data: jobs = [] } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
    staleTime: 30000,
  })

  const jobMap = new Map(jobs.map(j => [j.id, j]))

  // Group by job, preserving order of first appearance.
  type Group = { jobId: number; active: QuarantineEntry[]; removed: QuarantineEntry[] }
  const groups: Group[] = []
  const seen = new Map<number, Group>()

  const allEntries = [...active, ...removed]
  for (const e of allEntries) {
    const id = e.job_id ?? 0
    if (!seen.has(id)) {
      const g: Group = { jobId: id, active: [], removed: [] }
      seen.set(id, g)
      groups.push(g)
    }
    const g = seen.get(id)!
    if (e.status === 'active') g.active.push(e)
    else g.removed.push(e)
  }

  // Jobs with active quarantine entries first.
  groups.sort((a, b) => b.active.length - a.active.length)

  const clearRemoved = useMutation({
    mutationFn: (jobId: number | undefined) => clearRemovedQuarantine(jobId),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['quarantine-removed'] })
      toast('Recently removed list cleared.', 'ok')
    },
    onError: () => toast('Failed to clear removed entries.', 'err'),
  })

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Quarantine</div>
      </div>

      {groups.length === 0 && (
        <Card>
          <div className="empty">
            <Archive size={44}/>
            <div className="empty-title">No quarantined files</div>
            <div className="empty-desc">
              Files removed during mirror or two-way sync are held here.
              Nothing is here yet.
            </div>
          </div>
        </Card>
      )}

      {groups.map(({ jobId, active: activeEntries, removed: removedEntries }) => {
        const job = jobMap.get(jobId)
        const hasBoth = activeEntries.length > 0 && removedEntries.length > 0
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
              {activeEntries.length > 0 && (
                <Badge variant="pending">{activeEntries.length} in quarantine</Badge>
              )}
            </div>

            {/* Active quarantine entries */}
            {activeEntries.length > 0 && (
              <div style={{
                padding: '12px 16px',
                borderBottom: hasBoth ? '1px solid var(--border)' : undefined,
              }}>
                <QuarantineCard
                  entries={activeEntries}
                  onChanged={() => {
                    qc.invalidateQueries({ queryKey: ['quarantine'] })
                    qc.invalidateQueries({ queryKey: ['quarantine-removed'] })
                  }}
                />
              </div>
            )}

            {/* Recently removed */}
            {removedEntries.length > 0 && (
              <div style={{ padding: '12px 16px' }}>
                <div style={{ display: 'flex', alignItems: 'center', marginBottom: 10 }}>
                  <span className="fs11 text3" style={{ fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.04em', flex: 1 }}>
                    Recently Removed
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => clearRemoved.mutate(jobId)}
                    disabled={clearRemoved.isPending}
                  >
                    Clear
                  </Button>
                </div>
                <div className="tbl-wrap">
                  <table>
                    <thead><tr>
                      <th>File</th><th>Size</th><th>Action</th><th>Removed at</th>
                    </tr></thead>
                    <tbody>
                      {removedEntries.slice(0, 10).map(e => (
                        <tr key={e.id}>
                          <td className="td-mono">{e.rel_path}</td>
                          <td className="td-muted">{fmtBytes(e.size_bytes)}</td>
                          <td>
                            <span className={e.status === 'restored' ? 'badge b-synced' : 'badge b-disabled'}>
                              {e.status}
                            </span>
                          </td>
                          <td className="td-muted">{e.removed_at ? fmtDate(e.removed_at) : '—'}</td>
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
