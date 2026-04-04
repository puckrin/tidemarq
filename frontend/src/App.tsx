import { useState, useEffect, useMemo } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { AuthProvider, useAuth } from './store/auth'
import { ToastProvider } from './components/Toast'
import { AuditLogProvider } from './store/auditLog'
import { JobProgressProvider } from './store/jobProgress'
import { Sidebar, type View } from './components/Sidebar'
import { Topbar } from './components/Topbar'
import { useTheme } from './hooks/useTheme'
import { useQuery } from '@tanstack/react-query'
import { listConflicts, listJobs } from './api/client'

import { LoginView }     from './views/LoginView'
import { DashboardView } from './views/DashboardView'
import { JobsView }      from './views/JobsView'
import { JobDetailView } from './views/JobDetailView'
import { NewJobView }    from './views/NewJobView'
import { ConflictsView } from './views/ConflictsView'
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

  const { data: jobs = [] } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
    refetchInterval: 30000,
    enabled: authed,
  })

  const { data: conflicts = [] } = useQuery({
    queryKey: ['conflicts'],
    queryFn: () => listConflicts(),
    refetchInterval: 30000,
    enabled: authed,
  })

  const pendingConflicts = conflicts.filter(c => c.status === 'pending').length

  // Build a stable jobId→name map for the audit log
  const jobNames = useMemo(() => {
    const m: Record<number, string> = {}
    jobs.forEach(j => { m[j.id] = j.name })
    return m
  }, [jobs])

  const nav = (v: View, id?: number) => {
    setView(v)
    if (id != null) setJobId(id)
  }

  if (!authed) {
    return <LoginView onLogin={() => setAuthed(true)} />
  }

  return (
    <JobProgressProvider>
    <AuditLogProvider jobNames={jobNames}>
      <div style={{ display: 'flex', width: '100%', height: '100%' }}>
        <Sidebar current={view} onNav={nav} conflictCount={pendingConflicts} />
        <div className="main">
          <Topbar theme={theme} onToggleTheme={toggle} />
          <div className="page">
            {view === 'dashboard'  && <DashboardView onNav={nav} />}
            {view === 'jobs'       && <JobsView onNav={nav} />}
            {view === 'new-job'    && <NewJobView onNav={nav} />}
            {view === 'edit-job'   && jobId != null && <NewJobView onNav={nav} editJobId={jobId} />}
            {view === 'job-detail' && jobId != null && <JobDetailView jobId={jobId} onNav={nav} />}
            {view === 'conflicts'  && <ConflictsView />}
            {view === 'audit'      && <AuditView onNav={nav} />}
            {view === 'mounts'     && <MountsView />}
            {view === 'settings'   && <SettingsView />}
          </div>
        </div>
      </div>
    </AuditLogProvider>
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
