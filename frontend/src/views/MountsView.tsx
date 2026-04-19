import { useState } from 'react'
import { HardDrive, Plus, Trash2, Pencil, Check, X } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listMounts, createMount, updateMount, deleteMount, testMount } from '../api/client'
import { Button } from '../components/Button'
import { Card } from '../components/Card'
import { Modal } from '../components/Modal'
import { useToast } from '../components/Toast'
import type { Mount, MountInput } from '../api/types'

const INIT_SFTP: MountInput = {
  name: '', type: 'sftp', host: '', port: 22, username: '',
  password: '', ssh_key: '', sftp_host_key: '',
}
const INIT_SMB: MountInput = {
  name: '', type: 'smb', host: '', port: 445, username: '',
  password: '', smb_share: '', smb_domain: '',
}

function defaultForm(type: 'sftp' | 'smb'): MountInput {
  return type === 'sftp' ? { ...INIT_SFTP } : { ...INIT_SMB }
}

function MountForm({
  initial,
  onSave,
  onCancel,
  saving,
}: {
  initial: MountInput
  onSave: (m: MountInput) => void
  onCancel: () => void
  saving: boolean
}) {
  const [form, setForm] = useState<MountInput>(initial)
  const setField = (updates: Partial<MountInput>) =>
    setForm(f => ({ ...f, ...updates }))

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 14 }}>
      <div className="fg" style={{ marginBottom: 0 }}>
        <label className="fl">Name</label>
        <input className="fi" value={form.name} onChange={e => setField({ name: e.target.value })} placeholder="e.g. NAS backup share" />
      </div>

      <div className="fg" style={{ marginBottom: 0 }}>
        <label className="fl">Type</label>
        <select className="fs" value={form.type} onChange={e => {
          const t = e.target.value as 'sftp' | 'smb'
          setForm({ ...defaultForm(t), name: form.name })  // intentional full reset on type change
        }}>
          <option value="sftp">SFTP</option>
          <option value="smb">SMB / CIFS</option>
        </select>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr auto', gap: 8 }}>
        <div className="fg" style={{ marginBottom: 0 }}>
          <label className="fl">Host</label>
          <input className="fi mono" style={{ fontSize: 13 }} value={form.host} onChange={e => setField({ host: e.target.value })} placeholder="192.168.1.100" />
        </div>
        <div className="fg" style={{ marginBottom: 0 }}>
          <label className="fl">Port</label>
          <input className="fi" style={{ maxWidth: 80 }} type="number" min={1} max={65535}
            value={form.port} onChange={e => setField({ port: Number(e.target.value) })} />
        </div>
      </div>

      <div className="fg" style={{ marginBottom: 0 }}>
        <label className="fl">Username</label>
        <input className="fi" value={form.username} onChange={e => setField({ username: e.target.value })} autoComplete="username" />
      </div>

      <div className="fg" style={{ marginBottom: 0 }}>
        <label className="fl">Password <span className="text3">(leave blank to keep current)</span></label>
        <input className="fi" type="password" value={form.password ?? ''} onChange={e => setField({ password: e.target.value })} autoComplete="new-password" />
      </div>

      {form.type === 'sftp' && (
        <>
          <div className="fg" style={{ marginBottom: 0 }}>
            <label className="fl">SSH private key <span className="text3">(PEM, leave blank to use password)</span></label>
            <textarea
              className="fi mono"
              style={{ fontSize: 12, resize: 'vertical', minHeight: 80 }}
              value={form.ssh_key ?? ''}
              onChange={e => setField({ ssh_key: e.target.value })}
              placeholder="-----BEGIN OPENSSH PRIVATE KEY-----"
            />
          </div>
          <div className="fg" style={{ marginBottom: 0 }}>
            <label className="fl">Known host key fingerprint <span className="text3">(SHA256: — leave blank to trust on first connect)</span></label>
            <input className="fi mono" style={{ fontSize: 12 }} value={form.sftp_host_key ?? ''} onChange={e => setField({ sftp_host_key: e.target.value })} placeholder="SHA256:..." />
          </div>
        </>
      )}

      {form.type === 'smb' && (
        <>
          <div className="fg" style={{ marginBottom: 0 }}>
            <label className="fl">Share name</label>
            <input className="fi mono" style={{ fontSize: 13 }} value={form.smb_share ?? ''} onChange={e => setField({ smb_share: e.target.value })} placeholder="backup" />
          </div>
          <div className="fg" style={{ marginBottom: 0 }}>
            <label className="fl">Domain <span className="text3">(optional)</span></label>
            <input className="fi" value={form.smb_domain ?? ''} onChange={e => setField({ smb_domain: e.target.value })} placeholder="WORKGROUP" />
          </div>
        </>
      )}

      <div style={{ display: 'flex', gap: 8, justifyContent: 'flex-end', marginTop: 4 }}>
        <Button variant="ghost" onClick={onCancel}>Cancel</Button>
        <Button variant="primary" disabled={saving || !form.name || !form.host}
          onClick={() => onSave(form)}>
          {saving ? 'Saving…' : <><Check size={14} /> Save mount</>}
        </Button>
      </div>
    </div>
  )
}

