// Provider dan hook auth sengaja satu berkas agar konteks tidak terekspos keluar.
/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { onAccessTokenChange, refreshSession, setAccessToken } from '@/lib/api'
import * as authApi from './api'
import type { LoginInput, RegisterInput, User } from './types'

interface AuthContextValue {
  user: User | null
  /** True selama pemulihan sesi awal (boot). */
  loading: boolean
  login: (input: LoginInput) => Promise<User>
  register: (input: RegisterInput) => Promise<User>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextValue | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)

  // Bila token hilang (mis. refresh gagal), bersihkan user.
  useEffect(() => onAccessTokenChange((token) => {
    if (!token) setUser(null)
  }), [])

  // Pulihkan sesi saat aplikasi dibuka: refresh lalu ambil profil.
  useEffect(() => {
    let active = true
    void (async () => {
      const token = await refreshSession()
      if (!active) return
      if (token) {
        try {
          const profile = await authApi.me()
          if (active) setUser(profile)
        } catch {
          setAccessToken(null)
        }
      }
      if (active) setLoading(false)
    })()
    return () => {
      active = false
    }
  }, [])

  const login = useCallback(async (input: LoginInput) => {
    const data = await authApi.login(input)
    setUser(data.user)
    return data.user
  }, [])

  const register = useCallback(async (input: RegisterInput) => {
    const data = await authApi.register(input)
    setUser(data.user)
    return data.user
  }, [])

  const logout = useCallback(async () => {
    await authApi.logout()
    setUser(null)
  }, [])

  const refreshUser = useCallback(async () => {
    setUser(await authApi.me())
  }, [])

  const value = useMemo<AuthContextValue>(
    () => ({ user, loading, login, register, logout, refreshUser }),
    [user, loading, login, register, logout, refreshUser],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth harus dipakai di dalam <AuthProvider>')
  }
  return ctx
}
