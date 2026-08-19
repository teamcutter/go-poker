export function chips(value: number): string {
  if (!Number.isFinite(value)) return '0'
  const n = Math.round(value)
  if (Math.abs(n) >= 1_000_000) return trim(n / 1_000_000) + 'M'
  if (Math.abs(n) >= 10_000) return trim(n / 1000) + 'K'
  return n.toLocaleString('en-US')
}

function trim(value: number): string {
  return value.toFixed(1).replace(/\.0$/, '')
}

export function phaseLabel(phase: string): string {
  switch (phase) {
    case 'waiting':
      return 'Waiting'
    case 'preflop':
      return 'Pre-flop'
    case 'flop':
      return 'Flop'
    case 'turn':
      return 'Turn'
    case 'river':
      return 'River'
    case 'showdown':
      return 'Showdown'
    case 'complete':
      return 'Hand over'
    default:
      return phase
  }
}
