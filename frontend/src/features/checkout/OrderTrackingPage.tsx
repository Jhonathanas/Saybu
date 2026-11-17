import { zodResolver } from '@hookform/resolvers/zod'
import { useForm } from 'react-hook-form'
import { useNavigate } from 'react-router'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { usePageTitle } from '@/lib/seo'

const schema = z.object({
  code: z.string().trim().min(1, 'Isi kode pesanan.'),
  phone: z.string().trim().min(8, 'Isi nomor HP pemesan.'),
})

type FormValues = z.infer<typeof schema>

/** OrderTrackingPage melacak pesanan dengan kode + nomor HP (guest). */
export function OrderTrackingPage() {
  usePageTitle('Lacak pesanan')
  const navigate = useNavigate()
  const {
    register,
    handleSubmit,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  async function onSubmit(values: FormValues) {
    const code = values.code.trim().toUpperCase()
    const phone = values.phone.trim()
    navigate(`/pesanan/${encodeURIComponent(code)}?phone=${encodeURIComponent(phone)}`)
  }

  return (
    <div className="mx-auto flex max-w-md flex-col gap-6 py-4">
      <header>
        <h1 className="text-xl font-bold text-stone-900">Lacak pesanan</h1>
        <p className="mt-1 text-sm text-stone-500">
          Masukkan kode pesanan dan nomor HP yang dipakai saat memesan.
        </p>
      </header>

      <form onSubmit={handleSubmit(onSubmit)} noValidate className="flex flex-col gap-4">
        <Input
          label="Kode pesanan"
          placeholder="SB-20261114-0001"
          autoCapitalize="characters"
          error={errors.code?.message}
          {...register('code')}
        />
        <Input
          label="Nomor HP"
          inputMode="tel"
          placeholder="081234567890"
          error={errors.phone?.message}
          {...register('phone')}
        />
        <Button type="submit" loading={isSubmitting}>
          Lacak pesanan
        </Button>
      </form>
    </div>
  )
}
