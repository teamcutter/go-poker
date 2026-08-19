import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import * as api from './api'
import { getInitData } from './telegram'
import type { UserDTO } from './types'

export interface AppState {
  token: string | null
  user: UserDTO | null
  booting: boolean
  login: () => Promise<void>
  logout: () => void
  setUser: (user: UserDTO) => void
}

const Ctx = createContext<AppState | null>(null)

export function useApp(): AppState {
  const value = useContext(Ctx)
  if (!value) {
    throw new Error('useApp must be used inside AppProvider')
  }
  return value
}

export function AppProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(api.getToken())
  const [user, setUser] = useState<UserDTO | null>(null)
  const [booting, setBooting] = useState(true)

  const applyLogin = useCallback(async (initData: string) => {
    const res = await api.loginTelegram(initData)
    api.setToken(res.token)
    setToken(res.token)
    setUser(res.user)
  }, [])

  useEffect(() => {
    const doBoot = async () => {
      const initData = getInitData()
      if (initData) {
        try {
          await applyLogin(initData)
        } catch {
          api.clearToken()
          setToken(null)
        }
      }
      setBooting(false)
    }
    void doBoot()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const login = async () => {
    const initData = api.getInitDataForLogin()
    await applyLogin(initData)
  }

  const logout = () => {
    api.clearToken()
    setToken(null)
    setUser(null)
  }

  const value: AppState = { token, user, booting, login, logout, setUser }

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}