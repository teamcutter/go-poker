/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Signed Telegram initData used only when running outside Telegram in dev. */
  readonly VITE_DEV_TOKEN?: string
  /** Public URL of the API when the SPA is hosted on a different origin. */
  readonly VITE_API_BASE?: string
}