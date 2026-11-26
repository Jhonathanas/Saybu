import { expect, test } from '@playwright/test'
import { mockApi, ORDER_CODE } from './mock-api'

/** tambahBayam menyiapkan keranjang satu item lewat UI katalog. */
async function tambahBayam(page: import('@playwright/test').Page) {
  await page.goto('/produk')
  await page.getByRole('button', { name: 'Tambah Bayam Segar ke keranjang' }).click()
  await page.goto('/keranjang')
  await expect(page.getByText('Bayam Segar')).toBeVisible()
  await page.getByRole('button', { name: 'Checkout' }).click()
  await expect(page.getByRole('heading', { name: 'Checkout' })).toBeVisible()
}

test.describe('alur checkout', () => {
  test('tamu berhasil checkout dan melihat halaman sukses', async ({ page }) => {
    const state = await mockApi(page)
    await tambahBayam(page)

    await page.getByLabel('Nama', { exact: true }).fill('Siti Aminah')
    await page.getByLabel('Nomor HP / WhatsApp').fill('081234567890')
    await page.getByLabel('Zona ongkir').selectOption('z-1')
    await page.getByLabel('Alamat lengkap').fill('Jl. Melati No. 12')
    await page.getByLabel('Kota/Kabupaten').fill('Bandung')

    // Harga dihitung ulang server: 1 ikat Bayam (Rp5.000) + ongkir Rp10.000.
    await expect(page.getByText(/Rp\s*15\.000/)).toBeVisible()

    await page.getByRole('button', { name: 'Buat pesanan' }).click()

    await expect(page).toHaveURL(/\/pesanan\/SB-20261126-0001\?phone=.*baru=1/)
    await expect(page.getByText('Pesanan berhasil dibuat!')).toBeVisible()
    await expect(page.getByRole('heading', { name: 'Detail pesanan' })).toBeVisible()
    await expect(page.getByText(ORDER_CODE, { exact: true }).first()).toBeVisible()
    await expect(page.getByText('Bayam Segar')).toBeVisible()
    await expect(page.getByText('Belum dibayar')).toBeVisible()

    expect(state.createRequests).toHaveLength(1)
    const request = state.createRequests[0]
    expect(request.body.customer.name).toBe('Siti Aminah')
    expect(request.body.fulfillment).toBe('DELIVERY')
    expect(request.body.payment_method).toBe('TRANSFER')
    expect(request.body.items).toEqual([{ product_id: 'p-7', qty: 1 }])
    expect(request.idempotencyKey).toMatch(/^[0-9a-f-]{36}$/)
  })

  test('validasi menahan checkout yang belum lengkap', async ({ page }) => {
    const state = await mockApi(page)
    await tambahBayam(page)

    await page.getByRole('button', { name: 'Buat pesanan' }).click()

    await expect(page.getByText('Nama wajib diisi.')).toBeVisible()
    await expect(page.getByText('Nomor HP minimal 8 digit.')).toBeVisible()
    await expect(page.getByText('Pilih zona ongkir.')).toBeVisible()
    await expect(page.getByText('Alamat wajib untuk pengiriman.')).toBeVisible()
    expect(state.createRequests).toHaveLength(0)
    await expect(page).toHaveURL(/\/checkout$/)
  })

  test('ambil di tempat dengan COD tidak memungut ongkir', async ({ page }) => {
    const state = await mockApi(page)
    await tambahBayam(page)

    await page.getByLabel('Nama', { exact: true }).fill('Budi Santoso')
    await page.getByLabel('Nomor HP / WhatsApp').fill('081234567891')
    await page.getByRole('radio', { name: 'Ambil di tempat' }).check()
    await page.getByRole('radio', { name: /Bayar di tempat \(COD\)/ }).check()

    await expect(page.getByText('Pesanan diambil langsung di toko. Ongkir tidak dikenakan.')).toBeVisible()
    await expect(page.locator('aside').getByText(/Rp\s*5\.000/).first()).toBeVisible()

    await page.getByRole('button', { name: 'Buat pesanan' }).click()

    await expect(page).toHaveURL(/\/pesanan\/SB-20261126-0001/)
    await expect(page.getByText('Ambil di tempat')).toBeVisible()
    await expect(page.getByText('Bayar di tempat', { exact: true })).toBeVisible()

    expect(state.createRequests).toHaveLength(1)
    const request = state.createRequests[0]
    expect(request.body.fulfillment).toBe('PICKUP')
    expect(request.body.payment_method).toBe('COD')
    expect(request.body.zone_id).toBeUndefined()
  })
})

test('lacak pesanan dengan kode dan nomor HP', async ({ page }) => {
  await mockApi(page, { existingOrder: true })

  await page.goto('/pesanan')
  await page.getByLabel('Kode pesanan').fill(ORDER_CODE)
  await page.getByLabel('Nomor HP').fill('081234567890')
  await page.getByRole('button', { name: 'Lacak pesanan' }).click()

  await expect(page).toHaveURL(/\/pesanan\/SB-20261126-0001\?phone=/)
  await expect(page.getByRole('heading', { name: 'Detail pesanan' })).toBeVisible()
  await expect(page.getByText('Menunggu Pembayaran').first()).toBeVisible()
  await expect(page.getByText('Siti Aminah · 081234567890')).toBeVisible()
})
