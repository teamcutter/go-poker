export interface TelegramHaptics {
  impactOccurred?(style: 'light' | 'medium' | 'heavy' | 'rigid' | 'soft'): void
  notificationOccurred?(type: 'error' | 'success' | 'warning'): void
  selectionChanged?(): void
}

export interface TelegramWebApp {
  initData: string
  initDataUnsafe?: {
    start_param?: string
    user?: { first_name?: string; username?: string }
  }
  HapticFeedback?: TelegramHaptics
  ready(): void
  expand(): void
  setHeaderColor?(color: string): void
  setBackgroundColor?(color: string): void
  enableClosingConfirmation?(): void
  /** Opens a t.me URL inside Telegram rather than punting to a browser. */
  openTelegramLink?(url: string): void
}

declare global {
  interface Window {
    Telegram?: { WebApp: TelegramWebApp }
  }
}

const SHELL_COLOR = '#0a0b0d'

// Browser-testing fallback, dev only. `import.meta.env.DEV` is statically
// replaced with `false` by `vite build`, so the token is dead code the minifier
// drops — it never reaches a production bundle.
const DEV_INIT_DATA: string = import.meta.env.DEV ? (import.meta.env.VITE_DEV_TOKEN ?? '') : ''

export function hasDevFallback(): boolean {
  return DEV_INIT_DATA !== ''
}

export function isInsideTelegram(): boolean {
  return Boolean(window.Telegram?.WebApp?.initData)
}

export function hasRealInitData(): boolean {
  return Boolean(window.Telegram?.WebApp?.initData)
}

export function getInitData(): string {
  const app = window.Telegram?.WebApp
  if (app?.initData) {
    app.ready()
    app.expand()
    return app.initData
  }
  return DEV_INIT_DATA
}

export function getStartParam(): string {
  return window.Telegram?.WebApp?.initDataUnsafe?.start_param ?? ''
}

// Who to address the deep link to. Both are inlined at build time, so changing
// them needs a rebuild. Without a bot username there is no link to build at all,
// and every invite quietly degrades to sharing the bare table code.
const BOT_USERNAME = (import.meta.env.VITE_BOT_USERNAME ?? '').replace(/^@/, '').trim()
const MINIAPP_NAME = (import.meta.env.VITE_MINIAPP_NAME ?? '').trim()

/**
 * A t.me link that opens the mini app straight into `code`. Telegram hands the
 * value back as `start_param`, which App.tsx feeds to the lobby as `autoJoin` —
 * so the recipient lands in the seat instead of retyping a code.
 *
 * Empty when no bot username is configured; callers fall back to the raw code.
 */
export function inviteLink(code: string): string {
  if (!BOT_USERNAME) return ''
  // Table codes are drawn from an alphanumeric alphabet, which is exactly what
  // startapp permits, so no escaping is needed.
  const target = MINIAPP_NAME ? `${BOT_USERNAME}/${MINIAPP_NAME}` : BOT_USERNAME
  return `https://t.me/${target}?startapp=${code}`
}

/**
 * Hands `code` to Telegram's own "forward to…" picker. Returns false when that
 * is not possible — outside Telegram, or with no bot configured — so the caller
 * can fall back to the clipboard.
 */
export function shareInvite(code: string, text: string): boolean {
  const app = window.Telegram?.WebApp
  const link = inviteLink(code)
  if (!app?.openTelegramLink || !link) return false
  const url = `https://t.me/share/url?url=${encodeURIComponent(link)}&text=${encodeURIComponent(text)}`
  app.openTelegramLink(url)
  return true
}

export function getTelegramName(): string | null {
  const tgUser = window.Telegram?.WebApp?.initDataUnsafe?.user
  return tgUser?.first_name ?? tgUser?.username ?? null
}

export function applyTheme(): void {
  const app = window.Telegram?.WebApp
  if (!app) return
  app.ready()
  app.expand()
  app.setHeaderColor?.(SHELL_COLOR)
  app.setBackgroundColor?.(SHELL_COLOR)
}

export function haptic(style: 'light' | 'medium' | 'heavy' = 'light'): void {
  window.Telegram?.WebApp?.HapticFeedback?.impactOccurred?.(style)
}

export function hapticNotify(type: 'error' | 'success' | 'warning'): void {
  window.Telegram?.WebApp?.HapticFeedback?.notificationOccurred?.(type)
}
