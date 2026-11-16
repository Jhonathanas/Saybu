import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from './Button'

/** CopyButton menyalin teks ke papan klip dengan umpan balik toast. */
export function CopyButton({ value, label = 'Salin' }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      toast.success('Disalin ke papan klip')
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error('Gagal menyalin. Salin manual ya.')
    }
  }

  return (
    <Button type="button" variant="secondary" size="sm" onClick={copy}>
      {copied ? 'Tersalin ✓' : label}
    </Button>
  )
}
