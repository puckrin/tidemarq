import { render, screen, act } from '@testing-library/react'
import { ToastProvider, useToast } from './Toast'

function ToastTrigger({ message, kind }: { message: string; kind?: 'ok' | 'err' | 'info' }) {
  const toast = useToast()
  return <button onClick={() => toast(message, kind)}>show</button>
}

function setup(message: string, kind?: 'ok' | 'err' | 'info') {
  render(
    <ToastProvider>
      <ToastTrigger message={message} kind={kind} />
    </ToastProvider>
  )
}

describe('Toast', () => {
  it('renders children without showing any toast initially', () => {
    setup('hello')
    expect(screen.queryByText('hello')).toBeNull()
  })

  it('shows a toast when triggered', () => {
    setup('File synced')
    act(() => { screen.getByRole('button').click() })
    expect(screen.getByText('File synced')).toBeInTheDocument()
  })

  it('shows ok toast', () => {
    setup('Done', 'ok')
    act(() => { screen.getByRole('button').click() })
    const toastEl = screen.getByText('Done').closest('.toast')
    expect(toastEl).toHaveClass('ok')
  })

  it('shows err toast', () => {
    setup('Failed', 'err')
    act(() => { screen.getByRole('button').click() })
    const toastEl = screen.getByText('Failed').closest('.toast')
    expect(toastEl).toHaveClass('err')
  })

  it('shows info toast by default', () => {
    setup('Info message')
    act(() => { screen.getByRole('button').click() })
    const toastEl = screen.getByText('Info message').closest('.toast')
    expect(toastEl).toHaveClass('info')
  })

  it('throws when useToast is used outside ToastProvider', () => {
    const err = console.error
    console.error = () => {}
    expect(() => render(<ToastTrigger message="x" />)).toThrow('useToast must be used within ToastProvider')
    console.error = err
  })
})
