import { Archive } from 'lucide-react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { listQuarantine, listJobs } from '../api/client'
import { Card, CardHeader } from '../components/Card'
import { QuarantineCard } from '../components/QuarantineCard'
import { Badge } from '../components/Badge'
import type { QuarantineEntry } from '../api/types'
import type { View } from '../components/Sidebar'

interface Props { onNav: (v: View, id?: number) => void }

export function QuarantineView({ onNav }: Props) {
  const qc = useQueryClient()

  const { data: allEntries = [] } = useQuery({
    queryKey: ['quarantine'],
    queryFn: () => listQuarantine(),
    refetchInterval: 30000,
    staleTime: 10000,
  })

  const { data: jobs = [] } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
    staleTime: 30000,
  })

  // Group entries by job_id, preserving the order jobs appear in the jobs list.
  const jobMap = new Map(jobs.map(j => [j.id, j]))

  const grouped: { jobId: number; jobName: string; entries: QuarantineEntry[] }[] = []
  const seen = new Map<number, QuarantineEntry[]>()

  for (const e of allEntries) {
    const id = e.job_id ?? 0
    if (!seen.has(id)) {
      seen.set(id, [])
      grouped.push({ jobId: id, jobName: jobMap.get(id)?.name ?? `Job #${id}`, entries: seen.get(id)! })
    }
    seen.get(id)!.push(e)
  }

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Quarantine</div>
      </div>

      {grouped.length === 0 && (
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

      {grouped.map(({ jobId, jobName, entries }) => {
        const job = jobMap.get(jobId)
        return (
          <Card key={jobId} style={{ marginBottom: 16 }}>
            <CardHeader
              title={
                <div className="flex gap8" style={{ alignItems: 'center', flexWrap: 'wrap' }}>
                  <span
                    style={{ cursor: 'pointer', fontWeight: 600 }}
                    onClick={() => onNav('job-detail', jobId)}
                  >
                    {jobName}
                  </span>
                  {job && (
                    <>
                      <span className="mode-pill" style={{ fontSize: 11 }}>
                        {job.mode.replace(/-/g, ' ')}
                      </span>
                      <Badge variant={
                        job.status === 'running' ? 'running'
                        : job.status === 'idle'   ? 'synced'
                        : job.status === 'error'  ? 'error'
                        : job.status === 'paused' ? 'pending'
                        : 'disabled'
                      }>
                        {job.status}
                      </Badge>
                    </>
                  )}
                  {job && (
                    <span className="fs12 text3 mono" style={{ fontWeight: 400 }}>
                      {job.destination_path}
                    </span>
                  )}
                </div>
              }
            />
            <QuarantineCard
              entries={entries}
              onChanged={() => qc.invalidateQueries({ queryKey: ['quarantine'] })}
            />
          </Card>
        )
      })}
    </div>
  )
}
