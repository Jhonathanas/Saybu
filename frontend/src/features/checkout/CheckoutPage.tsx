import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router'
import { z } from 'zod'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Feedback'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { useAuth } from '@/features/auth/AuthProvider'
import { lineTotal, useCart } from '@/features/cart/store'
import { ApiError } from '@/lib/api'
import { formatIDR, formatQty } from '@/lib/format'
import { usePageTitle } from '@/lib/seo'
import { createOrder, getQuote, listZones, newIdempotencyKey } from './api'
import type { CreateOrderInput, Fulfillment, PaymentMethod } from './types'

const schema = z
  .object({
    name: z.string().trim().min(1, 'Nama wajib diisi.'),
    phone: z
      .string()
      .trim()
      .min(8, 'Nomor HP minimal 8 digit.')
      .max(15, 'Nomor HP maksimal 15 digit.'),
    email: z.union([z.literal(''), z.string().trim().email('Email tidak valid.')]),
    note: z.string().trim(),
    fulfillment: z.enum(['DELIVERY', 'PICKUP']),
    zone_id: z.string(),
    line: z.string(),
    rt_rw: z.string(),
    kelurahan: z.string(),
    kecamatan: z.string(),
    city: z.string(),
    postal_code: z.string(),
    landmark: z.string(),
    payment_method: z.enum(['TRANSFER', 'COD']),
  })
  .superRefine((val, ctx) => {
    if (val.fulfillment !== 'DELIVERY') return
    if (!val.zone_id) {
      ctx.addIssue({ path: ['zone_id'], code: z.ZodIssueCode.custom, message: 'Pilih zona ongkir.' })
    }
    if (!val.line.trim()) {
      ctx.addIssue({
        path: ['line'],
        code: z.ZodIssueCode.custom,
        message: 'Alamat wajib untuk pengiriman.',
      })
    }
  })

type FormValues = z.infer<typeof schema>

/** describeError menggabungkan pesan error dengan detail validasi bila ada. */
function describeError(err: ApiError): string {
  if (err.isNetworkError) {
    return 'Tidak bisa menghubungi server. Periksa koneksi lalu coba lagi.'
  }
  const extra = err.details.map((d) => d.message).filter(Boolean).join(' ')
  return extra ? `${err.message} ${extra}` : err.message
}

