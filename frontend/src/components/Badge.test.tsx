import { render, screen } from '@testing-library/react'
import { Badge, TagBadge } from './Badge'

describe('Badge', () => {
  const variants = ['running','synced','pending','error','disabled','ignored','tag'] as const

  variants.forEach(v => {
    it(`renders ${v} variant with correct class`, () => {
      render(<Badge variant={v}>{v}</Badge>)
      const el = screen.getByText(v).closest('.badge')
      expect(el).toBeTruthy()
      expect(el!.className).toContain(`b-${v}`)
    })
  })

  it('renders children text', () => {
    render(<Badge variant="synced">Synced</Badge>)
    expect(screen.getByText('Synced')).toBeInTheDocument()
  })

  it('includes a dot span', () => {
    const { container } = render(<Badge variant="running">Running</Badge>)
    expect(container.querySelector('.dot')).toBeTruthy()
  })
})

describe('TagBadge', () => {
  it('renders with b-tag class', () => {
    const { container } = render(<TagBadge>*.tmp</TagBadge>)
    expect(container.querySelector('.b-tag')).toBeTruthy()
    expect(screen.getByText('*.tmp')).toBeInTheDocument()
  })
})
