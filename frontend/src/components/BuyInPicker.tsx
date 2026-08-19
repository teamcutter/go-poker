import { haptic } from '../telegram'
import { chips } from '../utils/chips'

export const BUY_INS = [100, 200, 500, 1000]

/** Largest preset the player can actually afford, or 0 when they are broke. */
export function defaultBuyIn(bankroll: number): number {
  const affordable = BUY_INS.filter((n) => n <= bankroll)
  return affordable.length > 0 ? affordable[affordable.length - 1] : 0
}

interface BuyInPickerProps {
  bankroll: number
  value: number
  onChange: (amount: number) => void
}

export default function BuyInPicker({ bankroll, value, onChange }: BuyInPickerProps) {
  return (
    <div className="field" style={{ marginBottom: 18 }}>
      <div className="space-between">
        <span className="field-label">Buy-in</span>
        <span className="field-label">Bankroll {chips(bankroll)}</span>
      </div>
      <div className="segmented">
        {BUY_INS.map((n) => (
          <button
            key={n}
            className={n === value ? 'on' : ''}
            disabled={n > bankroll}
            onClick={() => {
              haptic()
              onChange(n)
            }}
          >
            {chips(n)}
          </button>
        ))}
      </div>
    </div>
  )
}
