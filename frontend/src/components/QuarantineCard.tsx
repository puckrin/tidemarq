import { useState } from 'react'
import { RotateCcw, Trash2 } from 'lucide-react'
import { useMutation } from '@tanstack/react-query'
import { restoreQuarantine, deleteQuarantineEntry } from '../api/client'
import { Button } from './Button'
import { Modal } from './Modal'
import { useToast } from './Toast'
import type { QuarantineEntry } from '../api/types'

interface Props {
  entries: QuarantineEntry[]
  onChanged: () => void
}

function fmtBytes(b: number) {
  if (b < 1024) return `${b} B`
  if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1073741824) return `${(b / 1048576).toFixed(1)} MB`
  return `${(b / 1073741824).toFixed(2)} GB`
}

function timeRemaining(expiresAt: string): { label: string; urgent: boolean; critical: boolean } {
  const diff = new Date(expiresAt).getTime() - Date.now()
  if (diff <= 0) return { label: 'Expired', urgent: true, critical: true }
  const days = Math.floor(diff / 86400000)
  const hours = Math.floor((diff % 86400000) / 3600000)
  const critical = days < 3
  const urgent = days < 7
  if (days > 7)  return { label: `${days}d`, urgent: false, critical: false }
  if (days > 0)  return { label: `${days}d ${hours}h`, urgent, critical }
  if (hours > 0) return { label: `${hours}h`, urgent: true, critical: true }
  const mins = Math.floor((diff % 3600000) / 60000)
  return { label: `${mins}m`, urgent: true, critical: true }
}

export function QuarantineCard({ entries, onChanged }: Props) {
  const toast = useToast()
  const [deleteAllModal, setDeleteAllModal] = useState(false)
  const [bulkWorking, setBulkWorking] = useState(false)

  const restore = useMutation({
    mutationFn: (id: number) => restoreQuarantine(id),
    onSuccess: () => { onChanged(); toast('File restored to destination.', 'ok') },
    onError:   () => toast('Failed to restore file.', 'err'),
  })

  const purge = useMutation({
    mutationFn: (id: number) => deleteQuarantineEntry(id),
    onSuccess: () => { onChanged(); toast('File permanently deleted.', 'ok') },
    onError:   () => toast('Failed to delete file.', 'err'),
  })

  const handleRestoreAll = async () => {
    setBulkWorking(true)
    try {
      for (const e of entries) await restoreQuarantine(e.id)
      onChanged()
      toast(`${entries.length} file${entries.length === 1 ? '' : 's'} restored.`, 'ok')
    } catch {
      toast('Some files could not be restored.', 'err')
      onChanged()
    } finally {
      setBulkWorking(false)
    }
  }

  const handleDeleteAll = async () => {
    setDeleteAllModal(false)
    setBulkWorking(true)
    try {
      for (const e of entries) await deleteQuarantineEntry(e.id)
      onChanged()
      toast(`${entries.length} file${entries.length === 1 ? '' : 's'} permanently deleted.`, 'ok')
    } catch {
      toast('Some files could not be deleted.', 'err')
      onChanged()
    } finally {
      setBulkWorking(false)
    }
  }

  if (entries.length === 0) return null

  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 10 }}>
        <div className="fs12 text3">
          {entries.length} file{entries.length === 1 ? '' : 's'} held in quarantine — restore to put them back or delete to remove permanently.
        </div>
        <div className="flex gap8">
          <Button variant="ghost" size="sm" onClick={handleRestoreAll} disabled={bulkWorking}>
            <RotateCcw size={12}/> Restore all
          </Button>
          <Button
            variant="ghost"
            size="sm"
            style={{ color: 'var(--coral-light)' }}
            onClick={() => setDeleteAllModal(true)}
            disabled={bulkWorking}
          >
            <Trash2 size={12}/> Delete all
          </Button>
        </div>
      </div>

      <div className="tbl-wrap">
        <table>
          <thead><tr>
            <th>File</th>
            <th>Removed</th>
            <th>Expires in</th>
            <th>Size</th>
            <th style={{ width: 150 }}></th>
          </tr></thead>
          <tbody>
            {entries.map(q => {
              const remaining = timeRemaining(q.expires_at)
              return (
                <tr key={q.id}>
                  <td
                    className="mono fs12"
                    style={{ maxWidth: 340, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
                    title={q.rel_path}
                  >
                    {q.rel_path}
                  </td>
                  <td className="td-muted">{new Date(q.deleted_at).toLocaleString()}</td>
                  <td style={{
                    fontWeight: remaining.urgent ? 600 : undefined,
                    color: remaining.critical
                      ? 'var(--coral-light)'
                      : remaining.urgent
                      ? 'var(--amber-light)'
                      : 'var(--text2)',
                    fontSize: 12,
                  }}>
                    {remaining.label}
                  </td>
                  <td className="td-muted">{fmtBytes(q.size_bytes)}</td>
                  <td>
                    <div className="flex gap8" style={{ justifyContent: 'flex-end' }}>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => restore.mutate(q.id)}
                        disabled={restore.isPending || purge.isPending || bulkWorking}
                      >
                        <RotateCcw size={12}/> Restore
                      </Button>
                      <Button
                        variant="ghost"
                        size="sm"
                        style={{ color: 'var(--coral-light)' }}
                        onClick={() => purge.mutate(q.id)}
                        disabled={restore.isPending || purge.isPending || bulkWorking}
                      >
                        <Trash2 size={12}/> Delete
                      </Button>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>

      <Modal
        open={deleteAllModal}
        title={`Delete all ${entries.length} quarantined file${entries.length === 1 ? '' : 's'}?`}
        body="This will permanently remove all quarantined files for this job. This cannot be undone."
        confirmLabel="Delete all"
        confirmVariant="danger"
        onConfirm={handleDeleteAll}
        onClose={() => setDeleteAllModal(false)}
      />
    </>
  )
}
