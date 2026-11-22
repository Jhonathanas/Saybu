import { useState } from 'react'

interface QtyPickerProps {
  unit: string
  value: number
  min: number
  step: number
  max: number
  onChange: (value: number) => void
  disabled?: boolean
}

function toDisplay(unit: string, qty: number): string {
  if (unit === 'kg') {
    return String(Number((qty / 1000).toFixed(3)))
  }
  return String(qty)
}

function fromDisplay(unit: string, raw: string): number {
  const parsed = Number(raw.replace(',', '.'))
  if (!Number.isFinite(parsed)) return 0
  return Math.round(unit === 'kg' ? parsed * 1000 : parsed)
}

/** QtyPicker memilih jumlah dengan tombol −/+ dan input teks. */
export function QtyPicker({
  unit,
  value,
  min,
  step,
  max,
  onChange,
  disabled = false,
}: QtyPickerProps) {
  const [draft, setDraft] = useState<string | null>(null)
  const stepValue = step > 0 ? step : 1
  const minValue = min > 0 ? min : stepValue

  const clamp = (next: number) => Math.max(0, Math.min(next, max))

  const buttonClass =
    'tap-target inline-flex items-center justify-center rounded-lg border border-border bg-card text-lg text-foreground hover:bg-muted disabled:opacity-40'

  return (
    <div className="flex items-center gap-2">
      <button
        type="button"
        className={buttonClass}
        aria-label="Kurangi jumlah"
        disabled={disabled || value <= minValue}
        onClick={() => onChange(clamp(value - stepValue))}
      >
        −
      </button>
      <input
        aria-label="Jumlah"
        inputMode="decimal"
        className="tap-target w-20 rounded-lg border border-border bg-card px-2 text-center text-sm text-foreground"
        value={draft ?? toDisplay(unit, value)}
        disabled={disabled}
        onChange={(e) => setDraft(e.target.value)}
        onBlur={() => {
          if (draft !== null) {
            onChange(clamp(fromDisplay(unit, draft)))
            setDraft(null)
          }
        }}
      />
      <span className="w-8 text-sm text-muted-foreground">{unit}</span>
      <button
        type="button"
        className={buttonClass}
        aria-label="Tambah jumlah"
        disabled={disabled || value >= max}
        onClick={() => onChange(clamp(value + stepValue))}
      >
        +
      </button>
    </div>
  )
}
