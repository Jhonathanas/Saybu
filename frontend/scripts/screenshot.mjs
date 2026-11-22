// Ambil screenshot UI Saybu memakai data contoh (tanpa backend).
// Jalankan: npm run dev  →  npm run screenshot
import { mkdir } from 'node:fs/promises'
import { resolve } from 'node:path'
import { chromium } from '@playwright/test'

const baseURL = process.env.BASE_URL ?? 'http://127.0.0.1:5173'
const outDir = resolve(process.cwd(), '../ui-app')

const categories = [
  { id: 'c-1', name: 'Buah', slug: 'buah', sort: 1 },
  { id: 'c-2', name: 'Sayur', slug: 'sayur', sort: 2 },
]

// Sama dengan gambar contoh di backend/cmd/seed/main.go.
const imageBySlug = {
  'pisang-cavendish': '/images/produk-pisang.webp',
  'jeruk-baby-pacitan': '/images/kategori-buah.webp',
  'apel-fuji': '/images/produk-apel.webp',
  'semangka-merah': '/images/kategori-buah.webp',
  'mangga-harum-manis': '/images/kategori-buah.webp',
  'alpukat-mentega': '/images/kategori-buah.webp',
  'bayam-segar': '/images/produk-bayam.webp',
  'kangkung': '/images/kategori-sayur.webp',
  'tomat-merah': '/images/produk-tomat.webp',
  'wortel-brastagi': '/images/kategori-sayur.webp',
  'kentang-dieng': '/images/kategori-sayur.webp',
  'paket-sayur-sop': '/images/produk-default.webp',
}

const seed = [
  ['Buah', 'buah', 'Pisang Cavendish', 'pisang-cavendish', 'kg', 18000, 25000, 250],
  ['Buah', 'buah', 'Jeruk Baby Pacitan', 'jeruk-baby-pacitan', 'kg', 25000, 20000, 250],
  ['Buah', 'buah', 'Apel Fuji', 'apel-fuji', 'kg', 32000, 15000, 250],
  ['Buah', 'buah', 'Semangka Merah', 'semangka-merah', 'buah', 35000, 20, 1],
  ['Buah', 'buah', 'Mangga Harum Manis', 'mangga-harum-manis', 'kg', 28000, 12000, 250],
  ['Buah', 'buah', 'Alpukat Mentega', 'alpukat-mentega', 'buah', 12000, 30, 1],
  ['Sayur', 'sayur', 'Bayam Segar', 'bayam-segar', 'ikat', 5000, 40, 1],
  ['Sayur', 'sayur', 'Kangkung', 'kangkung', 'ikat', 4000, 35, 1],
  ['Sayur', 'sayur', 'Tomat Merah', 'tomat-merah', 'kg', 14000, 18000, 250],
  ['Sayur', 'sayur', 'Wortel Brastagi', 'wortel-brastagi', 'kg', 16000, 20000, 250],
  ['Sayur', 'sayur', 'Kentang Dieng', 'kentang-dieng', 'kg', 15000, 22000, 250],
  ['Sayur', 'sayur', 'Paket Sayur Sop', 'paket-sayur-sop', 'paket', 15000, 25, 1],
]

const products = seed.map(([categoryName, categorySlug, name, slug, unit, price, stock, step], i) => {
  const category = categories.find((c) => c.slug === categorySlug)
  const image = imageBySlug[slug] ?? '/images/produk-default.webp'
  return {
    id: `p-${i + 1}`,
    slug,
    name,
    category,
    description: `${name} segar, dipanen pagi ini dan dikirim langsung ke rumah Anda.`,
    unit,
    price,
    stock,
    min_qty: step,
    step_qty: step,
    is_available: true,
    images: [image],
    created_at: '2026-11-18T01:00:00Z',
  }
})

async function mockApi(page) {
  await page.route('**/api/v1/**', async (route) => {
    const url = new URL(route.request().url())
    const path = url.pathname

    if (path.startsWith('/api/v1/auth/')) {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ error: { code: 'UNAUTHORIZED', message: 'Belum masuk' } }),
      })
      return
    }

    if (path === '/api/v1/categories') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ data: categories }),
      })
      return
    }

    if (path === '/api/v1/products') {
      const category = url.searchParams.get('category')
      const q = url.searchParams.get('q')?.toLowerCase() ?? ''
      const sort = url.searchParams.get('sort') ?? 'newest'
      const pageNo = Number(url.searchParams.get('page') ?? '1') || 1
      const limit = Number(url.searchParams.get('limit') ?? '12') || 12

      let items = products.filter((p) => {
        if (category && p.category?.slug !== category) return false
        if (q && !p.name.toLowerCase().includes(q)) return false
        return true
      })
      if (sort === 'price_asc') items = [...items].sort((a, b) => a.price - b.price)
      if (sort === 'price_desc') items = [...items].sort((a, b) => b.price - a.price)
      if (sort === 'newest') items = [...items].reverse()

      const total = items.length
      const total_pages = Math.max(1, Math.ceil(total / limit))
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          data: items.slice((pageNo - 1) * limit, pageNo * limit),
          meta: { page: pageNo, limit, total, total_pages },
        }),
      })
      return
    }

    await route.fulfill({
      status: 404,
      contentType: 'application/json',
      body: JSON.stringify({ error: { code: 'NOT_FOUND', message: 'Tidak ditemukan' } }),
    })
  })
}

async function capture(browser, { name, path, viewport, fullPage }) {
  const page = await browser.newPage({ viewport, deviceScaleFactor: 1 })
  await mockApi(page)
  await page.goto(`${baseURL}${path}`, { waitUntil: 'networkidle' })
  await page.waitForSelector('article', { timeout: 15000 })
  await page.evaluate(() => document.fonts.ready)
  await page.screenshot({ path: resolve(outDir, name), fullPage })
  await page.close()
  console.log(`Tersimpan: ui-app/${name}`)
}

await mkdir(outDir, { recursive: true })
const browser = await chromium.launch()

try {
  await capture(browser, {
    name: 'beranda-mobile.png',
    path: '/',
    viewport: { width: 390, height: 844 },
    fullPage: false,
  })
  await capture(browser, {
    name: 'beranda-mobile-full.png',
    path: '/',
    viewport: { width: 390, height: 844 },
    fullPage: true,
  })
  await capture(browser, {
    name: 'produk-mobile.png',
    path: '/produk',
    viewport: { width: 390, height: 844 },
    fullPage: false,
  })
  await capture(browser, {
    name: 'produk-mobile-full.png',
    path: '/produk',
    viewport: { width: 390, height: 844 },
    fullPage: true,
  })
  await capture(browser, {
    name: 'beranda-desktop.png',
    path: '/',
    viewport: { width: 1280, height: 800 },
    fullPage: false,
  })
  await capture(browser, {
    name: 'produk-desktop.png',
    path: '/produk',
    viewport: { width: 1280, height: 800 },
    fullPage: false,
  })
} finally {
  await browser.close()
}
