import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useParams, useSearchParams } from 'react-router'
import { describe, expect, it } from 'vitest'
import { OrderTrackingPage } from './OrderTrackingPage'

function DetailStub() {
  const { code } = useParams()
  const [params] = useSearchParams()
  return (
    <div>
      Detail {code} · {params.get('phone')}
    </div>
  )
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={['/pesanan']}>
      <Routes>
        <Route path="/pesanan" element={<OrderTrackingPage />} />
        <Route path="/pesanan/:code" element={<DetailStub />} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('OrderTrackingPage', () => {
  it('menolak submit kosong', async () => {
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Lacak pesanan' }))
    expect(await screen.findByText('Isi kode pesanan.')).toBeInTheDocument()
  })

  it('mengarahkan ke detail dengan kode & nomor HP', async () => {
    renderPage()
    await userEvent.type(screen.getByLabelText('Kode pesanan'), 'sb-20261114-0001')
    await userEvent.type(screen.getByLabelText('Nomor HP'), '081234567890')
    await userEvent.click(screen.getByRole('button', { name: 'Lacak pesanan' }))

    expect(
      await screen.findByText('Detail SB-20261114-0001 · 081234567890'),
    ).toBeInTheDocument()
  })
})
