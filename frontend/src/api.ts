import { getInitData } from './telegram'
import type { LoginResult, TableDTO } from './types'

const TOKEN_KEY = 'gopoker.token'
// Empty means same-origin (local dev via the Vite proxy, or nginx in compose).
// Set to the API's public URL when the SPA is hosted separately, e.g. on Vercel
// — WebSocket upgrades cannot be proxied through Vercel rewrites, so the socket
// has to address the backend directly.
const API_BASE = (import.meta.env.VITE_API_BASE ?? '').replace(/\/$/, '')

export function getToken(): string | null {
  return localStorage.getItem(TOKEN_KEY)
}

export function setToken(token: string): void {
  localStorage.setItem(TOKEN_KEY, token)
}

export function clearToken(): void {
  localStorage.removeItem(TOKEN_KEY)
}

export function getInitDataForLogin(): string {
  const data = getInitData()
  if (!data) {
    throw new Error('Telegram init data is not available')
  }
  return data
}

export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

async function request<T>(path: string, options: RequestInit = {}): Promise<T> {
  const token = getToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) {
    headers.Authorization = `Bearer ${token}`
  }

  let res: Response
  try {
    res = await fetch(API_BASE + path, { ...options, headers })
  } catch {
    throw new ApiError(0, 'network', 'Cannot reach the server')
  }

  const body = await res.json().catch(() => null)
  if (!res.ok) {
    const err = body?.error ?? {}
    throw new ApiError(res.status, err.code ?? 'unknown', err.message ?? 'Request failed')
  }
  return body?.data as T
}

function post<T>(path: string, payload?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: payload === undefined ? undefined : JSON.stringify(payload),
  })
}

export function loginTelegram(initData: string): Promise<LoginResult> {
  return post<LoginResult>('/api/auth/telegram', { init_data: initData })
}

export function createTable(maxSeats: number, bigBlind: number): Promise<{ table: TableDTO }> {
  return post<{ table: TableDTO }>('/api/tables', { max_seats: maxSeats, big_blind: bigBlind })
}

export function listTables(): Promise<TableDTO[]> {
  return request<TableDTO[]>('/api/tables')
}


export function joinTable(code: string, buyIn: number): Promise<{ joined: boolean; bankroll: number }> {
  return post<{ joined: boolean; bankroll: number }>(`/api/tables/${code}/join`, { buy_in: buyIn })
}

export function leaveTable(code: string): Promise<{ left: boolean; bankroll: number }> {
  return post<{ left: boolean; bankroll: number }>(`/api/tables/${code}/leave`)
}

export function getWallet(): Promise<{ bankroll: number }> {
  return request<{ bankroll: number }>('/api/poker/wallet')
}

export function topUpWallet(): Promise<{ bankroll: number }> {
  return post<{ bankroll: number }>('/api/poker/wallet/topup')
}

export function startHand(code: string): Promise<{ started: boolean }> {
  return post<{ started: boolean }>(`/api/tables/${code}/start`)
}

export function wsTableUrl(code: string): string {
  const token = getToken()
  const q = token ? `?token=${encodeURIComponent(token)}` : ''
  const origin = API_BASE || window.location.origin
  const wsOrigin = origin.replace(/^http/, 'ws')
  return `${wsOrigin}/api/ws/tables/${code}${q}`
}