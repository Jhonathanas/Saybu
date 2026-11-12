import { request, requestWithMeta, setAccessToken } from '@/lib/api'
import type { AuthData, ListUsersMeta, LoginInput, RegisterInput, User } from './types'

export async function register(input: RegisterInput): Promise<AuthData> {
  const data = await request<AuthData>('/auth/register', {
    method: 'POST',
    body: input,
    auth: false,
  })
  setAccessToken(data.access_token)
  return data
}

export async function login(input: LoginInput): Promise<AuthData> {
  const data = await request<AuthData>('/auth/login', {
    method: 'POST',
    body: input,
    auth: false,
  })
  setAccessToken(data.access_token)
  return data
}

export async function logout(): Promise<void> {
  try {
    await request<void>('/auth/logout', { method: 'POST', auth: false })
  } finally {
    setAccessToken(null)
  }
}

export function me(): Promise<User> {
  return request<User>('/me')
}

export function listUsers(params: { page?: number; limit?: number; q?: string } = {}) {
  const query = new URLSearchParams()
  if (params.page) query.set('page', String(params.page))
  if (params.limit) query.set('limit', String(params.limit))
  if (params.q) query.set('q', params.q)
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return requestWithMeta<User[], ListUsersMeta>(`/admin/users${suffix}`)
}
