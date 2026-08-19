import type { ReactNode } from 'react'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'gold'
type Size = 'sm' | 'md' | 'lg'

interface ButtonProps {
  children: ReactNode
  onClick?: () => void
  variant?: Variant
  size?: Size
  block?: boolean
  pill?: boolean
  disabled?: boolean
  type?: 'button' | 'submit'
  className?: string
}

const VARIANT_CLASS: Record<Variant, string> = {
  primary: 'btn-primary',
  secondary: 'btn-secondary',
  ghost: 'btn-ghost',
  danger: 'btn-danger',
  gold: 'btn-gold',
}

export default function Button({
  children,
  onClick,
  variant = 'primary',
  size = 'md',
  block = false,
  pill = false,
  disabled = false,
  type = 'button',
  className = '',
}: ButtonProps) {
  const classes = ['btn', VARIANT_CLASS[variant]]
  if (size === 'sm') classes.push('btn-sm')
  if (size === 'lg') classes.push('btn-lg')
  if (block) classes.push('btn-block')
  if (pill) classes.push('btn-pill')
  if (className) classes.push(className)

  return (
    <button type={type} className={classes.join(' ')} onClick={onClick} disabled={disabled}>
      {children}
    </button>
  )
}
