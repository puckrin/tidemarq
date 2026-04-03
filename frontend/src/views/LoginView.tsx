import { useState, type FormEvent } from 'react'
import { useAuth } from '../store/auth'
import { Button } from '../components/Button'

interface Props {
  onLogin: () => void
}

export function LoginView({ onLogin }: Props) {
  const { login } = useAuth()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const submit = async (e: FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    try {
      await login(username, password)
      onLogin()
    } catch {
      setError('Invalid username or password.')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div style={{
      display: 'flex', alignItems: 'center', justifyContent: 'center',
      height: '100%', background: 'var(--bg)',
    }}>
      <div style={{ width: 360 }}>
        {/* Logo */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 14, marginBottom: 32, justifyContent: 'center' }}>
          <svg width="40" height="40" viewBox="0 0 36 36" fill="none">
            <rect x="0" y="27" width="36" height="4" rx="2" fill="#E0F4F7" opacity="0.28"/>
            <rect x="0" y="19" width="27" height="4" rx="2" fill="#E0F4F7" opacity="0.50"/>
            <rect x="0" y="11" width="18" height="4" rx="2" fill="#E0F4F7" opacity="0.75"/>
            <rect x="0" y="3"  width="10" height="4" rx="2" fill="#E0F4F7" opacity="1"/>
          </svg>
          <div>
            <div style={{ fontSize: 22, fontWeight: 700, color: 'var(--ghost-teal)', letterSpacing: -0.3 }}>tidemarq</div>
            <div style={{ fontSize: 11, color: 'var(--light-teal)' }}>keep your directories in line</div>
          </div>
        </div>

        <div className="card" style={{ padding: 28 }}>
          <div style={{ fontSize: 17, fontWeight: 600, marginBottom: 20 }}>Sign in</div>
          <form onSubmit={submit}>
            <div className="fg">
              <label className="fl">Username</label>
              <input
                className="fi"
                type="text"
                autoComplete="username"
                value={username}
                onChange={e => setUsername(e.target.value)}
                required
              />
            </div>
            <div className="fg">
              <label className="fl">Password</label>
              <input
                className="fi"
                type="password"
                autoComplete="current-password"
                value={password}
                onChange={e => setPassword(e.target.value)}
                required
              />
            </div>
            {error && (
              <div style={{ color: 'var(--coral-light)', fontSize: 13, marginBottom: 12 }}>{error}</div>
            )}
            <Button variant="primary" style={{ width: '100%', justifyContent: 'center' }} disabled={loading}>
              {loading ? 'Signing in…' : 'Sign in'}
            </Button>
          </form>
        </div>
      </div>
    </div>
  )
}
