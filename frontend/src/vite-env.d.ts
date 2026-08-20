/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Signed Telegram initData used only when running outside Telegram in dev. */
  readonly VITE_DEV_TOKEN?: string
  /** Public URL of the API when the SPA is hosted on a different origin. */
  readonly VITE_API_BASE?: string
  /** Bot that hosts the mini app, without the @. Enables deep-link invites. */
  readonly VITE_BOT_USERNAME?: string
  /** Mini app short name. Omit when the app is the bot's main mini app. */
  readonly VITE_MINIAPP_NAME?: string
}