import { render, screen, fireEvent } from '@testing-library/react'
import { Modal } from './Modal'

describe('Modal', () => {
  const base = {
    title: 'Confirm action',
    body: 'Are you sure?',
    onConfirm: vi.fn(),
    onClose: vi.fn(),
  }

  it('renders nothing when closed', () => {
    render(<Modal {...base} open={false} />)
    expect(screen.queryByText('Confirm action')).toBeNull()
  })

  it('renders title and body when open', () => {
    render(<Modal {...base} open={true} />)
    expect(screen.getByText('Confirm action')).toBeInTheDocument()
    expect(screen.getByText('Are you sure?')).toBeInTheDocument()
  })

  it('calls onConfirm when confirm button clicked', () => {
    const onConfirm = vi.fn()
    render(<Modal {...base} open={true} onConfirm={onConfirm} />)
    fireEvent.click(screen.getByText('Confirm'))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('calls onClose when cancel button clicked', () => {
    const onClose = vi.fn()
    render(<Modal {...base} open={true} onClose={onClose} />)
    fireEvent.click(screen.getByText('Cancel'))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('uses custom confirm label', () => {
    render(<Modal {...base} open={true} confirmLabel="Delete" />)
    expect(screen.getByText('Delete')).toBeInTheDocument()
  })

  it('closes when overlay backdrop clicked', () => {
    const onClose = vi.fn()
    const { container } = render(<Modal {...base} open={true} onClose={onClose} />)
    fireEvent.click(container.querySelector('.overlay')!)
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('does not close when modal content clicked', () => {
    const onClose = vi.fn()
    const { container } = render(<Modal {...base} open={true} onClose={onClose} />)
    fireEvent.click(container.querySelector('.modal')!)
    expect(onClose).not.toHaveBeenCalled()
  })
})
