type CardSize = 'xs' | 'sm' | 'md' | 'lg'

interface CardViewProps {
  code: string
  hidden?: boolean
  size?: CardSize
  dimmed?: boolean
  highlight?: boolean
  /** Deal-in animation, for cards arriving on the table. */
  animate?: boolean
  /** Flip-in animation, for cards turned face up at showdown. */
  reveal?: boolean
  /** Stagger, in ms, so a street lands card by card instead of all at once. */
  delay?: number
}

const SUIT_CLASS: Record<string, string> = {
  '♠': 'pcard-s',
  '♥': 'pcard-h',
  '♦': 'pcard-d',
  '♣': 'pcard-c',
}

export default function CardView({
  code,
  hidden = false,
  size = 'md',
  dimmed = false,
  highlight = false,
  animate = false,
  reveal = false,
  delay = 0,
}: CardViewProps) {
  const base = ['pcard', `pcard-${size}`]
  if (dimmed) base.push('pcard-muted')
  if (highlight) base.push('pcard-win')
  if (reveal) base.push('pcard-reveal')
  else if (animate) base.push('pcard-deal')
  const style = delay > 0 ? { animationDelay: `${delay}ms` } : undefined

  const suit = code.length >= 2 ? code.slice(1) : ''
  const suitClass = SUIT_CLASS[suit]

  if (hidden || !code || !suitClass) {
    return <div className={[...base, 'pcard-back'].join(' ')} style={style} />
  }

  const raw = code.slice(0, 1)
  const rank = raw === 'T' ? '10' : raw

  return (
    <div className={[...base, suitClass].join(' ')} style={style}>
      <span className={`pcard-rank${rank.length > 1 ? ' pcard-rank-wide' : ''}`}>{rank}</span>
      <span className="pcard-suit">{suit}</span>
    </div>
  )
}
