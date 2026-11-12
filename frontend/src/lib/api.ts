// Klien API tunggal. Semua panggilan backend lewat sini agar penanganan
// error dan refresh 401 terpusat (lihat AGENTS.md §8).
//
// Access token disimpan di memori (tidak di localStorage); refresh token
// berada di cookie HttpOnly yang dikirim otomatis oleh browser.

import { API_BASE_URL } from './env'

export interface ApiErrorDetail {
  field: string
  message: string
}

export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly details: ApiErrorDetail[]
  readonly requestId?: string

  constructor(
    status: number,
    code: string,
    message: string,
    details: ApiErrorDetail[] = [],
    requestId?: string,
  ) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
    this.details = details
    this.requestId = requestId
  }

  /** True bila error karena masalah jaringan (bukan respons server). */
  get isNetworkError(): boolean {
    return this.status === 0
  }
}

interface Envelope<T, M = Record<string, unknown>> {
  data: T
  meta?: M
}

interface ErrorEnvelope {
  error?: {
    code?: string
    message?: string
    details?: ApiErrorDetail[]
    request_id?: string
  }
}

let accessToken: string | null = null

export function setAccessToken(token: string | null): void {
  accessToken = token
}

export function getAccessToken(): string | null {
  return accessToken
}

export interface RequestOptions {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
  body?: unknown
  /** Kirim Authorization: Bearer. Default true bila token tersedia. */
  auth?: boolean
  signal?: AbortSignal
}

/** Callback yang dipanggil setelah refresh berhasil. */
type TokenListener = (token: string | null) => void
const listeners = new Set<TokenListener>()

export function onAccessTokenChange(listener: TokenListener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function emit(token: string | null) {
  accessToken = token
  listeners.forEach((l) => l(token))
}

// Refresh single-flight: banyak request 401 bersamaan hanya memicu satu refresh.
let refreshPromise: Promise<string | null> | null = null

async function refreshAccessToken(): Promise<string | null> {
  if (!refreshPromise) {
    refreshPromise = doRefresh().finally(() => {
      refreshPromise = null
    })
  }
  return refreshPromise
}

async function doRefresh(): Promise<string | null> {
  try {
    const res = await fetch(`${API_BASE_URL}/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
      headers: { Accept: 'application/json' },
    })
    if (!res.ok) {
      emit(null)
      return null
    }
    const body = (await res.json()) as Envelope<{ access_token: string }>
    emit(body.data.access_token)
    return body.data.access_token
  } catch {
    emit(null)
    return null
  }
}

async function parseError(res: Response): Promise<ApiError> {
  let code = 'ERROR'
  let message = 'Terjadi kesalahan. Coba lagi.'
  let details: ApiErrorDetail[] = []
  let requestId: string | undefined

  try {
    const body = (await res.json()) as ErrorEnvelope
    if (body.error) {
      code = body.error.code ?? code
      message = body.error.message ?? message
      details = body.error.details ?? []
      requestId = body.error.request_id
    }
  } catch {
    // Respons bukan JSON; pakai pesan default.
  }
  return new ApiError(res.status, code, message, details, requestId)
}

async function send(path: string, options: RequestOptions): Promise<Response> {
  const headers: Record<string, string> = { Accept: 'application/json' }
  if (options.body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  const withAuth = options.auth !== false
  if (withAuth && accessToken) {
    headers.Authorization = `Bearer ${accessToken}`
  }

  try {
    return await fetch(`${API_BASE_URL}${path}`, {
      method: options.method ?? 'GET',
      headers,
      credentials: 'include',
      body: options.body === undefined ? undefined : JSON.stringify(options.body),
      signal: options.signal,
    })
  } catch {
    throw new ApiError(0, 'NETWORK_ERROR', 'Tidak bisa menghubungi server.')
  }
}

/**
 * request melakukan panggilan API. Bila menerima 401 dan refresh tersedia,
 * refresh dijalankan sekali lalu request diulang.
 */
export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  let res = await send(path, options)

  if (res.status === 401 && options.auth !== false) {
    const token = await refreshAccessToken()
    if (token) {
      res = await send(path, options)
    }
  }

  if (!res.ok) {
    throw await parseError(res)
  }

  if (res.status === 204) {
    return undefined as T
  }
  const body = (await res.json()) as Envelope<T>
  return body.data
}

/** requestWithMeta mengembalikan data + meta (untuk paginasi). */
export async function requestWithMeta<T, M = Record<string, unknown>>(
  path: string,
  options: RequestOptions = {},
): Promise<Envelope<T, M>> {
  let res = await send(path, options)
  if (res.status === 401 && options.auth !== false) {
    const token = await refreshAccessToken()
    if (token) {
      res = await send(path, options)
    }
  }
  if (!res.ok) {
    throw await parseError(res)
  }
  return (await res.json()) as Envelope<T, M>
}

/** refreshSession memaksa refresh (dipakai saat boot untuk memulihkan sesi). */
export function refreshSession(): Promise<string | null> {
  return refreshAccessToken()
}
