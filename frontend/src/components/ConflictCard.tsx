import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { resolveConflict } from '../api/client'
import { Button } from './Button'
import { useToast } from './Toast'
import type { Conflict } from '../api/types'

interface Props {
  conflicts: Conflict[]   // pending conflicts only
  onChanged: () => void
}

function fmtDate(d: string) {
  return new Date(d).toLocaleString()
}

function fmtBytes(b: number) {
  if (b < 1024) return `${b} B`
  if (b < 1048576) return `${(b / 1024).toFixed(1)} KB`
  if (b < 1073741824) return `${(b / 1048576).toFixed(1)} MB`
  return `${(b / 1073741824).toFixed(2)} GB`
}

export function ConflictCard({ conflicts, onChanged }: Props) {
  const toast = useToast()
  const [selected, setSelected] = useState<Conflict | null>(null)

  const resolve = useMutation({
    mutationFn: ({ id, action }: { id: number; action: string }) => resolveConflict(id, action),
    onSuccess: () => {
      onChanged()
      toast('Conflict resolved.', 'ok')
      setSelected(null)
    },
    onError: () => toast('Failed to resolve conflict.', 'err'),
  })

  if (conflicts.length === 0) return null

  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', marginBottom: 10 }}>
        <div className="fs12 text3">
          {conflicts.length} conflict{conflicts.length === 1 ? '' : 's'} awaiting resolution — click a row to resolve.
        </div>
      </div>

      <div className="tbl-wrap">
        <table>
          <thead><tr>
            <th>File</th><th>Detected</th><th>Src size</th><th>Dst size</th><th>Strategy</th>
          </tr></thead>
          <tbody>
            {conflicts.map(c => (
              <tr
                key={c.id}
                onClick={() => setSelected(selected?.id === c.id ? null : c)}
                style={{ background: selected?.id === c.id ? 'var(--active)' : '', cursor: 'pointer' }}
              >
                <td className="td-mono">{c.rel_path}</td>
                <td className="td-muted">{fmtDate(c.created_at)}</td>
                <td className="td-muted">{fmtBytes(c.src_size)}</td>
                <td className="td-muted">{fmtBytes(c.dest_size)}</td>
                <td><span className="badge b-pending">{c.strategy}</span></td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {selected && (
        <div style={{
          marginTop: 12,
          padding: 14,
          background: 'var(--input-bg)',
          border: '1px solid var(--border)',
          borderRadius: 'var(--radius)',
        }}>
          <div style={{ fontSize: 13, fontWeight: 600, marginBottom: 4 }}>
            Resolve —{' '}
            <span className="mono" style={{ fontWeight: 400 }}>{selected.rel_path}</span>
          </div>

          <div className="diff-cols" style={{ marginBottom: 12 }}>
            <div className="diff-side">
              <div className="diff-hd src">Source</div>
              <div className="diff-body">
                <div className="diff-row"><strong>Size</strong>: {fmtBytes(selected.src_size)}</div>
                <div className="diff-row"><strong>Modified</strong>: {fmtDate(selected.src_mod_time)}</div>
                <div className="diff-row"><strong>Hash</strong>: <span className="mono" style={{ fontSize: 10 }}>{selected.src_content_hash.slice(0, 16)}…</span></div>
              </div>
            </div>
            <div className="diff-side">
              <div className="diff-hd dst">Destination</div>
              <div className="diff-body">
                <div className="diff-row"><strong>Size</strong>: {fmtBytes(selected.dest_size)}</div>
                <div className="diff-row"><strong>Modified</strong>: {fmtDate(selected.dest_mod_time)}</div>
                <div className="diff-row"><strong>Hash</strong>: <span className="mono" style={{ fontSize: 10 }}>{selected.dest_content_hash.slice(0, 16)}…</span></div>
              </div>
            </div>
          </div>

          <div className="flex gap8">
            <Button
              variant="primary"
              onClick={() => resolve.mutate({ id: selected.id, action: 'keep-source' })}
              disabled={resolve.isPending}
            >
              Keep source
            </Button>
            <Button
              variant="secondary"
              onClick={() => resolve.mutate({ id: selected.id, action: 'keep-dest' })}
              disabled={resolve.isPending}
            >
              Keep destination
            </Button>
            <Button
              variant="ghost"
              onClick={() => resolve.mutate({ id: selected.id, action: 'keep-both' })}
              disabled={resolve.isPending}
            >
              Keep both
            </Button>
            <div style={{ flex: 1 }} />
            <Button variant="ghost" onClick={() => setSelected(null)}>Dismiss</Button>
          </div>
        </div>
      )}
    </>
  )
}