export function CheckoutPage() {
  usePageTitle('Checkout')
  const { user } = useAuth()
  const navigate = useNavigate()
  const lines = useCart((s) => s.lines)
  const clearCart = useCart((s) => s.clear)
  const [formError, setFormError] = useState<string | null>(null)

  const subtotal = useMemo(() => lines.reduce((sum, l) => sum + lineTotal(l), 0), [lines])

  const {
    register,
    handleSubmit,
    watch,
    reset,
    getValues,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      name: user?.name ?? '',
      phone: user?.phone ?? '',
      email: user?.email ?? '',
      note: '',
      fulfillment: 'DELIVERY',
      zone_id: '',
      line: '',
      rt_rw: '',
      kelurahan: '',
      kecamatan: '',
      city: '',
      postal_code: '',
      landmark: '',
      payment_method: 'TRANSFER',
    },
  })

  // Isi otomatis data pembeli saat profil user tersedia (tanpa menimpa isian).
  useEffect(() => {
    if (!user) return
    const current = getValues()
    reset({
      ...current,
      name: current.name || user.name,
      phone: current.phone || user.phone,
      email: current.email || user.email || '',
    })
  }, [user, getValues, reset])

  const fulfillment = watch('fulfillment')
  const zoneId = watch('zone_id')
  const delivery = fulfillment === 'DELIVERY'

  const zonesQuery = useQuery({ queryKey: ['shipping-zones'], queryFn: listZones })

  const quoteQuery = useQuery({
    queryKey: ['shipping-quote', zoneId, subtotal],
    queryFn: () => getQuote(zoneId, subtotal),
    enabled: delivery && !!zoneId,
  })

  const shippingFee = delivery ? (quoteQuery.data?.fee ?? 0) : 0
  const total = subtotal + shippingFee

  // Idempotency key stabil selama isi pesanan tidak berubah, agar klik ganda
  // atau retry jaringan tidak membuat pesanan ganda.
  const idemRef = useRef<{ key: string; fingerprint: string } | null>(null)

  function keyFor(payload: CreateOrderInput): string {
    const fingerprint = JSON.stringify(payload)
    if (idemRef.current && idemRef.current.fingerprint === fingerprint) {
      return idemRef.current.key
    }
    const key = newIdempotencyKey()
    idemRef.current = { key, fingerprint }
    return key
  }

  async function onSubmit(values: FormValues) {
    setFormError(null)
    if (lines.length === 0) {
      setFormError('Keranjang kosong.')
      return
    }

    const delivery = values.fulfillment === 'DELIVERY'
    const payload: CreateOrderInput = {
      customer: {
        name: values.name.trim(),
        phone: values.phone.trim(),
        email: values.email.trim() || undefined,
        note: values.note.trim() || undefined,
      },
      fulfillment: values.fulfillment as Fulfillment,
      zone_id: delivery ? values.zone_id : undefined,
      address: delivery
        ? {
            line: values.line.trim(),
            rt_rw: values.rt_rw.trim(),
            kelurahan: values.kelurahan.trim(),
            kecamatan: values.kecamatan.trim(),
            city: values.city.trim(),
            postal_code: values.postal_code.trim(),
            landmark: values.landmark.trim(),
          }
        : undefined,
      payment_method: values.payment_method as PaymentMethod,
      items: lines.map((l) => ({ product_id: l.productId, qty: l.qty })),
    }

    try {
      const order = await createOrder(payload, keyFor(payload))
      idemRef.current = null
      clearCart()
      const phone = encodeURIComponent(order.customer.phone)
      navigate(`/pesanan/${order.code}?phone=${phone}&baru=1`, { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        setFormError(describeError(err))
      } else {
        setFormError('Terjadi kesalahan tak terduga. Coba lagi.')
      }
    }
  }

  if (lines.length === 0) {
    return (
      <div className="flex flex-col gap-4">
        <h1 className="text-xl font-bold text-stone-900">Checkout</h1>
        <Alert tone="info">
          Keranjang kosong.{' '}
          <Link to="/produk" className="font-medium underline">
            Mulai belanja
          </Link>
          .
        </Alert>
      </div>
    )
  }

  const zoneOptions = [
    { value: '', label: zonesQuery.isLoading ? 'Memuat zona…' : 'Pilih zona ongkir' },
    ...(zonesQuery.data ?? []).map((z) => ({
      value: z.id,
      label: `${z.name} — ${formatIDR(z.fee)}`,
    })),
  ]

  return (
    <div className="flex flex-col gap-5">
      <h1 className="text-xl font-bold text-stone-900">Checkout</h1>

      {formError && <Alert tone="error">{formError}</Alert>}
      {zonesQuery.isError && (
        <Alert tone="error">Gagal memuat zona ongkir. Muat ulang halaman.</Alert>
      )}

      <form onSubmit={handleSubmit(onSubmit)} noValidate className="grid gap-5 lg:grid-cols-[1fr_20rem]">
        <div className="flex flex-col gap-5">
          <section className="rounded-2xl border border-stone-200 bg-white p-4">
            <h2 className="font-semibold text-stone-900">Data pembeli</h2>
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              <Input label="Nama" error={errors.name?.message} {...register('name')} />
              <Input
                label="Nomor HP / WhatsApp"
                inputMode="tel"
                placeholder="081234567890"
                error={errors.phone?.message}
                {...register('phone')}
              />
              <Input
                label="Email (opsional)"
                type="email"
                error={errors.email?.message}
                {...register('email')}
              />
              <Input
                label="Catatan (opsional)"
                placeholder="Mis. pilih yang segar"
                {...register('note')}
              />
            </div>
          </section>

          <section className="rounded-2xl border border-stone-200 bg-white p-4">
            <h2 className="font-semibold text-stone-900">Pengiriman</h2>
            <div className="mt-3 flex flex-col gap-3">
              <div role="radiogroup" aria-label="Cara pengiriman" className="grid gap-2 sm:grid-cols-2">
                {(
                  [
                    { value: 'DELIVERY', label: 'Dikirim ke alamat' },
                    { value: 'PICKUP', label: 'Ambil di tempat' },
                  ] as const
                ).map((opt) => (
                  <label
                    key={opt.value}
                    className={`tap-target flex cursor-pointer items-center gap-2 rounded-lg border px-3 text-sm ${
                      fulfillment === opt.value
                        ? 'border-brand-500 bg-brand-50 text-brand-800'
                        : 'border-stone-300 bg-white text-stone-700'
                    }`}
                  >
                    <input
                      type="radio"
                      value={opt.value}
                      className="accent-brand-600"
                      {...register('fulfillment')}
                    />
                    {opt.label}
                  </label>
                ))}
              </div>

              {delivery && (
                <>
                  <Select
                    label="Zona ongkir"
                    options={zoneOptions}
                    error={errors.zone_id?.message}
                    disabled={zonesQuery.isLoading}
                    {...register('zone_id')}
                  />
                  {quoteQuery.isError && (
                    <Alert tone="error">Zona tidak dikenal. Pilih zona lain.</Alert>
                  )}
                  <Input
                    label="Alamat lengkap"
                    placeholder="Jl. Melati No. 12"
                    error={errors.line?.message}
                    {...register('line')}
                  />
                  <div className="grid gap-3 sm:grid-cols-2">
                    <Input label="RT/RW" placeholder="003/004" {...register('rt_rw')} />
                    <Input label="Kelurahan" {...register('kelurahan')} />
                    <Input label="Kecamatan" {...register('kecamatan')} />
                    <Input label="Kota/Kabupaten" {...register('city')} />
                    <Input label="Kode pos" inputMode="numeric" {...register('postal_code')} />
                    <Input label="Patokan (opsional)" {...register('landmark')} />
                  </div>
                </>
              )}

              {!delivery && (
                <Alert tone="info">
                  Pesanan diambil langsung di toko. Ongkir tidak dikenakan.
                </Alert>
              )}
            </div>
          </section>

          <section className="rounded-2xl border border-stone-200 bg-white p-4">
            <h2 className="font-semibold text-stone-900">Metode pembayaran</h2>
            <div role="radiogroup" aria-label="Metode pembayaran" className="mt-3 grid gap-2 sm:grid-cols-2">
              {(
                [
                  { value: 'TRANSFER', label: 'Transfer bank', hint: 'Instruksi muncul setelah pesan' },
                  { value: 'COD', label: 'Bayar di tempat (COD)', hint: 'Bayar saat terima/ambil' },
                ] as const
              ).map((opt) => (
                <label
                  key={opt.value}
                  className={`tap-target flex cursor-pointer flex-col justify-center rounded-lg border px-3 py-2 text-sm ${
                    watch('payment_method') === opt.value
                      ? 'border-brand-500 bg-brand-50 text-brand-800'
                      : 'border-stone-300 bg-white text-stone-700'
                  }`}
                >
                  <span className="flex items-center gap-2 font-medium">
                    <input
                      type="radio"
                      value={opt.value}
                      className="accent-brand-600"
                      {...register('payment_method')}
                    />
                    {opt.label}
                  </span>
                  <span className="pl-6 text-xs text-stone-500">{opt.hint}</span>
                </label>
              ))}
            </div>
          </section>
        </div>

        <aside className="lg:sticky lg:top-20 lg:self-start">
          <div className="flex flex-col gap-3 rounded-2xl border border-stone-200 bg-white p-4">
            <h2 className="font-semibold text-stone-900">Ringkasan pesanan</h2>
            <ul className="flex flex-col divide-y divide-stone-100">
              {lines.map((line) => (
                <li key={line.productId} className="flex items-start justify-between gap-3 py-2">
                  <div>
                    <p className="text-sm font-medium text-stone-900">{line.name}</p>
                    <p className="text-xs text-stone-500">
                      {formatQty(line.unit, line.qty)} × {formatIDR(line.price)}
                    </p>
                  </div>
                  <p className="text-sm font-medium text-stone-900">{formatIDR(lineTotal(line))}</p>
                </li>
              ))}
            </ul>
            <dl className="flex flex-col gap-1.5 border-t border-stone-100 pt-3 text-sm">
              <div className="flex justify-between">
                <dt className="text-stone-500">Subtotal</dt>
                <dd className="text-stone-900">{formatIDR(subtotal)}</dd>
              </div>
              <div className="flex justify-between">
                <dt className="text-stone-500">Ongkir</dt>
                <dd className="text-stone-900">
                  {!delivery
                    ? 'Rp0'
                    : quoteQuery.isFetching
                      ? 'Menghitung…'
                      : shippingFee === 0
                        ? 'Gratis'
                        : formatIDR(shippingFee)}
                </dd>
              </div>
              <div className="flex justify-between text-base font-bold text-stone-900">
                <dt>Total</dt>
                <dd>{formatIDR(total)}</dd>
              </div>
            </dl>
            <Button type="submit" loading={isSubmitting} className="w-full">
              Buat pesanan
            </Button>
            <p className="text-center text-xs text-stone-500">
              Harga & stok dihitung ulang oleh server saat pesanan dibuat.
            </p>
          </div>
        </aside>
      </form>
    </div>
  )
}
