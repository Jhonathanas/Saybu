import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { DEFAULT_PRODUCT_IMAGE, ProductImage } from './ProductImage'

describe('ProductImage', () => {
  it('memakai gambar default bila src kosong', () => {
    render(<ProductImage alt="Bayam" className="size-16" />)

    expect(screen.getByRole('img', { name: 'Bayam' })).toHaveAttribute(
      'src',
      DEFAULT_PRODUCT_IMAGE,
    )
  })

  it('mengganti ke gambar default bila URL gagal dimuat', () => {
    render(<ProductImage src="/uploads/rusak.jpg" alt="Apel" />)
    const img = screen.getByRole('img', { name: 'Apel' })

    expect(img).toHaveAttribute('src', '/uploads/rusak.jpg')
    fireEvent.error(img)
    expect(img).toHaveAttribute('src', DEFAULT_PRODUCT_IMAGE)
  })
})
