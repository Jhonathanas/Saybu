import { Navigate, Outlet, useLocation } from 'react-router'
import { PageLoader } from '@/components/ui/Feedback'
import { useAuth } from './AuthProvider'
import type { Role } from './types'

/** RequireAuth melindungi rute yang butuh login. */
export function RequireAuth() {
  const { user, loading } = useAuth()
  const location = useLocation()

  if (loading) return <PageLoader />
  if (!user) {
    return <Navigate to="/masuk" replace state={{ from: location.pathname }} />
  }
  return <Outlet />
}

/** RequireRole melindungi rute yang butuh role tertentu (mis. admin). */
export function RequireRole({ role }: { role: Role }) {
  const { user, loading } = useAuth()

  if (loading) return <PageLoader />
  if (!user) return <Navigate to="/masuk" replace />
  if (user.role !== role) return <Navigate to="/" replace />
  return <Outlet />
}
