import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError, getAccessToken, request, setAccessToken } from './api'

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('request', () => {
  beforeEach(() => {
    setAccessToken(null)
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('mengembalikan data dari envelope', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { data: { ok: true } }))
    vi.stubGlobal('fetch', fetchMock)

    const data = await request<{ ok: boolean }>('/version')
    expect(data.ok).toBe(true)
    expect(fetchMock).toHaveBeenCalledTimes(1)
  })

  it('mengirim header Authorization bila ada token', async () => {
    setAccessToken('token-abc')
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse(200, { data: {} }))
    vi.stubGlobal('fetch', fetchMock)

    await request('/me')
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit]
    const headers = init.headers as Record<string, string>
    expect(headers.Authorization).toBe('Bearer token-abc')
  })

  it('melempar ApiError dengan detail validasi', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        jsonResponse(400, {
          error: {
            code: 'VALIDATION_ERROR',
            message: 'Data tidak valid.',
            details: [{ field: 'email', message: 'format salah' }],
          },
        }),
      ),
    )

    await expect(request('/auth/register', { method: 'POST', body: {} })).rejects.toMatchObject({
      status: 400,
      code: 'VALIDATION_ERROR',
      details: [{ field: 'email', message: 'format salah' }],
    })
  })

  it('refresh sekali lalu ulangi request setelah 401', async () => {
    const fetchMock = vi
      .fn()
      // request pertama -> 401
      .mockResolvedValueOnce(jsonResponse(401, { error: { code: 'UNAUTHORIZED', message: 'x' } }))
      // refresh -> token baru
      .mockResolvedValueOnce(jsonResponse(200, { data: { access_token: 'baru' } }))
      // ulangan request -> sukses
      .mockResolvedValueOnce(jsonResponse(200, { data: { id: 'u-1' } }))
    vi.stubGlobal('fetch', fetchMock)

    const data = await request<{ id: string }>('/me')
    expect(data.id).toBe('u-1')
    expect(getAccessToken()).toBe('baru')
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('tidak mengulang request bila refresh gagal', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(jsonResponse(401, { error: { code: 'UNAUTHORIZED', message: 'x' } }))
      .mockResolvedValueOnce(jsonResponse(401, { error: { code: 'UNAUTHORIZED', message: 'x' } }))
    vi.stubGlobal('fetch', fetchMock)

    await expect(request('/me')).rejects.toBeInstanceOf(ApiError)
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(getAccessToken()).toBeNull()
  })

  it('menandai error jaringan', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('gagal')))
    await expect(request('/version')).rejects.toMatchObject({ status: 0, code: 'NETWORK_ERROR' })
  })

  it('mengembalikan undefined untuk 204', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 204 })))
    await expect(request<void>('/auth/logout', { method: 'POST' })).resolves.toBeUndefined()
  })
})
