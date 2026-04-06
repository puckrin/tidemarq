import { useState, useEffect } from 'react'
import { Plus, Trash2 } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { listUsers, createUser, updateUser, deleteUser, getHealth,
         listNotificationTargets, createNotificationTarget, deleteNotificationTarget,
         listNotificationRules, createNotificationRule, deleteNotificationRule } from '../api/client'
import { useAuth } from '../store/auth'
import { Button } from '../components/Button'
import { Modal } from '../components/Modal'
import { useToast } from '../components/Toast'
import type { User, NotificationTarget, NotificationRule } from '../api/types'

type Tab = 'general' | 'users' | 'notifications' | 'about'

interface EditUserState {
  user: User
  password: string
  role: User['role']
}

// Shown once on first login if the password is still the factory default
function FirstRunBanner({ onDismiss }: { onDismiss: () => void }) {
  return (
    <div style={{
      background: 'rgba(194,125,26,0.12)',
      border: '1px solid rgba(245,200,66,0.35)',
      borderRadius: 8,
      padding: '14px 18px',
      marginBottom: 20,
      display: 'flex',
      alignItems: 'center',
      gap: 16,
      fontSize: 13,
    }}>
      <span style={{ color: 'var(--amber-light)', flex: 1 }}>
        ⚠ You are using the default admin password. Please change it in <strong>Settings → Users</strong> before exposing this instance to a network.
      </span>
      <Button variant="ghost" size="sm" onClick={onDismiss}>Dismiss</Button>
    </div>
  )
}

// Detect whether the user is likely still on the default password.
// We can't know for sure from the frontend, so we flag it when the
// only user is "admin" and the instance was created very recently (within 5 minutes).
function useIsDefaultPassword(users: User[]) {
  const SEEN_KEY = 'tidemarq_pwd_warned'
  const [show, setShow] = useState(false)

  useEffect(() => {
    if (sessionStorage.getItem(SEEN_KEY)) return
    const onlyAdmin = users.length === 1 && users[0].username === 'admin'
    if (!onlyAdmin) { sessionStorage.setItem(SEEN_KEY, '1'); return }
    const createdAt = new Date(users[0].created_at).getTime()
    const ageMs = Date.now() - createdAt
    if (ageMs < 5 * 60 * 1000) setShow(true)   // within 5 min of first start
  }, [users])

  const dismiss = () => { sessionStorage.setItem(SEEN_KEY, '1'); setShow(false) }
  return { show, dismiss }
}

