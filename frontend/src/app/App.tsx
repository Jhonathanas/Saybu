import { Navigate, Route, Routes } from 'react-router'
import { AdminLayout } from './layouts/AdminLayout'
import { StoreLayout } from './layouts/StoreLayout'
import { AccountPage } from './pages/AccountPage'
import { CartPage } from './pages/CartPage'
import { HomePage } from './pages/HomePage'
import { NotFoundPage } from './pages/NotFoundPage'
import { ProductDetailPage } from './pages/ProductDetailPage'
import { ProductListPage } from './pages/ProductListPage'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { RequireAuth, RequireRole } from '@/features/auth/guards'
import { AdminDashboardPage } from '@/features/admin/AdminDashboardPage'
import { AdminProductsPage } from '@/features/admin/AdminProductsPage'
import { AdminUsersPage } from '@/features/admin/AdminUsersPage'

export function App() {
  return (
    <Routes>
      <Route element={<StoreLayout />}>
        <Route index element={<HomePage />} />
        <Route path="produk" element={<ProductListPage />} />
        <Route path="produk/:slug" element={<ProductDetailPage />} />
        <Route path="keranjang" element={<CartPage />} />
        <Route element={<RequireAuth />}>
          <Route path="akun" element={<AccountPage />} />
        </Route>
      </Route>

      <Route path="masuk" element={<LoginPage />} />
      <Route path="daftar" element={<RegisterPage />} />

      <Route path="admin" element={<RequireRole role="admin" />}>
        <Route element={<AdminLayout />}>
          <Route index element={<AdminDashboardPage />} />
          <Route path="produk" element={<AdminProductsPage />} />
          <Route path="pengguna" element={<AdminUsersPage />} />
        </Route>
      </Route>

      <Route path="login" element={<Navigate to="/masuk" replace />} />
      <Route path="register" element={<Navigate to="/daftar" replace />} />
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
