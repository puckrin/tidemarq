import { useState, useEffect } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './store/auth'
import { ToastProvider } from './components/Toast'
import { JobProgressProvider } from './store/jobProgress'
import { Sidebar, type View } from './components/Sidebar'
import { useTheme } from './hooks/useTheme'
import { useQuery } from '@tanstack/react-query'
import { listConflicts, listQuarantine } from './api/client'

import { LoginView }     from './views/LoginView'
import { DashboardView } from './views/DashboardView'
import { JobsView }      from './views/JobsView'
import { JobDetailView } from './views/JobDetailView'
import { NewJobView }    from './views/NewJobView'
import { ConflictsView }   from './views/ConflictsView'
import { QuarantineView }  from './views/QuarantineView'
import { AuditView }     from './views/AuditView'
import { MountsView }    from './views/MountsView'
import { SettingsView }  from './views/SettingsView'

import './styles/global.css'

const qc = new QueryClient({
  defaultOptions: { queries: { retry: 1, staleTime: 5000 } },
})

function Shell() {
  const { user }  = useAuth()
  const { theme, toggle } = useTheme()
  const [view, setView]   = useState<View>('dashboard')
  const [jobId, setJobId] = useState<number | undefined>()
  const [authed, setAuthed] = useState(!!user)

  useEffect(() => { setAuthed(!!user) }, [user])

  const { data: conflicts = [] } = useQuery({
    queryKey: ['conflicts'],
    queryFn: () => listConflicts(),
    refetchInterval: 30000,
    enabled: authed,
  })

  const pendingConflicts = conflicts.filter(c => c.status === 'pending').length

  const { data: quarantine = [] } = useQuery({
    queryKey: ['quarantine'],
    queryFn: () => listQuarantine(),
    refetchInterval: 60000,
    enabled: authed,
  })
  const quarantineCount = quarantine.length

  const nav = (v: View, id?: number) => {
    setView(v)
    if (id != null) setJobId(id)
  }

  if (!authed) {
    return <LoginView onLogin={() => setAuthed(true)} />
  }

  return (
    <JobProgressProvider>
      <div style={{ display: 'flex', width: '100%', height: '100%' }}>
        <Sidebar current={view} onNav={nav} conflictCount={pendingConflicts} quarantineCount={quarantineCount} />
        <div className="main">
          <div className="page">
            {view === 'dashboard'  && <DashboardView onNav={nav} />}
            {view === 'jobs'       && <JobsView onNav={nav} />}
            {view === 'new-job'    && <NewJobView onNav={nav} />}
            {view === 'edit-job'   && jobId != null && <NewJobView onNav={nav} editJobId={jobId} />}
            {view === 'job-detail' && jobId != null && <JobDetailView jobId={jobId} onNav={nav} />}
            {view === 'conflicts'  && <ConflictsView onNav={nav} />}
            {view === 'quarantine' && <QuarantineView onNav={nav} />}
            {view === 'audit'      && <AuditView onNav={nav} />}
            {view === 'mounts'     && <MountsView />}
            {view === 'settings'   && <SettingsView theme={theme} onToggleTheme={toggle} />}
          </div>
        </div>
      </div>
    </JobProgressProvider>
  )
}

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <AuthProvider>
        <ToastProvider>
          <Shell />
        </ToastProvider>
      </AuthProvider>
    </QueryClientProvider>
  )
}
