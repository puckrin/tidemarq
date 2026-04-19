import { render, screen } from '@testing-library/react'
import { Card, CardHeader } from './Card'

describe('Card', () => {
  it('renders children', () => {
    render(<Card>hello</Card>)
    expect(screen.getByText('hello')).toBeInTheDocument()
  })

  it('includes card class', () => {
    const { container } = render(<Card>x</Card>)
    expect(container.firstChild).toHaveClass('card')
  })

  it('adds p0 class when noPad is set', () => {
    const { container } = render(<Card noPad>x</Card>)
    expect(container.firstChild).toHaveClass('p0')
  })

  it('does not add p0 class by default', () => {
    const { container } = render(<Card>x</Card>)
    expect(container.firstChild).not.toHaveClass('p0')
  })

  it('merges custom className', () => {
    const { container } = render(<Card className="my-card">x</Card>)
    expect(container.firstChild).toHaveClass('my-card')
  })

  it('applies custom style', () => {
    const { container } = render(<Card style={{ color: 'red' }}>x</Card>)
    expect((container.firstChild as HTMLElement).style.color).toBe('red')
  })
})

describe('CardHeader', () => {
  it('renders title', () => {
    render(<CardHeader title="My Title" />)
    expect(screen.getByText('My Title')).toBeInTheDocument()
  })

  it('renders action when provided', () => {
    render(<CardHeader title="T" action={<button>Edit</button>} />)
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument()
  })

  it('renders without action', () => {
    render(<CardHeader title="T" />)
    expect(screen.getByText('T')).toBeInTheDocument()
  })
})
