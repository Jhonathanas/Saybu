import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { Link, useLocation, useNavigate } from 'react-router'
import { z } from 'zod'
import { Alert } from '@/components/ui/Feedback'
import { Button } from '@/components/ui/Button'
import { Input } from '@/components/ui/Input'
import { ApiError } from '@/lib/api'
import { useAuth } from './AuthProvider'

const schema = z.object({
  identifier: z.string().trim().min(1, 'Isi nomor HP atau email.'),
  password: z.string().min(1, 'Isi kata sandi.'),
})

type FormValues = z.infer<typeof schema>

export function LoginPage() {
  const { login } = useAuth()
  const navigate = useNavigate()
  const location = useLocation()
  const [formError, setFormError] = useState<string | null>(null)

  const {
    register,
    handleSubmit,
    setError,
    formState: { errors, isSubmitting },
  } = useForm<FormValues>({ resolver: zodResolver(schema) })

  const from = (location.state as { from?: string } | null)?.from ?? '/'

  async function onSubmit(values: FormValues) {
    setFormError(null)
    try {
      await login(values)
      navigate(from, { replace: true })
    } catch (err) {
      if (err instanceof ApiError) {
        setFormError(err.message)
      } else {
        setError('root', { message: 'Terjadi kesalahan tak terduga.' })
      }
    }
  }

  return (
    <div className="mx-auto flex min-h-screen max-w-md flex-col justify-center gap-6 px-4 py-10">
      <header className="text-center">
        <h1 className="text-2xl font-bold text-brand-700">Masuk ke Saybu</h1>
        <p className="mt-1 text-sm text-stone-500">
          Gunakan nomor HP atau email yang terdaftar.
        </p>
      </header>

      {formError && <Alert tone="error">{formError}</Alert>}
      {errors.root?.message && <Alert tone="error">{errors.root.message}</Alert>}

      <form onSubmit={handleSubmit(onSubmit)} className="flex flex-col gap-4" noValidate>
        <Input
          label="Nomor HP atau email"
          autoComplete="username"
          placeholder="081234567890"
          error={errors.identifier?.message}
          {...register('identifier')}
        />
        <Input
          label="Kata sandi"
          type="password"
          autoComplete="current-password"
          error={errors.password?.message}
          {...register('password')}
        />
        <Button type="submit" loading={isSubmitting}>
          Masuk
        </Button>
      </form>

      <p className="text-center text-sm text-stone-600">
        Belum punya akun?{' '}
        <Link to="/daftar" className="font-medium text-brand-700 hover:underline">
          Daftar di sini
        </Link>
      </p>
      <p className="text-center text-sm">
        <Link to="/" className="text-stone-500 hover:underline">
          Lanjut belanja tanpa akun
        </Link>
      </p>
    </div>
  )
}
