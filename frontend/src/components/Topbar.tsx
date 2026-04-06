import { Moon, Sun } from 'lucide-react'

interface Props {
  theme: 'dark' | 'light'
  onToggleTheme: () => void
  children?: React.ReactNode
}

import React from 'react'

export function Topbar({ theme, onToggleTheme, children }: Props) {
  return (
    <div className="topbar">
      <div style={{ flex: 1 }}>{children}</div>
      <div className="topbar-actions">
        <button className="icon-btn" onClick={onToggleTheme} title="Toggle theme">
          {theme === 'dark' ? <Moon size={16} /> : <Sun size={16} />}
        </button>
      </div>
    </div>
  )
}
