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
  const [error, setError]       = useState('')
  const [loading, setLoading]   = useState(false)

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
      width: '100%',
      height: '100%',
      display: 'flex',
      flexDirection: 'column',
      alignItems: 'center',
      justifyContent: 'center',
      background: '#0A4452',   /* always sidebar teal — independent of theme */
    }}>

      {/* Logo + wordmark — flex-start so SVG top = wordmark top; SVG sized to match text block height */}
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

      {/* Card */}
      <div style={{
        width: 360,
        background: '#0d4354',
        border: '1px solid rgba(163,221,230,0.14)',
        borderRadius: 10,
        padding: 28,
        boxShadow: '0 20px 60px rgba(0,0,0,0.4)',
      }}>
        <div style={{ fontSize: 17, fontWeight: 600, color: '#E0F4F7', marginBottom: 22 }}>
          Sign in
        </div>

        <form onSubmit={submit}>
          <div className="fg">
            <label className="fl" style={{ color: '#A3DDE6' }}>Username</label>
            <input
              className="fi"
              type="text"
              autoComplete="username"
              autoFocus
              value={username}
              onChange={e => setUsername(e.target.value)}
              required
            />
          </div>

          <div className="fg">
            <label className="fl" style={{ color: '#A3DDE6' }}>Password</label>
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
            <div style={{ color: '#F28B74', fontSize: 13, marginBottom: 14 }}>
              {error}
            </div>
          )}

          <Button
            variant="primary"
            style={{ width: '100%', justifyContent: 'center' }}
            disabled={loading}
          >
            {loading ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>
      </div>
    </div>
  )
}