export function SettingsView() {
  const [tab, setTab] = useState<Tab>('general')
  const { user: me } = useAuth()
  const qc    = useQueryClient()
  const toast = useToast()

  const { data: users = [] } = useQuery({ queryKey: ['users'], queryFn: listUsers })
  const { data: health }      = useQuery({ queryKey: ['health'], queryFn: getHealth })

  const { show: showBanner, dismiss: dismissBanner } = useIsDefaultPassword(users)

  // New user form
  const [newUser, setNewUser]     = useState({ username: '', password: '', role: 'viewer' as User['role'] })
  const [delTarget, setDelTarget] = useState<User | null>(null)
  const [editTarget, setEditTarget] = useState<EditUserState | null>(null)

  const addUser = useMutation({
    mutationFn: () => createUser(newUser),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] })
      toast('User created.', 'ok')
      setNewUser({ username: '', password: '', role: 'viewer' })
    },
    onError: () => toast('Failed to create user.', 'err'),
  })

  const editUser = useMutation({
    mutationFn: () => updateUser(editTarget!.user.id, {
      role: editTarget!.role,
      ...(editTarget!.password ? { password: editTarget!.password } : {}),
    }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['users'] })
      toast('User updated.', 'ok')
      setEditTarget(null)
    },
    onError: () => toast('Failed to update user.', 'err'),
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

  // ── Notifications state ──────────────────────────────────────────────────
  const { data: notifTargets = [] } = useQuery({
    queryKey: ['notif-targets'],
    queryFn: listNotificationTargets,
    enabled: tab === 'notifications' && me?.role === 'admin',
  })
  const { data: notifRules = [] } = useQuery({
    queryKey: ['notif-rules'],
    queryFn: () => listNotificationRules(),
    enabled: tab === 'notifications' && me?.role === 'admin',
  })

  const [newTarget, setNewTarget] = useState({
    name: '', type: 'webhook' as NotificationTarget['type'], enabled: true,
    config: '{}',
  })
  const [newRule, setNewRule] = useState({
    target_id: 0,
    event: 'job_failed' as NotificationRule['event'],
  })
  const [delNotifTarget, setDelNotifTarget] = useState<NotificationTarget | null>(null)

  const addTarget = useMutation({
    mutationFn: () => {
      let config: object
      try { config = JSON.parse(newTarget.config) } catch { config = {} }
      return createNotificationTarget({ name: newTarget.name, type: newTarget.type, enabled: newTarget.enabled, config })
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notif-targets'] })
      toast('Target created.', 'ok')
      setNewTarget({ name: '', type: 'webhook', enabled: true, config: '{}' })
    },
    onError: () => toast('Failed to create target.', 'err'),
  })

  const delTarget2 = useMutation({
    mutationFn: (id: number) => deleteNotificationTarget(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notif-targets'] })
      qc.invalidateQueries({ queryKey: ['notif-rules'] })
      toast('Target deleted.', 'ok')
      setDelNotifTarget(null)
    },
    onError: () => toast('Failed to delete target.', 'err'),
  })

  const addRule = useMutation({
    mutationFn: () => createNotificationRule({ target_id: newRule.target_id, event: newRule.event }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['notif-rules'] })
      toast('Rule created.', 'ok')
    },
    onError: () => toast('Failed to create rule.', 'err'),
  })

  const delRule = useMutation({
    mutationFn: (id: number) => deleteNotificationRule(id),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['notif-rules'] }) },
    onError: () => toast('Failed to delete rule.', 'err'),
  })

  return (
    <div>
      <div className="page-hd">
        <div className="page-title">Settings</div>
      </div>

      {showBanner && <FirstRunBanner onDismiss={dismissBanner} />}

      <div className="stabs">
        {(['general', 'users', 'notifications', 'about'] as Tab[]).map(t => (
          <div key={t} className={`stab${tab === t ? ' active' : ''}`} onClick={() => setTab(t)}>
            {t.charAt(0).toUpperCase() + t.slice(1)}
          </div>
        ))}
      </div>

      {/* ── General ──────────────────────────────────────── */}
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
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <Button variant="primary" onClick={() => toast('Settings saved.', 'ok')}>Save changes</Button>
          </div>
        </div>
      )}

      {/* ── Users ────────────────────────────────────────── */}
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
                      <td className="fw5">
                        {u.username}
                        {u.username === me?.username && (
                          <span className="badge b-tag" style={{ marginLeft: 8 }}>you</span>
                        )}
                      </td>
                      <td><span className="badge b-tag">{u.role}</span></td>
                      <td className="td-muted">{new Date(u.created_at).toLocaleDateString()}</td>
                      <td>
                        <div style={{ display: 'flex', gap: 6 }}>
                          <Button
                            variant="ghost"
                            size="sm"
                            onClick={() => setEditTarget({ user: u, password: '', role: u.role })}
                          >
                            Edit
                          </Button>
                          {u.username !== me?.username && (
                            <Button variant="ghost" size="sm" onClick={() => setDelTarget(u)}>
                              Delete
                            </Button>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {me?.role === 'admin' && (
            <div className="ssec">
              <div className="ssec-title">Add User</div>
              <div className="grid3" style={{ gap: 16 }}>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Username</label>
                  <input className="fi" value={newUser.username} onChange={e => setNewUser(u => ({ ...u, username: e.target.value }))}/>
                </div>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Password</label>
                  <input className="fi" type="password" value={newUser.password} onChange={e => setNewUser(u => ({ ...u, password: e.target.value }))}/>
                </div>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Role</label>
                  <select className="fs" value={newUser.role} onChange={e => setNewUser(u => ({ ...u, role: e.target.value as User['role'] }))}>
                    <option value="admin">Admin</option>
                    <option value="operator">Operator</option>
                    <option value="viewer">Viewer</option>
                  </select>
                </div>
              </div>
              <div style={{ marginTop: 16 }}>
                <Button
                  variant="primary"
                  onClick={() => addUser.mutate()}
                  disabled={addUser.isPending || !newUser.username || !newUser.password}
                >
                  {addUser.isPending ? 'Creating…' : 'Create user'}
                </Button>
              </div>
            </div>
          )}
        </div>
      )}

      {/* ── Notifications ────────────────────────────────── */}
      {tab === 'notifications' && me?.role === 'admin' && (
        <div>
          <div className="ssec">
            <div className="ssec-title">Notification Targets</div>
            {notifTargets.length > 0 && (
              <div className="tbl-wrap" style={{ marginBottom: 16 }}>
                <table>
                  <thead><tr><th>Name</th><th>Type</th><th>Enabled</th><th></th></tr></thead>
                  <tbody>
                    {notifTargets.map(t => (
                      <tr key={t.id}>
                        <td className="fw5">{t.name}</td>
                        <td><span className="badge b-tag">{t.type}</span></td>
                        <td>{t.enabled ? 'Yes' : 'No'}</td>
                        <td>
                          <Button variant="ghost" size="sm" onClick={() => setDelNotifTarget(t)}>
                            <Trash2 size={12}/>
                          </Button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
            <div className="card-title mb12" style={{ fontSize: 13 }}>Add target</div>
            <div className="grid3" style={{ gap: 12 }}>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">Name</label>
                <input className="fi" value={newTarget.name} onChange={e => setNewTarget(t => ({ ...t, name: e.target.value }))} />
              </div>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">Type</label>
                <select className="fs" value={newTarget.type} onChange={e => setNewTarget(t => ({ ...t, type: e.target.value as NotificationTarget['type'] }))}>
                  <option value="webhook">Webhook</option>
                  <option value="smtp">SMTP</option>
                  <option value="gotify">Gotify</option>
                </select>
              </div>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">Enabled</label>
                <select className="fs" value={newTarget.enabled ? '1' : '0'} onChange={e => setNewTarget(t => ({ ...t, enabled: e.target.value === '1' }))}>
                  <option value="1">Yes</option>
                  <option value="0">No</option>
                </select>
              </div>
            </div>
            <div className="fg" style={{ marginBottom: 0, marginTop: 12 }}>
              <label className="fl">Config (JSON)</label>
              <textarea
                className="fi mono"
                style={{ fontSize: 12, resize: 'vertical', minHeight: 80 }}
                value={newTarget.config}
                onChange={e => setNewTarget(t => ({ ...t, config: e.target.value }))}
                placeholder={newTarget.type === 'webhook'
                  ? '{"url":"https://hooks.example.com/...","method":"POST"}'
                  : newTarget.type === 'smtp'
                    ? '{"host":"smtp.example.com","port":587,"username":"user","password":"...","from":"tidemarq@example.com","to":"admin@example.com"}'
                    : '{"url":"https://gotify.example.com","app_token":"...","priority":5}'}
              />
            </div>
            <div style={{ marginTop: 12 }}>
              <Button variant="primary" disabled={addTarget.isPending || !newTarget.name}
                onClick={() => addTarget.mutate()}>
                <Plus size={13}/> {addTarget.isPending ? 'Creating…' : 'Add target'}
              </Button>
            </div>
          </div>

          {notifTargets.length > 0 && (
            <div className="ssec">
              <div className="ssec-title">Notification Rules</div>
              {notifRules.length > 0 && (
                <div className="tbl-wrap" style={{ marginBottom: 16 }}>
                  <table>
                    <thead><tr><th>Target</th><th>Event</th><th></th></tr></thead>
                    <tbody>
                      {notifRules.map(r => (
                        <tr key={r.id}>
                          <td>{notifTargets.find(t => t.id === r.target_id)?.name ?? r.target_id}</td>
                          <td><span className="badge b-tag">{r.event.replace(/_/g, ' ')}</span></td>
                          <td>
                            <Button variant="ghost" size="sm" onClick={() => delRule.mutate(r.id)}>
                              <Trash2 size={12}/>
                            </Button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
              <div style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Target</label>
                  <select className="fs" value={newRule.target_id}
                    onChange={e => setNewRule(r => ({ ...r, target_id: Number(e.target.value) }))}>
                    <option value={0}>— select —</option>
                    {notifTargets.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}
                  </select>
                </div>
                <div className="fg" style={{ marginBottom: 0 }}>
                  <label className="fl">Event</label>
                  <select className="fs" value={newRule.event}
                    onChange={e => setNewRule(r => ({ ...r, event: e.target.value as NotificationRule['event'] }))}>
                    <option value="job_failed">Job failed</option>
                    <option value="job_completed">Job completed</option>
                    <option value="job_started">Job started</option>
                    <option value="conflict_detected">Conflict detected</option>
                  </select>
                </div>
                <Button variant="primary" disabled={addRule.isPending || !newRule.target_id}
                  onClick={() => addRule.mutate()}>
                  <Plus size={13}/> {addRule.isPending ? 'Adding…' : 'Add rule'}
                </Button>
              </div>
            </div>
          )}

          <Modal
            open={!!delNotifTarget}
            title="Delete notification target?"
            body={`This will permanently remove "${delNotifTarget?.name}" and all its rules.`}
            confirmLabel="Delete"
            confirmVariant="danger"
            onConfirm={() => delNotifTarget && delTarget2.mutate(delNotifTarget.id)}
            onClose={() => setDelNotifTarget(null)}
          />
        </div>
      )}

      {tab === 'notifications' && me?.role !== 'admin' && (
        <div className="ssec">
          <div className="text3" style={{ fontSize: 13 }}>Notification settings are only available to admins.</div>
        </div>
      )}

      {/* ── About ────────────────────────────────────────── */}
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

      {/* ── Edit user modal ───────────────────────────────── */}
      {editTarget && (
        <div className="overlay open" onClick={() => setEditTarget(null)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <div className="modal-title">Edit user — {editTarget.user.username}</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 14, marginBottom: 24 }}>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">Role</label>
                <select
                  className="fs"
                  value={editTarget.role}
                  onChange={e => setEditTarget(t => t && ({ ...t, role: e.target.value as User['role'] }))}
                >
                  <option value="admin">Admin</option>
                  <option value="operator">Operator</option>
                  <option value="viewer">Viewer</option>
                </select>
              </div>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">New password <span className="text3">(leave blank to keep current)</span></label>
                <input
                  className="fi"
                  type="password"
                  placeholder="Enter new password…"
                  value={editTarget.password}
                  onChange={e => setEditTarget(t => t && ({ ...t, password: e.target.value }))}
                  autoComplete="new-password"
                />
              </div>
            </div>
            <div className="modal-acts">
              <Button variant="ghost" onClick={() => setEditTarget(null)}>Cancel</Button>
              <Button variant="primary" onClick={() => editUser.mutate()} disabled={editUser.isPending}>
                {editUser.isPending ? 'Saving…' : 'Save changes'}
              </Button>
            </div>
          </div>
        </div>
      )}

      {/* ── Delete user modal ─────────────────────────────── */}
      <Modal
        open={!!delTarget}
        title="Delete user?"
        body={`This will permanently delete the account for "${delTarget?.username}". This cannot be undone.`}
        confirmLabel="Delete"
        confirmVariant="danger"
        onConfirm={() => delTarget && delUser.mutate(delTarget.id)}
        onClose={() => setDelTarget(null)}
      />
    </div>
  )
}
