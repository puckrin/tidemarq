import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { ToastProvider } from './Toast'
import { ConflictCard } from './ConflictCard'
import type { Conflict } from '../api/types'

vi.mock('../api/client', () => ({
  resolveConflict: vi.fn().mockResolvedValue({}),
}))

import { resolveConflict } from '../api/client'
const mockResolve = resolveConflict as ReturnType<typeof vi.fn>

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return (
    <QueryClientProvider client={qc}>
      <ToastProvider>{children}</ToastProvider>
    </QueryClientProvider>
  )
}

function makeConflict(overrides?: Partial<Conflict>): Conflict {
  return {
    id: 1,
    job_id: 1,
    rel_path: 'docs/report.pdf',
    src_content_hash: 'abc123def456abc123def456',
    dest_content_hash: 'xyz789uvw012xyz789uvw012',
    src_hash_algo: 'blake3',
    dest_hash_algo: 'blake3',
    src_mod_time: '2024-01-01T10:00:00Z',
    dest_mod_time: '2024-01-02T10:00:00Z',
    src_size: 1024,
    dest_size: 2048,
    strategy: 'ask-user',
    status: 'pending',
    resolution: null,
    resolved_at: null,
    created_at: '2024-01-03T10:00:00Z',
    ...overrides,
  }
}

describe('ConflictCard', () => {
  beforeEach(() => mockResolve.mockClear())

  it('renders nothing when conflicts array is empty', () => {
    render(<ConflictCard conflicts={[]} onChanged={() => {}} />, { wrapper })
    expect(screen.queryByRole('table')).toBeNull()
  })

  it('renders a row for each conflict', () => {
    const conflicts = [
      makeConflict({ id: 1, rel_path: 'a/file1.txt' }),
      makeConflict({ id: 2, rel_path: 'b/file2.txt' }),
    ]
    render(<ConflictCard conflicts={conflicts} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText('a/file1.txt')).toBeInTheDocument()
    expect(screen.getByText('b/file2.txt')).toBeInTheDocument()
  })

  it('shows singular count text for one conflict', () => {
    render(<ConflictCard conflicts={[makeConflict()]} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText(/1 conflict awaiting/)).toBeInTheDocument()
  })

  it('shows plural count text for multiple conflicts', () => {
    const conflicts = [makeConflict({ id: 1 }), makeConflict({ id: 2 })]
    render(<ConflictCard conflicts={conflicts} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText(/2 conflicts awaiting/)).toBeInTheDocument()
  })

  it('clicking a row opens the detail panel for that conflict', () => {
    render(<ConflictCard conflicts={[makeConflict()]} onChanged={() => {}} />, { wrapper })
    expect(screen.queryByText('Keep source')).toBeNull()
    fireEvent.click(screen.getByText('docs/report.pdf'))
    expect(screen.getByText('Keep source')).toBeInTheDocument()
    expect(screen.getByText('Keep destination')).toBeInTheDocument()
    expect(screen.getByText('Keep both')).toBeInTheDocument()
  })

  it('clicking the selected row again closes the detail panel', () => {
    render(<ConflictCard conflicts={[makeConflict()]} onChanged={() => {}} />, { wrapper })
    // First click opens panel — path appears in both table row and detail header.
    fireEvent.click(screen.getAllByText('docs/report.pdf')[0]!)
    expect(screen.getByText('Keep source')).toBeInTheDocument()
    // Second click on the table row closes the panel.
    fireEvent.click(screen.getAllByText('docs/report.pdf')[0]!)
    expect(screen.queryByText('Keep source')).toBeNull()
  })

  it('detail panel shows the file path', () => {
    render(<ConflictCard conflicts={[makeConflict()]} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByText('docs/report.pdf'))
    // rel_path appears in both the table row and the detail header
    expect(screen.getAllByText('docs/report.pdf').length).toBeGreaterThanOrEqual(2)
  })

  it('detail panel shows truncated hashes', () => {
    render(<ConflictCard conflicts={[makeConflict()]} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByText('docs/report.pdf'))
    // First 16 chars of each hash
    expect(screen.getByText(/abc123def456abc1/)).toBeInTheDocument()
    expect(screen.getByText(/xyz789uvw012xyz7/)).toBeInTheDocument()
  })

  it('keep source button calls resolveConflict with keep-source action', async () => {
    const onChanged = vi.fn()
    render(<ConflictCard conflicts={[makeConflict({ id: 42 })]} onChanged={onChanged} />, { wrapper })
    fireEvent.click(screen.getByText('docs/report.pdf'))
    fireEvent.click(screen.getByText('Keep source'))
    await waitFor(() => expect(mockResolve).toHaveBeenCalledWith(42, 'keep-source'))
  })

  it('keep destination button calls resolveConflict with keep-dest action', async () => {
    render(<ConflictCard conflicts={[makeConflict({ id: 7 })]} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByText('docs/report.pdf'))
    fireEvent.click(screen.getByText('Keep destination'))
    await waitFor(() => expect(mockResolve).toHaveBeenCalledWith(7, 'keep-dest'))
  })

  it('keep both button calls resolveConflict with keep-both action', async () => {
    render(<ConflictCard conflicts={[makeConflict({ id: 3 })]} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByText('docs/report.pdf'))
    fireEvent.click(screen.getByText('Keep both'))
    await waitFor(() => expect(mockResolve).toHaveBeenCalledWith(3, 'keep-both'))
  })

  it('dismiss button closes the detail panel without calling the API', () => {
    render(<ConflictCard conflicts={[makeConflict()]} onChanged={() => {}} />, { wrapper })
    fireEvent.click(screen.getByText('docs/report.pdf'))
    fireEvent.click(screen.getByText('Dismiss'))
    expect(screen.queryByText('Keep source')).toBeNull()
    expect(mockResolve).not.toHaveBeenCalled()
  })

  it('strategy is displayed as a badge in the table', () => {
    render(<ConflictCard conflicts={[makeConflict({ strategy: 'newest-wins' })]} onChanged={() => {}} />, { wrapper })
    expect(screen.getByText('newest-wins')).toBeInTheDocument()
  })
})
