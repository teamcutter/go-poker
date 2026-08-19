import { useState } from 'react'
import { useApp } from '../store'
import { hapticNotify, hasDevFallback, isInsideTelegram } from '../telegram'
import Button from '../components/Button'

export default function LoginScreen() {
  const { login } = useApp()
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const canSignIn = isInsideTelegram() || hasDevFallback()

  const handleLogin = async () => {
    setBusy(true)
    setError(null)
    try {
      await login()
    } catch (err) {
      hapticNotify('error')
      setError(err instanceof Error ? err.message : 'Login failed')
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="center login">
      <div className="login-mark">♠</div>
      <h1 className="login-title">GoPoker</h1>
      <p className="login-sub">
        Texas Hold'em with your friends. No ads, no pop-ups, no fake felt — just the game.
      </p>

      {error && (
        <div className="error-banner" style={{ width: '100%', maxWidth: 320 }}>
          {error}
        </div>
      )}

      <div className="login-actions">
        <Button size="lg" block onClick={handleLogin} disabled={busy || !canSignIn}>
          {busy ? 'Signing in…' : 'Play now'}
        </Button>
      </div>

      {!canSignIn && (
        <p className="login-note">
          Open this inside Telegram, or set <code>VITE_DEV_TOKEN</code> to test in a browser.
        </p>
      )}

      <div className="login-features">
        <span>Private tables</span>
        <span>·</span>
        <span>Free chips</span>
        <span>·</span>
        <span>Instant seats</span>
      </div>
    </main>
  )
}
