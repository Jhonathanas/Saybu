export type Role = 'customer' | 'admin'

export interface User {
  id: string
  name: string
  phone: string
  email: string
  role: Role
  created_at?: string
}

export interface AuthData {
  user: User
  access_token: string
  expires_in: number
}

export interface LoginInput {
  identifier: string
  password: string
}

export interface RegisterInput {
  name: string
  phone: string
  email?: string
  password: string
}

export interface ListUsersMeta {
  page: number
  limit: number
  total: number
  total_pages: number
}
