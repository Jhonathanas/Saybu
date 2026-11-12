import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useNavigate } from 'react-router'
import { z } from 'zod'
import { Alert } from '@/components/ui/Feedback'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { ApiError } from '@/lib/api'
import { useAuth } from './AuthProvider'

const phonePattern = /^08[0-9]{8,11}$/

const schema = z
  .object({
    name: z.string().trim().min(2, 'Nama minimal 2 karakter.'),
    phone: z
      .string()
      .trim()
      .regex(phonePattern, 'Format nomor HP: 08xxxxxxxxxx (10–13 angka).'),
    email: z
      .string()
      .trim()
      .email('Format email tidak valid.')
      .optional()
      .or(z.literal('')),
    password: z.string().min(8, 'Kata sandi minimal 8 karakter.'),
    confirm: z.string(),
  })
  .refine((v) => v.password === v.confirm, {
    path: ['confirm'],
    message: 'Kata sandi tidak sama.',
  })

type FormValues = z.infer<typeof schema>

export function RegisterPage() {
  const { register: registerUser } = useAuth()
  const navigate = useNavigate()
  const [formError, setFormError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  async function onSubmit(values: FormValues) {
    setFormError(null)
    try {
      await registerUser({
        name: values.name,
        phone: values.phone,
        email: values.email ? values.email : undefined,
        password: values.password,
      })
      navigate('/', { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        // Tampilkan kesalahan per-field dari server bila ada.
        let mapped = false
        for (const detail of err.details) {
          if (
            detail.field === 'name' ||
            detail.field === 'phone' ||
            detail.field === 'email' ||
            detail.field === 'password'
          ) {
            setError(detail.field, { message: detail.message })
            mapped = true
          }
        }
        if (!mapped) setFormError(err.message)
      } else {
        setError('root', { message: 'Terjadi kesalahan tak terduga.' })
      }
    }
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col justify-center gap-6 px-4 py-10">
      <header className="text-center">
        <h1 className="text-2xl font-bold text-brand-700">Daftar Akun Saybu</h1>
        <p className="mt-1 text-sm text-stone-500">
          Daftar untuk menyimpan riwayat pesanan. Bisa juga belanja tanpa akun.
        </p>
      </header>

      {formError && <Alert tone="error">{formError}</Alert>}
      {errors.root?.message && <Alert tone="error">{errors.root.message}</Alert>}

      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <Input
          label="Nama"
          autoComplete="name"
          placeholder="Siti Rahayu"
          error={errors.name?.message}
          {...register('name')}
        />
        <Input
          label="Nomor HP"
          type="tel"
          inputMode="numeric"
          autoComplete="tel"
          placeholder="081234567890"
          hint="Dipakai untuk konfirmasi pesanan."
          error={errors.phone?.message}
          {...register('phone')}
        />
        <Input
          label="Email (opsional)"
          type="email"
          autoComplete="email"
          placeholder="siti@example.com"
          error={errors.email?.message}
          {...register('email')}
        />
        <Input
          label="Kata sandi"
          type="password"
          autoComplete="new-password"
          hint="Minimal 8 karakter."
          error={errors.password?.message}
          {...register('password')}
        />
        <Input
          label="Ulangi kata sandi"
          type="password"
          autoComplete="new-password"
          error={errors.confirm?.message}
          {...register('confirm')}
        />
        <Button type="submit" loading={isSubmitting}>
          Daftar
        </Button>
      </form>

      <p className="text-center text-sm text-stone-600">
        Sudah punya akun?{' '}
        <Link to="/masuk" className="font-medium text-brand-700 hover:underline">
          Masuk
        </Link>
      </p>
    </div>
  )
}
