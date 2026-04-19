import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ToastProvider } from './Toast'
import { QuarantineCard } from './QuarantineCard'
import type { QuarantineEntry } from '../api/types'

vi.mock('../api/client', () => ({
  restoreQuarantine:    vi.fn().mockResolvedValue({}),
  deleteQuarantineEntry: vi.fn().mockResolvedValue({}),
}))

import { restoreQuarantine, deleteQuarantineEntry } from '../api/client'
const mockRestore = restoreQuarantine as ReturnType<typeof vi.fn>
const mockDelete  = deleteQuarantineEntry as ReturnType<typeof vi.fn>

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return (
    <QueryClientProvider client={qc}>
      <ToastProvider>{children}</ToastProvider>
    </QueryClientProvider>
  )
}

function makeEntry(overrides?: Partial<QuarantineEntry>): QuarantineEntry {
  const far = new Date(Date.now() + 30 * 86400000).toISOString() // 30 days out
  return {
    id: 1,
    job_id: 1,
    rel_path: 'images/photo.jpg',
    quarantine_path: '/quarantine/1/photo.jpg',
    content_hash: 'abc123',
    hash_algo: 'blake3',
    size_bytes: 204800,
    deleted_at: '2024-01-01T10:00:00Z',
    expires_at: far,
    status: 'active',
    removed_at: null,
    ...overrides,
  }
}

describe('QuarantineCard', () => {
  beforeEach(() => {
    mockRestore.mockClear()
    mockDelete.mockClear()
  })

  it('renders nothing when entries array is empty', () => {
    render(<QuarantineCard entries={[]} onChanged={() => {}} />, { wrapper })
    expect(screen.queryByRole('table')).toBeNull()
  })

  it('renders a row for each entry', () => {
    const entries = [
      makeEntry({ id: 1, rel_path: 'a/file1.txt' }),
      makeEntry({ id: 2, rel_path: 'b/file2.txt' }),
    ]
    render(<QuarantineCard entries={entries} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText('a/file1.txt')).toBeInTheDocument()
    expect(screen.getByText('b/file2.txt')).toBeInTheDocument()
  })

  it('shows singular count text for one entry', () => {
    render(<QuarantineCard entries={[makeEntry()]} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText(/1 file held in quarantine/)).toBeInTheDocument()
  })

  it('shows plural count text for multiple entries', () => {
    render(<QuarantineCard entries={[makeEntry({ id: 1 }), makeEntry({ id: 2 })]} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText(/2 files held in quarantine/)).toBeInTheDocument()
  })

  it('formats file size correctly', () => {
    render(<QuarantineCard entries={[makeEntry({ size_bytes: 204800 })]} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText('200.0 KB')).toBeInTheDocument()
  })

  it('shows normal expiry label for entries more than 7 days out', () => {
    const far = new Date(Date.now() + 20 * 86400000).toISOString()
    render(<QuarantineCard entries={[makeEntry({ expires_at: far })]} onChanged={() => {}} />, { wrapper })
    // Should show a "Nd" label — not bold/critical
    const cell = screen.getByText(/^\d+d$/)
    expect(cell).toBeInTheDocument()
    expect(cell.style.color).not.toBe('var(--coral-light)')
  })

  it('shows critical styling for entries expiring in less than 3 days', () => {
    const soon = new Date(Date.now() + 2 * 86400000).toISOString()
    render(<QuarantineCard entries={[makeEntry({ expires_at: soon })]} onChanged={() => {}} />, { wrapper })
    const cell = screen.getByText(/^\d+d \d+h$/)
    // urgent=true sets fontWeight:600 — a concrete value jsdom preserves reliably.
    // (CSS variable references like var(--coral-light) are dropped by jsdom.)
    expect(cell.style.fontWeight).toBe('600')
  })

  it('shows Expired label for entries past their expiry', () => {
    const past = new Date(Date.now() - 1000).toISOString()
    render(<QuarantineCard entries={[makeEntry({ expires_at: past })]} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText('Expired')).toBeInTheDocument()
  })

  it('restore button calls restoreQuarantine with the entry id', async () => {
    const onChanged = vi.fn()
    render(<QuarantineCard entries={[makeEntry({ id: 99 })]} onChanged={onChanged} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Restore$/ }))
    await waitFor(() => expect(mockRestore).toHaveBeenCalledWith(99))
  })

  it('delete button calls deleteQuarantineEntry with the entry id', async () => {
    render(<QuarantineCard entries={[makeEntry({ id: 55 })]} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Delete$/ }))
    await waitFor(() => expect(mockDelete).toHaveBeenCalledWith(55))
  })

  it('delete all button opens the confirmation modal', () => {
    render(<QuarantineCard entries={[makeEntry({ id: 1 }), makeEntry({ id: 2 })]} onChanged={() => {}} />, { wrapper })
    expect(screen.queryByText(/permanently remove all/i)).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: /Delete all/ }))
    expect(screen.getByText(/permanently remove all/i)).toBeInTheDocument()
  })

  it('confirming the delete all modal calls deleteQuarantineEntry for each entry', async () => {
    const entries = [makeEntry({ id: 10 }), makeEntry({ id: 20 })]
    render(<QuarantineCard entries={entries} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Delete all/ }))
    // Two "Delete all" buttons now exist: the toolbar button and the modal confirm.
    // The modal confirm is the last one in DOM order.
    const deleteAllButtons = screen.getAllByRole('button', { name: 'Delete all' })
    fireEvent.click(deleteAllButtons[deleteAllButtons.length - 1]!)
    await waitFor(() => {
      expect(mockDelete).toHaveBeenCalledWith(10)
      expect(mockDelete).toHaveBeenCalledWith(20)
    })
  })

  it('restore all button calls restoreQuarantine for each entry', async () => {
    const entries = [makeEntry({ id: 11 }), makeEntry({ id: 22 })]
    render(<QuarantineCard entries={entries} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Restore all/ }))
    await waitFor(() => {
      expect(mockRestore).toHaveBeenCalledWith(11)
      expect(mockRestore).toHaveBeenCalledWith(22)
    })
  })
})
