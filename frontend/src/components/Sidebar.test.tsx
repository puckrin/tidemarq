import { render, screen, fireEvent } from '@testing-library/react'
import { Sidebar } from './Sidebar'

vi.mock('../store/auth', () => ({
  useAuth: () => ({
    user: { id: 1, username: 'admin', role: 'admin' },
    logout: vi.fn(),
  }),
}))

const base = {
  current: 'dashboard' as const,
  onNav: vi.fn(),
  conflictCount: 0,
  quarantineCount: 0,
}

describe('Sidebar', () => {
  beforeEach(() => base.onNav.mockClear())

  it('renders all nav section labels', () => {
    render(<Sidebar {...base} />)
    expect(screen.getByText('Dashboard')).toBeInTheDocument()
    expect(screen.getByText('Sync Jobs')).toBeInTheDocument()
    expect(screen.getByText('Conflicts')).toBeInTheDocument()
    expect(screen.getByText('Quarantine')).toBeInTheDocument()
    expect(screen.getByText('Audit Log')).toBeInTheDocument()
    expect(screen.getByText('Settings')).toBeInTheDocument()
  })

  it('marks the current view as active', () => {
    render(<Sidebar {...base} current="jobs" />)
    const jobsItem = screen.getByText('Sync Jobs').closest('.nav-item')
    expect(jobsItem).toHaveClass('active')
  })

  it('job-detail view marks Sync Jobs as active', () => {
    render(<Sidebar {...base} current="job-detail" />)
    const jobsItem = screen.getByText('Sync Jobs').closest('.nav-item')
    expect(jobsItem).toHaveClass('active')
  })

  it('calls onNav when a nav item is clicked', () => {
    render(<Sidebar {...base} />)
    fireEvent.click(screen.getByText('Settings'))
    expect(base.onNav).toHaveBeenCalledWith('settings')
  })

  it('shows conflict badge when conflictCount > 0', () => {
    render(<Sidebar {...base} conflictCount={3} />)
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('hides conflict badge when conflictCount is 0', () => {
    render(<Sidebar {...base} conflictCount={0} />)
    expect(screen.queryByText('0')).toBeNull()
  })

  it('shows quarantine badge when quarantineCount > 0', () => {
    render(<Sidebar {...base} quarantineCount={7} />)
    expect(screen.getByText('7')).toBeInTheDocument()
  })

  it('shows the logged-in username', () => {
    render(<Sidebar {...base} />)
    const name = document.querySelector('.user-name')
    expect(name?.textContent).toBe('admin')
  })

  it('opens sign-out modal when sign-out button clicked', () => {
    render(<Sidebar {...base} />)
    fireEvent.click(screen.getByTitle('Sign out'))
    expect(screen.getByText('Sign out?')).toBeInTheDocument()
  })

  it('navigates to dashboard when logo clicked', () => {
    render(<Sidebar {...base} current="settings" />)
    fireEvent.click(screen.getByText('tidemarq'))
    expect(base.onNav).toHaveBeenCalledWith('dashboard')
  })
})
