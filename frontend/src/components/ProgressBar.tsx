type Color = 'teal' | 'grass' | 'amber' | 'coral'

interface Props {
  pct: number   // 0-100
  color?: Color
  height?: number
}

const fillCls: Record<Color, string> = {
  teal:  'pf-teal',
  grass: 'pf-grass',
  amber: 'pf-amber',
  coral: 'pf-coral',
}

export function ProgressBar({ pct, color = 'teal', height = 6 }: Props) {
  return (
    <div className="pbar" style={{ height }}>
      <div
        className={`pfill ${fillCls[color]}`}
        style={{ width: `${Math.min(100, Math.max(0, pct))}%` }}
      />
    </div>
  )
}
