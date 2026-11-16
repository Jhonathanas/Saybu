import { useMutation } from '@tanstack/react-query'
import { useRef, useState } from 'react'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Feedback'
import { ApiError } from '@/lib/api'
import { uploadPaymentProof } from './api'
import type { Payment } from './types'

/** Batas ukuran berkas bukti transfer (selaras UPLOAD_MAX_BYTES). */
const MAX_BYTES = 5 * 1024 * 1024
const ALLOWED_EXT = ['.jpg', '.jpeg', '.png', '.pdf']

interface ProofUploadFormProps {
  code: string
  phone?: string
  onUploaded: (payment: Payment) => void
}

/** ProofUploadForm mengunggah bukti transfer untuk sebuah pesanan. */
export function ProofUploadForm({ code, phone, onUploaded }: ProofUploadFormProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [file, setFile] = useState<File | null>(null)
  const [error, setError] = useState<string | null>(null)

  const upload = useMutation({
    mutationFn: (selected: File) => uploadPaymentProof(code, selected, phone),
    onSuccess: (payment) => {
      setFile(null)
      setError(null)
      if (inputRef.current) inputRef.current.value = ''
      onUploaded(payment)
    },
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Gagal mengunggah bukti. Coba lagi.')
    },
  })

  function handleSelect(selected: File | null) {
    setError(null)
    if (!selected) {
      setFile(null)
      return
    }
    const ext = `.${selected.name.split('.').pop()?.toLowerCase() ?? ''}`
    if (!ALLOWED_EXT.includes(ext)) {
      setError('Format berkas harus JPG, PNG, atau PDF.')
      setFile(null)
      return
    }
    if (selected.size > MAX_BYTES) {
      setError('Ukuran berkas melebihi 5 MB.')
      setFile(null)
      return
    }
    setFile(selected)
  }

  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault()
        if (file) upload.mutate(file)
      }}
    >
      <div className="flex flex-col gap-1.5">
        <label htmlFor="proof-file" className="text-sm font-medium text-stone-700">
          Unggah bukti transfer
        </label>
        <input
          id="proof-file"
          ref={inputRef}
          type="file"
          accept=".jpg,.jpeg,.png,.pdf,image/jpeg,image/png,application/pdf"
          className="text-sm"
          disabled={upload.isPending}
          onChange={(e) => handleSelect(e.target.files?.[0] ?? null)}
        />
        <p className="text-xs text-stone-500">
          Format JPG, PNG, atau PDF. Maksimal 5 MB. Sertakan kode pesanan bila memungkinkan.
        </p>
      </div>

      {error && <Alert tone="error">{error}</Alert>}

      <Button type="submit" loading={upload.isPending} disabled={!file}>
        Kirim bukti
      </Button>
    </form>
  )
}
