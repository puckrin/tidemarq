import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listUsers, createUser, deleteUser, getHealth } from '../api/client'
import { useAuth } from '../store/auth'
import { Button } from '../components/Button'
import { Modal } from '../components/Modal'
import { useToast } from '../components/Toast'
import type { User } from '../api/types'

type Tab = 'general' | 'users' | 'about'

export function SettingsView() {
  const [tab, setTab] = useState<Tab>('general')
  const { user } = useAuth()
  const qc = useQueryClient()
  const toast = useToast()

  const { data: users = [] } = useQuery({ queryKey: ['users'], queryFn: listUsers })
  const { data: health }      = useQuery({ queryKey: ['health'], queryFn: getHealth })

  // New user form
  const [newUser, setNewUser] = useState({ username: '', password: '', role: 'viewer' as User['role'] })
  const [delTarget, setDelTarget] = useState<User | null>(null)

  const addUser = useMutation({
    mutationFn: () => createUser(newUser),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] })
      toast('User created.', 'ok')
      setNewUser({ username: '', password: '', role: 'viewer' })
    },
    onError: () => toast('Failed to create user.', 'err'),
  })

  const delUser = useMutation({
    mutationFn: (id: number) => deleteUser(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] })
      toast('User deleted.', 'ok')
      setDelTarget(null)
    },
    onError: () => toast('Failed to delete user.', 'err'),
  })

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Settings</div>
      </div>

      <div className="stabs">
        {(['general','users','about'] as Tab[]).map(t => (
          <div key={t} className={`stab${tab===t?' active':''}`} onClick={() => setTab(t)}>
            {t.charAt(0).toUpperCase()+t.slice(1)}
          </div>
        ))}
      </div>

      {/* General */}
      {tab === 'general' && (
        <div>
          <div className="ssec">
            <div className="ssec-title">Sync Defaults</div>
            <div className="srow">
              <div>
                <div className="srow-name">Default conflict strategy</div>
                <div className="srow-desc">Applied when creating new jobs</div>
              </div>
              <select className="fs" style={{ maxWidth: 280 }}>
                <option>Ask user</option>
                <option>Newest wins</option>
                <option>Source wins</option>
              </select>
            </div>
            <div className="srow">
              <div>
                <div className="srow-name">Version history</div>
                <div className="srow-desc">Default number of versions to keep per file</div>
              </div>
              <input className="fi" style={{ maxWidth: 100 }} defaultValue="10" type="number" min={0}/>
            </div>
            <div className="srow">
              <div>
                <div className="srow-name">Soft delete retention</div>
                <div className="srow-desc">Days before quarantined files are permanently removed</div>
              </div>
              <input className="fi" style={{ maxWidth: 100 }} defaultValue="14" type="number" min={1}/>
            </div>
          </div>
          <div className="ssec">
            <div className="ssec-title">Security</div>
            <div className="srow">
              <div>
                <div className="srow-name">Session timeout</div>
                <div className="srow-desc">JWT token expiry (requires re-login)</div>
              </div>
              <select className="fs" style={{ maxWidth: 180 }}>
                <option>8 hours</option>
                <option>24 hours</option>
                <option>7 days</option>
              </select>
            </div>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button variant="primary" onClick={() => toast('Settings saved.', 'ok')}>Save changes</Button>
          </div>
        </div>
      )}

      {/* Users */}
      {tab === 'users' && (
        <div>
          <div className="ssec">
            <div className="ssec-title">User Accounts</div>
            <div className="tbl-wrap">
              <table>
                <thead><tr><th>Username</th><th>Role</th><th>Created</th><th></th></tr></thead>
                <tbody>
                  {users.map(u => (
                    <tr key={u.id}>
                      <td className="fw5">{u.username}</td>
                      <td><span className="badge b-tag">{u.role}</span></td>
                      <td className="td-muted">{new Date(u.created_at).toLocaleDateString()}</td>
                      <td>
                        {u.username !== user?.username && (
                          <Button variant="ghost" size="sm" onClick={() => setDelTarget(u)}>Delete</Button>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
          {user?.role === 'admin' && (
            <div className="ssec">
              <div className="ssec-title">Add User</div>
              <div className="grid3" style={{ gap: 16 }}>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Username</label>
                  <input className="fi" value={newUser.username} onChange={e => setNewUser(u => ({...u, username: e.target.value}))}/>
                </div>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Password</label>
                  <input className="fi" type="password" value={newUser.password} onChange={e => setNewUser(u => ({...u, password: e.target.value}))}/>
                </div>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Role</label>
                  <select className="fs" value={newUser.role} onChange={e => setNewUser(u => ({...u, role: e.target.value as User['role']}))}>
                    <option value="admin">Admin</option>
                    <option value="operator">Operator</option>
                    <option value="viewer">Viewer</option>
                  </select>
                </div>
              </div>
              <div style={{ marginTop: 16 }}>
                <Button variant="primary" onClick={() => addUser.mutate()} disabled={addUser.isPending || !newUser.username || !newUser.password}>
                  {addUser.isPending ? 'Creating…' : 'Create user'}
                </Button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* About */}
      {tab === 'about' && (
        <div className="ssec">
          <div className="ssec-title">System Information</div>
          <div className="srow">
            <div><div className="srow-name">Version</div></div>
            <div className="mono fs12">{health?.version ?? '—'}</div>
          </div>
          <div className="srow">
            <div><div className="srow-name">Database</div></div>
            <div className="fs13">{health?.database ?? '—'}</div>
          </div>
          <div className="srow">
            <div><div className="srow-name">Uptime</div></div>
            <div className="fs13">{health?.uptime ?? '—'}</div>
          </div>
        </div>
      )}

      <Modal
        open={!!delTarget}
        title="Delete user?"
        body={`This will permanently delete the account for "${delTarget?.username}".`}
        confirmLabel="Delete"
        confirmVariant="danger"
        onConfirm={() => delTarget && delUser.mutate(delTarget.id)}
        onClose={() => setDelTarget(null)}
      />
    </div>
  )
}
