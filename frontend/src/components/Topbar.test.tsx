import { render, screen, fireEvent } from '@testing-library/react'
import { Topbar } from './Topbar'

describe('Topbar', () => {
  it('renders children', () => {
    render(<Topbar theme="dark" onToggleTheme={() => {}}>Page title</Topbar>)
    expect(screen.getByText('Page title')).toBeInTheDocument()
  })

  it('renders without children', () => {
    const { container } = render(<Topbar theme="dark" onToggleTheme={() => {}} />)
    expect(container.firstChild).toBeInTheDocument()
  })

  it('calls onToggleTheme when button clicked', () => {
    const fn = vi.fn()
    render(<Topbar theme="dark" onToggleTheme={fn} />)
    fireEvent.click(screen.getByTitle('Toggle theme'))
    expect(fn).toHaveBeenCalledTimes(1)
  })

  it('renders toggle button for dark theme', () => {
    render(<Topbar theme="dark" onToggleTheme={() => {}} />)
    expect(screen.getByTitle('Toggle theme')).toBeInTheDocument()
  })

  it('renders toggle button for light theme', () => {
    render(<Topbar theme="light" onToggleTheme={() => {}} />)
    expect(screen.getByTitle('Toggle theme')).toBeInTheDocument()
  })
})
