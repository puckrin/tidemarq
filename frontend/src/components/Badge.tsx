type BadgeVariant =
  | 'running'
  | 'synced'
  | 'pending'
  | 'error'
  | 'disabled'
  | 'ignored'
  | 'tag'

interface Props {
  variant: BadgeVariant
  children: React.ReactNode
}

const cls: Record<BadgeVariant, string> = {
  running:  'b-running',
  synced:   'b-synced',
  pending:  'b-pending',
  error:    'b-error',
  disabled: 'b-disabled',
  ignored:  'b-ignored',
  tag:      'b-tag',
}

export function Badge({ variant, children }: Props) {
  return (
    <span className={`badge ${cls[variant]}`}>
      <span className="dot" />
      {children}
    </span>
  )
}

export function TagBadge({ children }: { children: React.ReactNode }) {
  return <span className="badge b-tag">{children}</span>
}

import React from 'react'
