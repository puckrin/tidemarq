import { render, screen } from '@testing-library/react'
import { RefreshCw } from 'lucide-react'
import { StatCard } from './StatCard'

describe('StatCard', () => {
  it('renders label, value and sub text', () => {
    render(<StatCard icon={<RefreshCw/>} color="teal" label="Total Jobs" value={12} sub="8 enabled" />)
    expect(screen.getByText('Total Jobs')).toBeInTheDocument()
    expect(screen.getByText('12')).toBeInTheDocument()
    expect(screen.getByText('8 enabled')).toBeInTheDocument()
  })

  it('renders without sub text', () => {
    render(<StatCard icon={<RefreshCw/>} color="teal" label="Jobs" value={5} />)
    expect(screen.getByText('Jobs')).toBeInTheDocument()
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('applies correct icon color class for grass', () => {
    const { container } = render(<StatCard icon={<RefreshCw/>} color="grass" label="L" value={0} />)
    expect(container.querySelector('.si-grass')).toBeTruthy()
  })

  it('applies correct icon color class for coral', () => {
    const { container } = render(<StatCard icon={<RefreshCw/>} color="coral" label="L" value={0} />)
    expect(container.querySelector('.si-coral')).toBeTruthy()
  })

  it('applies correct icon color class for amber', () => {
    const { container } = render(<StatCard icon={<RefreshCw/>} color="amber" label="L" value={0} />)
    expect(container.querySelector('.si-amber')).toBeTruthy()
  })

  it('applies valueStyle to value element', () => {
    render(<StatCard icon={<RefreshCw/>} color="coral" label="Errors" value={2} valueStyle={{ color: 'red' }} />)
    const val = screen.getByText('2')
    expect(val.style.color).toBe('red')
  })
})
