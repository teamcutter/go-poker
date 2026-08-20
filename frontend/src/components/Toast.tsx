import { useEffect, useState } from 'react'

interface ToastProps {
  /** Message to show; null dismisses it. */
  message: string | null
  /** Must be stable (useCallback) — it drives the auto-dismiss timer. */
  onDismiss: () => void
  /** Milliseconds on screen before it dismisses itself. */
  duration?: number
}

export default function Toast({ message, onDismiss, duration = 4000 }: ToastProps) {
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
    <div className="toast-layer" role="alert" aria-live="assertive">
      <button
        className={`toast${message ? '' : ' toast-leaving'}`}
        onClick={onDismiss}
        // The node outlives `message` so the exit can play; drop it once that
        // animation finishes. The target check ignores the icon's own strokes
        // animating and bubbling up from inside.
        onAnimationEnd={(e) => {
          if (e.target === e.currentTarget && !message) setShown('')
        }}
      >
        {/* Drawn rather than faded in: the ring sweeps closed, then the stem
            and dot land. Telegram animates its icons instead of its boxes. */}
        <svg className="toast-icon" viewBox="0 0 24 24" aria-hidden="true">
          <circle className="toast-icon-ring" cx="12" cy="12" r="10" />
          <line className="toast-icon-stem" x1="12" y1="6.8" x2="12" y2="13.4" />
          <circle className="toast-icon-dot" cx="12" cy="16.7" r="1.15" />
        </svg>
        <span className="toast-text">{shown}</span>
      </button>
    </div>
  )
}