export function MountsView() {
  const qc = useQueryClient()
  const toast = useToast()

  const { data: mounts = [] } = useQuery({ queryKey: ['mounts'], queryFn: listMounts })

  const [creating, setCreating]   = useState(false)
  const [editing, setEditing]     = useState<Mount | null>(null)
  const [delTarget, setDelTarget] = useState<Mount | null>(null)
  const [testing, setTesting]     = useState<number | null>(null)
  const [testResult, setTestResult] = useState<Record<number, { ok: boolean; error?: string }>>({})

  const createMut = useMutation({
    mutationFn: (data: MountInput) => createMount(data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['mounts'] })
      toast('Mount created.', 'ok')
      setCreating(false)
    },
    onError: () => toast('Failed to create mount.', 'err'),
  })

  const updateMut = useMutation({
    mutationFn: ({ id, data }: { id: number; data: MountInput }) => updateMount(id, data),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['mounts'] })
      toast('Mount updated.', 'ok')
      setEditing(null)
    },
    onError: () => toast('Failed to update mount.', 'err'),
  })

  const deleteMut = useMutation({
    mutationFn: (id: number) => deleteMount(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['mounts'] })
      toast('Mount deleted.', 'ok')
      setDelTarget(null)
    },
    onError: () => toast('Failed to delete mount.', 'err'),
  })

  const handleTest = async (m: Mount) => {
    setTesting(m.id)
    try {
      const result = await testMount(m.id)
      setTestResult(prev => ({ ...prev, [m.id]: result }))
    } catch {
      setTestResult(prev => ({ ...prev, [m.id]: { ok: false, error: 'Request failed' } }))
    } finally {
      setTesting(null)
    }
  }

  const editInitial = (m: Mount): MountInput => ({
    name:         m.name,
    type:         m.type,
    host:         m.host,
    port:         m.port || (m.type === 'sftp' ? 22 : 445),
    username:     m.username,
    password:     '',
    ssh_key:      '',
    smb_share:    m.smb_share,
    smb_domain:   m.smb_domain,
    sftp_host_key: m.sftp_host_key,
  })

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Network Mounts</div>
        <Button variant="primary" size="sm" className="mla" onClick={() => { setCreating(true); setEditing(null) }}>
          <Plus size={14} /> Add mount
        </Button>
      </div>

      {creating && (
        <Card style={{ marginBottom: 16 }}>
          <div className="card-title mb16">New mount</div>
          <MountForm
            initial={defaultForm('sftp')}
            onSave={data => createMut.mutate(data)}
            onCancel={() => setCreating(false)}
            saving={createMut.isPending}
          />
        </Card>
      )}

      {mounts.length === 0 && !creating && (
        <Card>
          <div className="empty">
            <HardDrive size={44} />
            <div className="empty-title">No mounts configured</div>
            <div className="empty-desc">Add an SFTP or SMB network mount to use as a job source or destination.</div>
            <Button variant="primary" onClick={() => setCreating(true)}><Plus size={14} /> Add mount</Button>
          </div>
        </Card>
      )}

      {mounts.map(m => (
        <Card key={m.id} style={{ marginBottom: 12 }}>
          {editing?.id === m.id ? (
            <>
              <div className="card-title mb16">Edit mount — {m.name}</div>
              <MountForm
                initial={editInitial(m)}
                onSave={data => updateMut.mutate({ id: m.id, data })}
                onCancel={() => setEditing(null)}
                saving={updateMut.isPending}
              />
            </>
          ) : (
            <div style={{ display: 'flex', alignItems: 'center', gap: 12 }}>
              <HardDrive size={20} style={{ flexShrink: 0, color: 'var(--text2)' }} />
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontWeight: 600, fontSize: 14 }}>{m.name}</div>
                <div className="mono fs12 text2">
                  {m.type.toUpperCase()} · {m.host}:{m.port || (m.type === 'sftp' ? 22 : 445)}
                  {m.type === 'smb' && m.smb_share ? ` / ${m.smb_share}` : ''}
                  {m.username ? ` · ${m.username}` : ''}
                </div>
                {(() => { const tr = testResult[m.id]; return tr && (
                  <div style={{ marginTop: 4, fontSize: 12 }}>
                    {tr.ok
                      ? <span style={{ color: 'var(--grass-light)' }}><Check size={11} style={{ display: 'inline' }}/> Reachable</span>
                      : <span style={{ color: 'var(--coral-light)' }}><X size={11} style={{ display: 'inline' }}/> {tr.error}</span>}
                  </div>
                )})()}
              </div>
              <div style={{ display: 'flex', gap: 6, flexShrink: 0 }}>
                <Button variant="ghost" size="sm"
                  disabled={testing === m.id}
                  onClick={() => handleTest(m)}>
                  {testing === m.id ? 'Testing…' : 'Test'}
                </Button>
                <Button variant="ghost" size="sm" onClick={() => { setEditing(m); setCreating(false) }}>
                  <Pencil size={13} />
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setDelTarget(m)}>
                  <Trash2 size={13} />
                </Button>
              </div>
            </div>
          )}
        </Card>
      ))}

      <Modal
        open={!!delTarget}
        title="Delete mount?"
        body={`This will remove the mount configuration for "${delTarget?.name}". Jobs using this mount will stop working.`}
        confirmLabel="Delete"
        confirmVariant="danger"
        onConfirm={() => delTarget && deleteMut.mutate(delTarget.id)}
        onClose={() => setDelTarget(null)}
      />
    </div>
  )
}
