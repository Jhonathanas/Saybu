// Tipe ringkasan dashboard admin (selaras dengan docs/API_SPEC.md §7.9).

export interface LowStockItem {
  product_id: string
  name: string
  /** Stok dalam satuan terkecil (gram untuk kg). */
  stock: number
  unit: string
}

/** TodayStats adalah ringkasan operasional hari ini untuk dashboard admin. */
export interface TodayStats {
  /** Jumlah pesanan yang dibuat hari ini (Asia/Jakarta). */
  orders_count: number
  /** Total pesanan hari ini yang sudah lunas. */
  revenue_paid: number
  /** Jumlah pesanan yang menunggu verifikasi saat ini (backlog). */
  awaiting_verification: number
  low_stock: LowStockItem[]
}
