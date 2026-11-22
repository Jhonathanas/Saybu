interface ProductImageProps {
  src?: string
  alt: string
  className?: string
}

/** ProductImage menampilkan gambar produk atau placeholder bila belum ada. */
export function ProductImage({ src, alt, className = '' }: ProductImageProps) {
  if (src) {
    return (
      <img
        src={src}
        alt={alt}
        loading="lazy"
        className={`object-cover ${className}`}
      />
    )
  }
  return (
    <div
      role="img"
      aria-label={`${alt} (belum ada foto)`}
      className={`flex items-center justify-center bg-muted text-2xl text-muted-foreground ${className}`}
    >
      🥬
    </div>
  )
}
