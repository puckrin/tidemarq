import { LayoutDashboard, RefreshCw, GitMerge, ScrollText, Settings, LogOut, HardDrive, Archive } from 'lucide-react'
import { useState } from 'react'
import { useAuth } from '../store/auth'
import { Modal } from './Modal'

export type View =
  | 'dashboard'
  | 'jobs'
  | 'new-job'
  | 'edit-job'
  | 'job-detail'
  | 'conflicts'
  | 'quarantine'
  | 'audit'
  | 'mounts'
  | 'settings'

interface Props {
  current: View
  onNav: (v: View) => void
  conflictCount: number
  quarantineCount: number
}

interface NavItem {
  view: View
  label: string
  icon: React.ReactNode
}

const overviewItems: NavItem[] = [
  { view: 'dashboard', label: 'Dashboard', icon: <LayoutDashboard size={18} /> },
  { view: 'jobs',      label: 'Sync Jobs', icon: <RefreshCw size={18} /> },
]

const manageItems: NavItem[] = [
  { view: 'conflicts',  label: 'Conflicts',  icon: <GitMerge size={18} /> },
  { view: 'quarantine', label: 'Quarantine', icon: <Archive size={18} /> },
  { view: 'audit',      label: 'Audit Log',  icon: <ScrollText size={18} /> },
  { view: 'mounts',     label: 'Mounts',     icon: <HardDrive size={18} /> },
]

const systemItems: NavItem[] = [
  { view: 'settings',  label: 'Settings',  icon: <Settings size={18} /> },
]

import React from 'react'

function initials(name: string) {
  return name.slice(0, 2).toUpperCase()
}

export function Sidebar({ current, onNav, conflictCount, quarantineCount }: Props) {
  const { user, logout } = useAuth()
  const [logoutModal, setLogoutModal] = useState(false)

  const isActive = (v: View) =>
    v === current ||
    (v === 'jobs' && (current === 'new-job' || current === 'edit-job' || current === 'job-detail'))

  const item = (nav: NavItem, badge?: number) => (
    <div
      key={nav.view}
      className={`nav-item${isActive(nav.view) ? ' active' : ''}`}
      onClick={() => onNav(nav.view)}
    >
      {nav.icon}
      {nav.label}
      {badge != null && badge > 0 && (
        <span className="nav-badge">{badge}</span>
      )}
    </div>
  )

  return (
    <>
      <nav className="sidebar">
        <div className="sidebar-logo" onClick={() => onNav('dashboard')}>
          <svg width="42" height="42" viewBox="0 0 36 36" fill="none" style={{ flexShrink: 0 }}>
            <rect x="0" y="27" width="36" height="4" rx="2" fill="#E0F4F7" opacity="0.28"/>
            <rect x="0" y="19" width="27" height="4" rx="2" fill="#E0F4F7" opacity="0.50"/>
            <rect x="0" y="11" width="18" height="4" rx="2" fill="#E0F4F7" opacity="0.75"/>
            <rect x="0" y="3"  width="10" height="4" rx="2" fill="#E0F4F7" opacity="1"/>
          </svg>
          <div>
            <div className="logo-name">tidemarq</div>
            <div className="logo-tag">keep your directories in line</div>
          </div>
        </div>

        <div className="nav-sep">Overview</div>
        {overviewItems.map(n => item(n))}

        <div className="nav-sep">Manage</div>
        {item(manageItems[0], conflictCount)}
        {item(manageItems[1], quarantineCount)}
        {item(manageItems[2])}
        {item(manageItems[3])}

        <div className="nav-sep">System</div>
        {systemItems.map(n => item(n))}

        <div style={{ flex: 1 }} />

        <div className="sidebar-footer">
          <div className="user-row">
            <div className="avatar">{user ? initials(user.username) : '?'}</div>
            <div>
              <div className="user-name">{user?.username ?? '—'}</div>
              <div className="user-role">{user?.role ?? ''}</div>
            </div>
            <button className="icon-btn mla" onClick={() => setLogoutModal(true)} title="Sign out">
              <LogOut size={17} />
            </button>
          </div>
        </div>
      </nav>

      <Modal
        open={logoutModal}
        title="Sign out?"
        body="You will be returned to the login screen. Any running jobs will continue in the background."
        confirmLabel="Sign out"
        confirmVariant="danger"
        onConfirm={() => { logout(); setLogoutModal(false) }}
        onClose={() => setLogoutModal(false)}
      />
    </>
  )
}
