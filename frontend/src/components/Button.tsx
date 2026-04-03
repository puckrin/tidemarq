import type { ButtonHTMLAttributes, ReactNode } from 'react'

type Variant = 'primary' | 'secondary' | 'danger' | 'ghost'
type Size = 'md' | 'sm'

interface Props extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  children: ReactNode
}

const variantStyles: Record<Variant, string> = {
  primary:   'btn-primary',
  secondary: 'btn-secondary',
  danger:    'btn-danger',
  ghost:     'btn-ghost',
}

export function Button({ variant = 'primary', size = 'md', children, style, className = '', ...rest }: Props) {
  const cls = [
    'btn',
    variantStyles[variant],
    size === 'sm' ? 'btn-sm' : '',
    className,
  ].filter(Boolean).join(' ')

  return (
    <button className={cls} style={style} {...rest}>
      {children}
    </button>
  )
}
