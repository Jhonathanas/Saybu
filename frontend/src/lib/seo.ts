// Helper SEO dasar: judul dokumen & meta description per halaman.
//
// Aplikasi ini SPA tanpa SSR; crawler modern menjalankan JS, jadi menyetel
// title/description lewat efek sudah cukup untuk kebutuhan toko ini.

import { useEffect } from 'react'

const BASE_TITLE = 'Saybu'

const DEFAULT_DESCRIPTION = 'Saybu — toko buah & sayur segar, antar ke rumah.'

/**
 * usePageTitle menyetel document.title (dan meta description bila diberikan)
 * selama halaman ter-mount.
 */
export function usePageTitle(title?: string, description?: string): void {
  useEffect(() => {
    document.title = title ? `${title} — ${BASE_TITLE}` : `${BASE_TITLE} — Buah & Sayur Segar`
  }, [title])

  useEffect(() => {
    if (description) setMetaDescription(description)
  }, [description])
}

/** setMetaDescription memperbarui atau membuat tag <meta name="description">. */
export function setMetaDescription(content: string): void {
  let el = document.querySelector('meta[name="description"]')
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute('name', 'description')
    document.head.appendChild(el)
  }
  el.setAttribute('content', content)
}

export { DEFAULT_DESCRIPTION }
