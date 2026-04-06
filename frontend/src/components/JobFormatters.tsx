import { Badge } from './Badge'
import type { Job } from '../api/types'

export function StatusBadge({ status }: { status: Job['status'] }) {
  const variantMap: Record<Job['status'], 'running' | 'synced' | 'pending' | 'error' | 'disabled'> = {
    running: 'running', idle: 'synced', paused: 'pending', error: 'error', disabled: 'disabled',
  }
  const labelMap: Record<Job['status'], string> = {
    running: 'Running', idle: 'Synced', paused: 'Stopped', error: 'Error', disabled: 'Disabled',
  }
  return <Badge variant={variantMap[status]}>{labelMap[status]}</Badge>
}

export function ModePill({ mode }: { mode: Job['mode'] }) {
  const labelMap: Record<Job['mode'], string> = {
    'one-way-backup': 'One-way backup',
    'one-way-mirror': 'One-way mirror',
    'two-way': 'Two-way',
  }
  return <span className="mode-pill">{labelMap[mode]}</span>
}
