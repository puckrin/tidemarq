import { render } from '@testing-library/react'
import { ProgressBar } from './ProgressBar'

describe('ProgressBar', () => {
  it('renders with correct width percentage', () => {
    const { container } = render(<ProgressBar pct={67} />)
    const fill = container.querySelector('.pfill') as HTMLElement
    expect(fill).toBeTruthy()
    expect(fill.style.width).toBe('67%')
  })

  it('clamps to 100% at maximum', () => {
    const { container } = render(<ProgressBar pct={120} />)
    const fill = container.querySelector('.pfill') as HTMLElement
    expect(fill.style.width).toBe('100%')
  })

  it('clamps to 0% at minimum', () => {
    const { container } = render(<ProgressBar pct={-10} />)
    const fill = container.querySelector('.pfill') as HTMLElement
    expect(fill.style.width).toBe('0%')
  })

  it('applies teal color class by default', () => {
    const { container } = render(<ProgressBar pct={50} />)
    const fill = container.querySelector('.pfill')!
    expect(fill.className).toContain('pf-teal')
  })

  it('applies grass color class', () => {
    const { container } = render(<ProgressBar pct={50} color="grass" />)
    expect(container.querySelector('.pfill')!.className).toContain('pf-grass')
  })

  it('applies amber color class', () => {
    const { container } = render(<ProgressBar pct={50} color="amber" />)
    expect(container.querySelector('.pfill')!.className).toContain('pf-amber')
  })

  it('applies coral color class', () => {
    const { container } = render(<ProgressBar pct={50} color="coral" />)
    expect(container.querySelector('.pfill')!.className).toContain('pf-coral')
  })

  it('respects custom height', () => {
    const { container } = render(<ProgressBar pct={50} height={10} />)
    const bar = container.querySelector('.pbar') as HTMLElement
    expect(bar.style.height).toBe('10px')
  })
})
