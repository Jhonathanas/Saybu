import { useEffect, useState } from 'react'

/** Gambar cadangan untuk produk tanpa foto atau saat URL gagal dimuat. */
export const DEFAULT_PRODUCT_IMAGE = '/images/produk-default.webp'

interface ProductImageProps {
  src?: string
  alt: string
  className?: string
}

/** ProductImage menampilkan gambar produk dengan fallback default yang valid. */
export function ProductImage({ src, alt, className = '' }: ProductImageProps) {
  const [failed, setFailed] = useState(false)

  // Produk lain bisa dipilih tanpa unmount komponen yang sama (mis. galeri).
  useEffect(() => setFailed(false), [src])

  const resolved = src && !failed ? src : DEFAULT_PRODUCT_IMAGE

  return (
    <img
      src={resolved}
      alt={alt}
      loading="lazy"
      className={`object-cover ${className}`}
      onError={() => setFailed(true)}
    />
  )
}
