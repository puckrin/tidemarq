import { useState, type FormEvent } from 'react'
import { useAuth } from '../store/auth'
import { Button } from '../components/Button'
import { ApiError } from '../api/client'

// Forced password-change screen. Shown immediately after login when the
// backend's JWT carries pwd_change_required=true (currently only the seeded
// default admin). No navigation, no header, no back-link — the user cannot
// reach any other view until they submit a valid new password.
export function ChangePasswordView() {
  const { user, changePassword, logout } = useAuth()
  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirm, setConfirm]               = useState('')
  const [error, setError]                   = useState('')
  const [loading, setLoading]               = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')

    if (newPassword.length < 8) {
      setError('New password must be at least 8 characters.')
      return
    }
    if (newPassword !== confirm) {
      setError('New password and confirmation do not match.')
      return
    }
    if (newPassword === currentPassword) {
      setError('New password must differ from the current password.')
      return
    }

    setLoading(true)
    try {
      await changePassword(currentPassword, newPassword)
      // On success the AuthProvider replaces the token and parses a new user
      // with passwordChangeRequired=false; App.tsx will re-render into the
      // dashboard automatically.
    } catch (err) {
      if (err instanceof ApiError && err.code === 'invalid_current_password') {
        setError('Current password is incorrect.')
      } else if (err instanceof ApiError) {
        setError(err.message)
      } else {
        setError('Could not change password. Please try again.')
      }
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      background: '#0A4452',
    }}>
      {/* Logo + wordmark — matches LoginView so the visual continuity is preserved. */}
      <div style={{ display: 'flex', alignItems: 'flex-start', gap: 14, marginBottom: 32 }}>
        <svg width="55" height="55" viewBox="0 0 36 36" fill="none" style={{ flexShrink: 0 }}>
          <rect x="0" y="27" width="36" height="4" rx="2" fill="#E0F4F7" opacity="0.28"/>
          <rect x="0" y="19" width="27" height="4" rx="2" fill="#E0F4F7" opacity="0.50"/>
          <rect x="0" y="11" width="18" height="4" rx="2" fill="#E0F4F7" opacity="0.75"/>
          <rect x="0" y="3"  width="10" height="4" rx="2" fill="#E0F4F7" opacity="1"/>
        </svg>
        <div>
          <div style={{ fontSize: 26, fontWeight: 700, color: '#E0F4F7', letterSpacing: -0.5, lineHeight: 1.2 }}>
            tidemarq
          </div>
          <div style={{ fontSize: 11, color: '#5DC4D4', marginTop: 3 }}>
            keep your directories in line
          </div>
        </div>
      </div>

      <div style={{
        width: 360,
        background: '#0d4354',
        border: '1px solid rgba(163,221,230,0.14)',
        borderRadius: 10,
        padding: 28,
        boxShadow: '0 20px 60px rgba(0,0,0,0.4)',
      }}>
        <div style={{ fontSize: 17, fontWeight: 600, color: '#E0F4F7', marginBottom: 8 }}>
          Set a new password
        </div>
        <div style={{ fontSize: 13, color: '#A3DDE6', marginBottom: 22, lineHeight: 1.45 }}>
          The default password must be changed before <strong>{user?.username ?? 'this account'}</strong> can access tidemarq.
        </div>

        <form onSubmit={submit}>
          <div className="fg">
            <label htmlFor="cp-current" className="fl" style={{ color: '#A3DDE6' }}>Current password</label>
            <input
              id="cp-current"
              className="fi"
              type="password"
              autoComplete="current-password"
              autoFocus
              value={currentPassword}
              onChange={e => setCurrentPassword(e.target.value)}
              required
            />
          </div>

          <div className="fg">
            <label htmlFor="cp-new" className="fl" style={{ color: '#A3DDE6' }}>New password</label>
            <input
              id="cp-new"
              className="fi"
              type="password"
              autoComplete="new-password"
              value={newPassword}
              onChange={e => setNewPassword(e.target.value)}
              required
            />
          </div>

          <div className="fg">
            <label htmlFor="cp-confirm" className="fl" style={{ color: '#A3DDE6' }}>Confirm new password</label>
            <input
              id="cp-confirm"
              className="fi"
              type="password"
              autoComplete="new-password"
              value={confirm}
              onChange={e => setConfirm(e.target.value)}
              required
            />
          </div>

          {error && (
            <div style={{ color: '#F28B74', fontSize: 13, marginBottom: 14 }}>
              {error}
            </div>
          )}

          <Button
            variant="primary"
            style={{ width: '100%', justifyContent: 'center' }}
            disabled={loading}
          >
            {loading ? 'Updating…' : 'Set new password'}
          </Button>

          {/* Escape hatch: a user who reaches this screen by mistake (e.g. wrong
              username on a shared machine) can sign out and try again. */}
          <button
            type="button"
            onClick={logout}
            style={{
              width: '100%',
              marginTop: 14,
              background: 'transparent',
              border: 'none',
              color: '#A3DDE6',
              fontSize: 12,
              cursor: 'pointer',
              padding: 4,
            }}
          >
            Sign out and use a different account
          </button>
        </form>
      </div>
    </div>
  )
}
