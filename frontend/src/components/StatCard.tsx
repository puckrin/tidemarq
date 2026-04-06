import type { ReactNode, CSSProperties } from 'react'

type IconColor = 'teal' | 'grass' | 'coral' | 'amber'

interface Props {
  icon: ReactNode
  color: IconColor
  label: string
  value: string | number
  sub?: string
  valueStyle?: CSSProperties
  style?: CSSProperties
  onClick?: () => void
}

const colorCls: Record<IconColor, string> = {
  teal:  'si-teal',
  grass: 'si-grass',
  coral: 'si-coral',
  amber: 'si-amber',
}

export function StatCard({ icon, color, label, value, sub, valueStyle, style, onClick }: Props) {
  return (
    <div className="stat-card" style={style} onClick={onClick}>
      <div className={`stat-icon ${colorCls[color]}`}>{icon}</div>
      <div className="stat-label">{label}</div>
      <div className="stat-val" style={valueStyle}>{value}</div>
      {sub && <div className="stat-sub">{sub}</div>}
    </div>
  )
}
