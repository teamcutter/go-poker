interface AvatarProps {
  id: string
  label?: string
  size?: number
  className?: string
}

const HUES = [4, 28, 48, 96, 150, 172, 196, 220, 258, 292, 320, 344]

function hash(value: string): number {
  let h = 0
  for (let i = 0; i < value.length; i++) {
    h = (h * 31 + value.charCodeAt(i)) | 0
  }
  return Math.abs(h)
}

/** One letter per word, max two — "Dev" -> "D", "Ada Lovelace" -> "AL". */
function initialsOf(value: string): string {
  const letters = value
    .split(/[\s._-]+/)
    .map((word) => word.replace(/[^a-zA-Z0-9]/g, '').slice(0, 1))
    .filter(Boolean)
    .slice(0, 2)
    .join('')
  return letters || '?'
}

export default function Avatar({ id, label, size = 30, className = '' }: AvatarProps) {
  const hue = HUES[hash(id) % HUES.length]
  const initials = initialsOf(label ?? id)

  return (
    <div
      className={`avatar ${className}`}
      style={{
        width: size,
        height: size,
        fontSize: Math.round(size * 0.4),
        background: `linear-gradient(150deg, hsl(${hue} 58% 46%), hsl(${(hue + 26) % 360} 56% 34%))`,
      }}
    >
      {initials}
    </div>
  )
}
