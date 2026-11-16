import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/lib/api'
import { ProofUploadForm } from './ProofUploadForm'
import type { Payment } from './types'
import * as api from './api'

vi.mock('./api', () => ({
  uploadPaymentProof: vi.fn(),
}))

const mocked = vi.mocked(api)

const PAYMENT: Payment = { method: 'TRANSFER', status: 'WAITING_VERIFICATION', amount: 98000 }

function renderForm(onUploaded = vi.fn()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  render(
    <QueryClientProvider client={queryClient}>
      <ProofUploadForm code="SB-20261114-0001" phone="081234567890" onUploaded={onUploaded} />
    </QueryClientProvider>,
  )
  return { onUploaded }
}

function file(name: string, type: string, size = 1024): File {
  return new File([new Uint8Array(size)], name, { type })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('ProofUploadForm', () => {
  it('menolak berkas dengan ekstensi tidak diizinkan', async () => {
    renderForm()
    // Pakai fireEvent agar validasi ekstensi di JS diuji (userEvent menolak
    // berkas yang tidak cocok dengan atribut `accept`).
    fireEvent.change(screen.getByLabelText('Unggah bukti transfer'), {
      target: { files: [file('bukti.gif', 'image/gif')] },
    })
    expect(await screen.findByText('Format berkas harus JPG, PNG, atau PDF.')).toBeInTheDocument()
    expect(mocked.uploadPaymentProof).not.toHaveBeenCalled()
  })

  it('menolak berkas melebihi 5 MB', async () => {
    renderForm()
    await userEvent.upload(
      screen.getByLabelText('Unggah bukti transfer'),
      file('bukti.jpg', 'image/jpeg', 6 * 1024 * 1024),
    )
    expect(await screen.findByText('Ukuran berkas melebihi 5 MB.')).toBeInTheDocument()
    expect(mocked.uploadPaymentProof).not.toHaveBeenCalled()
  })

  it('mengirim berkas valid dan memanggil onUploaded', async () => {
    mocked.uploadPaymentProof.mockResolvedValue(PAYMENT)
    const { onUploaded } = renderForm()

    await userEvent.upload(
      screen.getByLabelText('Unggah bukti transfer'),
      file('bukti.jpg', 'image/jpeg'),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Kirim bukti' }))

    await waitFor(() => expect(mocked.uploadPaymentProof).toHaveBeenCalledTimes(1))
    expect(mocked.uploadPaymentProof.mock.calls[0][0]).toBe('SB-20261114-0001')
    expect(mocked.uploadPaymentProof.mock.calls[0][2]).toBe('081234567890')
    await waitFor(() => expect(onUploaded).toHaveBeenCalledWith(PAYMENT))
  })

  it('menampilkan pesan error dari server', async () => {
    mocked.uploadPaymentProof.mockRejectedValue(
      new ApiError(409, 'CONFLICT', 'Bukti tidak bisa diunggah pada status ini.'),
    )
    renderForm()

    await userEvent.upload(
      screen.getByLabelText('Unggah bukti transfer'),
      file('bukti.png', 'image/png'),
    )
    await userEvent.click(screen.getByRole('button', { name: 'Kirim bukti' }))

    expect(
      await screen.findByText('Bukti tidak bisa diunggah pada status ini.'),
    ).toBeInTheDocument()
  })
})
