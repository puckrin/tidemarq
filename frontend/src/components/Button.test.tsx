import { render, screen, fireEvent } from '@testing-library/react'
import { Button } from './Button'

describe('Button', () => {
  it('renders children', () => {
    render(<Button>Click me</Button>)
    expect(screen.getByRole('button', { name: 'Click me' })).toBeInTheDocument()
  })

  it('applies primary variant class by default', () => {
    render(<Button>Primary</Button>)
    const btn = screen.getByRole('button')
    expect(btn.className).toContain('btn-primary')
    expect(btn.className).toContain('btn')
  })

  it('applies secondary variant class', () => {
    render(<Button variant="secondary">Secondary</Button>)
    expect(screen.getByRole('button').className).toContain('btn-secondary')
  })

  it('applies danger variant class', () => {
    render(<Button variant="danger">Delete</Button>)
    expect(screen.getByRole('button').className).toContain('btn-danger')
  })

  it('applies ghost variant class', () => {
    render(<Button variant="ghost">Ghost</Button>)
    expect(screen.getByRole('button').className).toContain('btn-ghost')
  })

  it('applies sm size class', () => {
    render(<Button size="sm">Small</Button>)
    expect(screen.getByRole('button').className).toContain('btn-sm')
  })

  it('does not apply btn-sm for md size', () => {
    render(<Button size="md">Normal</Button>)
    expect(screen.getByRole('button').className).not.toContain('btn-sm')
  })

  it('merges custom className', () => {
    render(<Button className="my-class">Classed</Button>)
    expect(screen.getByRole('button').className).toContain('my-class')
  })

  it('calls onClick handler', () => {
    const fn = vi.fn()
    render(<Button onClick={fn}>Click</Button>)
    fireEvent.click(screen.getByRole('button'))
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('is disabled when disabled prop set', () => {
    render(<Button disabled>Disabled</Button>)
    expect(screen.getByRole('button')).toBeDisabled()
  })

  it('does not fire onClick when disabled', () => {
    const fn = vi.fn()
    render(<Button disabled onClick={fn}>Disabled</Button>)
    fireEvent.click(screen.getByRole('button'))
    expect(fn).not.toHaveBeenCalled()
  })
})
