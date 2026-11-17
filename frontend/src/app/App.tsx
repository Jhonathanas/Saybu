import { Navigate, Route, Routes } from 'react-router'
import { AdminLayout } from './layouts/AdminLayout'
import { StoreLayout } from './layouts/StoreLayout'
import { AccountPage } from './pages/AccountPage'
import { CartPage } from './pages/CartPage'
import { HomePage } from './pages/HomePage'
import { NotFoundPage } from './pages/NotFoundPage'
import { ServerErrorPage } from './pages/ServerErrorPage'
import { ProductDetailPage } from './pages/ProductDetailPage'
import { ProductListPage } from './pages/ProductListPage'
import { LoginPage } from '@/features/auth/LoginPage'
import { RegisterPage } from '@/features/auth/RegisterPage'
import { RequireAuth, RequireRole } from '@/features/auth/guards'
import { CheckoutPage } from '@/features/checkout/CheckoutPage'
import { OrderDetailPage } from '@/features/checkout/OrderDetailPage'
import { OrderTrackingPage } from '@/features/checkout/OrderTrackingPage'
import { AdminDashboardPage } from '@/features/admin/AdminDashboardPage'
import { AdminOrderDetailPage } from '@/features/admin/AdminOrderDetailPage'
import { AdminOrdersPage } from '@/features/admin/AdminOrdersPage'
import { AdminPaymentsPage } from '@/features/admin/AdminPaymentsPage'
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
        <Route path="checkout" element={<CheckoutPage />} />
        <Route path="pesanan" element={<OrderTrackingPage />} />
        <Route path="pesanan/:code" element={<OrderDetailPage />} />
        <Route element={<RequireAuth />}>
          <Route path="akun" element={<AccountPage />} />
        </Route>
      </Route>

      <Route path="masuk" element={<LoginPage />} />
      <Route path="daftar" element={<RegisterPage />} />

      <Route path="admin" element={<RequireRole role="admin" />}>
        <Route element={<AdminLayout />}>
          <Route index element={<AdminDashboardPage />} />
          <Route path="pesanan" element={<AdminOrdersPage />} />
          <Route path="pesanan/:id" element={<AdminOrderDetailPage />} />
          <Route path="produk" element={<AdminProductsPage />} />
          <Route path="pembayaran" element={<AdminPaymentsPage />} />
          <Route path="pengguna" element={<AdminUsersPage />} />
        </Route>
      </Route>

      <Route path="login" element={<Navigate to="/masuk" replace />} />
      <Route path="register" element={<Navigate to="/daftar" replace />} />
      <Route path="500" element={<ServerErrorPage />} />
      <Route path="*" element={<NotFoundPage />} />
    </Routes>
  )
}
