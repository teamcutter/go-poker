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
