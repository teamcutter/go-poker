import { useEffect, useState } from 'react'
import type { CSSProperties } from 'react'

interface ToastProps {
  /** Message to show; null dismisses it. */
  message: string | null
  /** Must be stable (useCallback) — it drives the auto-dismiss timer. */
  onDismiss: () => void
  /** Milliseconds on screen before it dismisses itself. */
  duration?: number
}

export default function Toast({ message, onDismiss, duration = 4500 }: ToastProps) {
  // Held apart from `message` so the text survives the exit animation, which
  // plays after the owner has already cleared it.
  const [shown, setShown] = useState('')

  // Adjusted during render rather than in an effect: React's documented way to
  // derive state from a prop, and it puts the text on screen in the same pass
  // instead of one frame late, so the entry animation never plays on stale text.
  if (message && message !== shown) {
    setShown(message)
  }

  useEffect(() => {
    if (!message) return
    const timer = setTimeout(onDismiss, duration)
    return () => clearTimeout(timer)
  }, [message, duration, onDismiss])

  if (!shown) return null

  return (
    <div className="toast-layer">
      <div
        className={`toast${message ? '' : ' toast-leaving'}`}
        role="alert"
        style={{ '--toast-ms': `${duration}ms` } as CSSProperties}
        // The node outlives `message` so the exit can play; drop it once that
        // animation finishes. The target check ignores the countdown bar's own
        // animation bubbling up from inside.
        onAnimationEnd={(e) => {
          if (e.target === e.currentTarget && !message) setShown('')
        }}
      >
        <span className="toast-mark">!</span>
        <span className="toast-text">{shown}</span>
        <button className="toast-close" onClick={onDismiss} aria-label="Dismiss">
          ✕
        </button>
        <span className="toast-timer" />
      </div>
    </div>
  )
}
