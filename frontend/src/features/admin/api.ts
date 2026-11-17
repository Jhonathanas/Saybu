// Panggilan API khusus panel admin (ringkasan dashboard).
import { request } from '@/lib/api'
import type { TodayStats } from './types'

/** getTodayStats mengambil ringkasan operasional hari ini (admin). */
export function getTodayStats(): Promise<TodayStats> {
  return request<TodayStats>('/admin/stats/today')
}
