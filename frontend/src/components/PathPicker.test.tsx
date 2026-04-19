import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { PathPicker } from './PathPicker'
import type { PathValue } from './PathPicker'

vi.mock('../api/client', () => ({
  listMounts: vi.fn().mockResolvedValue([
    { id: 1, name: 'NAS', type: 'smb' },
  ]),
  browseDir: vi.fn().mockResolvedValue({
    path: '/data',
    entries: [
      { name: 'projects', is_dir: true },
      { name: 'readme.txt', is_dir: false },
    ],
  }),
}))

import { browseDir } from '../api/client'
const mockBrowse = browseDir as ReturnType<typeof vi.fn>

function wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

const localValue: PathValue = { path: '', mountId: null }

describe('PathPicker', () => {
  it('renders label when provided', () => {
    render(<PathPicker value={localValue} onChange={() => {}} label="Source path" />, { wrapper })
    expect(screen.getByText('Source path')).toBeInTheDocument()
  })

  it('renders without label', () => {
    render(<PathPicker value={localValue} onChange={() => {}} />, { wrapper })
    expect(screen.queryByText('Source path')).toBeNull()
  })

  it('shows Browse button', () => {
    render(<PathPicker value={localValue} onChange={() => {}} />, { wrapper })
    expect(screen.getByRole('button', { name: /Browse/i })).toBeInTheDocument()
  })

  it('calls onChange when path input changes', () => {
    const onChange = vi.fn()
    render(<PathPicker value={localValue} onChange={onChange} />, { wrapper })
    fireEvent.change(screen.getByRole('combobox'), { target: { value: '' } })
    // typing in the text input triggers onChange
    const input = screen.getByRole('textbox')
    fireEvent.change(input, { target: { value: '/home/user' } })
    expect(onChange).toHaveBeenCalledWith({ path: '/home/user', mountId: null })
  })

  it('shows path validation error for relative path on local FS', () => {
    render(<PathPicker value={{ path: 'relative/path', mountId: null }} onChange={() => {}} />, { wrapper })
    expect(screen.getByText(/Path must be absolute/i)).toBeInTheDocument()
  })

  it('does not show error for valid absolute path', () => {
    render(<PathPicker value={{ path: '/valid/path', mountId: null }} onChange={() => {}} />, { wrapper })
    expect(screen.queryByText(/Path must be absolute/i)).toBeNull()
  })

  it('opens browser when Browse clicked', async () => {
    render(<PathPicker value={{ path: '/data', mountId: null }} onChange={() => {}} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Browse/i }))
    await waitFor(() => expect(screen.getByText('Browse local filesystem')).toBeInTheDocument())
  })

  it('shows directory entries in browser', async () => {
    render(<PathPicker value={{ path: '/data', mountId: null }} onChange={() => {}} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Browse/i }))
    await waitFor(() => expect(screen.getByText('projects')).toBeInTheDocument())
    expect(screen.getByText('readme.txt')).toBeInTheDocument()
  })

  it('calls onChange and closes browser when "Use this folder" clicked', async () => {
    const onChange = vi.fn()
    render(<PathPicker value={{ path: '/data', mountId: null }} onChange={onChange} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Browse/i }))
    await waitFor(() => screen.getByText('Use this folder'))
    fireEvent.click(screen.getByText('Use this folder'))
    expect(onChange).toHaveBeenCalledWith({ path: '/data', mountId: null })
    expect(screen.queryByText('Browse local filesystem')).toBeNull()
  })

  it('closes browser when Cancel clicked', async () => {
    render(<PathPicker value={{ path: '/data', mountId: null }} onChange={() => {}} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Browse/i }))
    await waitFor(() => screen.getByText('Cancel'))
    fireEvent.click(screen.getByText('Cancel'))
    expect(screen.queryByText('Browse local filesystem')).toBeNull()
  })

  it('navigates into a directory on click', async () => {
    render(<PathPicker value={{ path: '/data', mountId: null }} onChange={() => {}} />, { wrapper })
    fireEvent.click(screen.getByRole('button', { name: /Browse/i }))
    await waitFor(() => screen.getByText('projects'))
    fireEvent.click(screen.getByText('projects'))
    await waitFor(() => expect(mockBrowse).toHaveBeenCalledWith('/data/projects', undefined))
  })
})
