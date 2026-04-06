import type { ReactNode, CSSProperties } from 'react'

interface CardProps {
  children: ReactNode
  style?: CSSProperties
  className?: string
  noPad?: boolean
}

export function Card({ children, style, className = '', noPad }: CardProps) {
  return (
    <div className={`card ${noPad ? 'p0' : ''} ${className}`} style={style}>
      {children}
    </div>
  )
}

interface CardHeaderProps {
  title: ReactNode
  action?: ReactNode
}

export function CardHeader({ title, action }: CardHeaderProps) {
  return (
    <div className="card-hd">
      <div className="card-title">{title}</div>
      {action}
    </div>
  )
}
