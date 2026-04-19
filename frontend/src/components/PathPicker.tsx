import { useState } from 'react'
import { Folder, FolderOpen, ChevronRight, Home, HardDrive, X, Check, Monitor } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { listMounts, browseDir } from '../api/client'
import { Button } from './Button'
import type { Mount } from '../api/types'

export interface PathValue {
  path: string
  mountId: number | null  // null = local filesystem
}

interface Props {
  value: PathValue
  onChange: (v: PathValue) => void
  label?: string
}

const LOCAL_ID = '__local__'

/** True for Windows drive roots: "C:" or "C:/" */
function isDriveRoot(p: string): boolean {
  return /^[A-Za-z]:\/?$/.test(p)
}

/** True for a bare Windows drive letter: "C:" */
function isDriveLetter(name: string): boolean {
  return /^[A-Za-z]:$/.test(name)
}

/** Returns an error string if the typed path is obviously invalid, or null if OK. */
function pathError(path: string, isMountPath: boolean): string | null {
  if (!path) return null
  if (isMountPath) {
    if (path === '..' || path.startsWith('../')) return 'Path cannot traverse outside the mount root'
    return null
  }
  if (!path.startsWith('/') && !/^[A-Za-z]:[\\/]/.test(path)) {
    return 'Path must be absolute (e.g. /data/files)'
  }
  return null
}

/**
 * For local FS: compute the child path to navigate into.
 * Handles Windows drive letters specially so "C:" → "C:/".
 */
function localChildPath(resolvedParent: string, entryName: string): string {
  if (isDriveLetter(entryName)) return entryName + '/'
  // Strip any trailing slash from parent before joining.
  const base = resolvedParent.replace(/\/+$/, '')
  return base === '' ? `/${entryName}` : `${base}/${entryName}`
}

/**
 * For local FS: compute the parent path to navigate up to.
 * Returns "" (top-level / drive list) when already at a drive root or Unix root.
 */
function localParentPath(resolvedCurrent: string): string {
  if (resolvedCurrent === '' || resolvedCurrent === '/' || isDriveRoot(resolvedCurrent)) {
    return ''
  }
  const lastSlash = resolvedCurrent.lastIndexOf('/')
  if (lastSlash <= 0) return '/'
  const parent = resolvedCurrent.slice(0, lastSlash)
  // If the remaining part is a bare drive letter (e.g. "C:"), step to its root.
  if (isDriveRoot(parent)) return parent + '/'
  return parent
}

/** Build breadcrumb segments [{label, path}] from a resolved local path. */
function localBreadcrumbs(resolvedPath: string): { label: string; path: string }[] {
  if (!resolvedPath) return []
  const parts = resolvedPath.replace(/\\/g, '/').split('/').filter(Boolean)
  return parts.map((part, i, arr) => {
    const rawPath = arr.slice(0, i + 1).join('/')
    // Drive letter: navigate to "C:/"  otherwise prepend "/"
    const navPath = isDriveLetter(rawPath) ? rawPath + '/' : '/' + rawPath
    return { label: part, path: navPath }
  })
}

interface BrowserProps {
  mountId: number | null
  mount: Mount | null
  onSelect: (path: string) => void
  onClose: () => void
  initialPath: string
}

function DirectoryBrowser({ mountId, mount, onSelect, onClose, initialPath }: BrowserProps) {
  const [currentPath, setCurrentPath] = useState<string>(initialPath || '')

  const queryKey = mountId != null
    ? ['browse', 'mount', mountId, currentPath]
    : ['browse', 'local', currentPath]

  const { data, isLoading, isError } = useQuery({
    queryKey,
    queryFn: () => browseDir(currentPath, mountId ?? undefined),
    staleTime: 30_000,
  })

  // Use the backend-resolved absolute path as the authoritative current location.
  // For mount FS the relative path is tracked directly in currentPath.
  const resolvedPath: string = mountId != null ? currentPath : (data?.path ?? currentPath)

  const isTopLevel = resolvedPath === '' || resolvedPath === '/'

  const canGoUp = mountId != null
    ? currentPath !== ''
    : resolvedPath !== '' && !isTopLevel

  // ── navigation ─────────────────────────────────────────────────────────────

  const navigateTo = (path: string) => setCurrentPath(path)

  const navigateUp = () => {
    if (mountId != null) {
      const parts = currentPath.split('/').filter(Boolean)
      parts.pop()
      setCurrentPath(parts.join('/'))
    } else {
      setCurrentPath(localParentPath(resolvedPath))
    }
  }

  /** Single-click a directory row → navigate into it immediately. */
  const navigateInto = (entryName: string) => {
    if (mountId != null) {
      setCurrentPath(currentPath ? `${currentPath}/${entryName}` : entryName)
    } else {
      setCurrentPath(localChildPath(resolvedPath, entryName))
    }
  }

  const handleUseFolder = () => {
    onSelect(resolvedPath)
    onClose()
  }

  // ── breadcrumbs ─────────────────────────────────────────────────────────────

  const crumbs = mountId != null
    ? currentPath.split('/').filter(Boolean).map((p, i, arr) => ({
        label: p,
        path: arr.slice(0, i + 1).join('/'),
      }))
    : localBreadcrumbs(resolvedPath)

  // ── render ──────────────────────────────────────────────────────────────────

  return (
    <div className="overlay open" onClick={onClose}>
      <div className="modal" style={{ maxWidth: 560, width: '100%' }} onClick={e => e.stopPropagation()}>

        {/* Title */}
        <div className="modal-title" style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          {mount ? <HardDrive size={16}/> : <Folder size={16}/>}
          {mount ? `Browse: ${mount.name}` : 'Browse local filesystem'}
        </div>

        {/* Breadcrumb */}
        <div style={{
          display: 'flex', alignItems: 'center', flexWrap: 'wrap', gap: 2,
          padding: '8px 0', fontFamily: 'var(--mono)', fontSize: 12,
        }}>
          <button
            className="btn btn-ghost"
            style={{ padding: '2px 6px', fontSize: 12, display: 'flex', alignItems: 'center', gap: 4 }}
            onClick={() => navigateTo('')}
          >
            {mountId != null ? <Home size={12}/> : <Monitor size={12}/>}
            {mountId != null ? 'Root' : 'Drives'}
          </button>
          {crumbs.map((seg, i) => (
            <span key={i} style={{ display: 'flex', alignItems: 'center', gap: 2 }}>
              <ChevronRight size={12} style={{ color: 'var(--text3)', flexShrink: 0 }}/>
              <button
                className="btn btn-ghost"
                style={{ padding: '2px 6px', fontSize: 12 }}
                onClick={() => navigateTo(seg.path)}
              >
                {seg.label}
              </button>
            </span>
          ))}
        </div>

        {/* Directory listing */}
        <div style={{
          border: '1px solid var(--input-border)',
          borderRadius: 'var(--radius)',
          background: 'var(--input-bg)',
          minHeight: 220,
          maxHeight: 340,
          overflowY: 'auto',
        }}>
          {isLoading && (
            <div className="text3" style={{ padding: 16, fontSize: 13 }}>Loading…</div>
          )}
          {isError && (
            <div style={{ padding: 16, fontSize: 13, color: 'var(--coral-light)' }}>
              Failed to load — check the path and permissions.
            </div>
          )}
          {data && data.entries.length === 0 && !canGoUp && (
            <div className="text3" style={{ padding: 16, fontSize: 13 }}>Empty</div>
          )}

          {/* ".." up row */}
          {canGoUp && data && (
            <div
              style={{
                display: 'flex', alignItems: 'center', gap: 10,
                padding: '8px 14px', cursor: 'pointer', fontSize: 13,
                borderBottom: '1px solid var(--border)',
              }}
              onClick={navigateUp}
            >
              <Folder size={14} style={{ color: 'var(--text3)', flexShrink: 0 }}/>
              <span className="mono" style={{ color: 'var(--text2)' }}>..</span>
            </div>
          )}

          {/* Directory and file entries */}
          {data?.entries.map(entry => (
            <div
              key={entry.name}
              style={{
                display: 'flex', alignItems: 'center', gap: 10,
                padding: '8px 14px', fontSize: 13,
                cursor: entry.is_dir ? 'pointer' : 'default',
                borderBottom: '1px solid var(--border)',
              }}
              onClick={() => { if (entry.is_dir) navigateInto(entry.name) }}
            >
              {entry.is_dir
                ? <FolderOpen size={14} style={{ color: 'var(--accent)', flexShrink: 0 }}/>
                : <span style={{ width: 14, flexShrink: 0 }}/>
              }
              <span
                className="mono"
                style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}
              >
                {entry.name}
              </span>
              {entry.is_dir && (
                <ChevronRight size={12} style={{ color: 'var(--text3)', flexShrink: 0 }}/>
              )}
            </div>
          ))}
        </div>

        {/* Current path status */}
        <div style={{ marginTop: 8, fontSize: 12, color: 'var(--text3)' }}>
          {isTopLevel && mountId == null
            ? 'Click a drive to open it, then navigate to your folder'
            : <>Current folder: <span className="mono">{resolvedPath || '/'}</span></>}
        </div>

        <div className="modal-acts">
          <Button variant="ghost" onClick={onClose}><X size={14}/> Cancel</Button>
          <Button
            variant="primary"
            disabled={isTopLevel && mountId == null}
            onClick={handleUseFolder}
          >
            <Check size={14}/> Use this folder
          </Button>
        </div>
      </div>
    </div>
  )
}

// ── PathPicker (outer control) ────────────────────────────────────────────────

export function PathPicker({ value, onChange, label }: Props) {
  const [browsing, setBrowsing] = useState(false)

  const { data: mounts = [] } = useQuery({
    queryKey: ['mounts'],
    queryFn: listMounts,
    staleTime: 60_000,
  })

  const locationId = value.mountId != null ? String(value.mountId) : LOCAL_ID

  const setLocation = (id: string) => {
    onChange(id === LOCAL_ID
      ? { path: '', mountId: null }
      : { path: '', mountId: Number(id) })
  }

  const selectedMount = mounts.find((m: Mount) => m.id === value.mountId) ?? null

  return (
    <div>
      {label && <label className="fl">{label}</label>}
      <div style={{ display: 'flex', gap: 8, alignItems: 'stretch' }}>

        {/* Location selector */}
        <select
          className="fs"
          style={{ flexShrink: 0, minWidth: 160, maxWidth: 200 }}
          value={locationId}
          onChange={e => setLocation(e.target.value)}
        >
          <option value={LOCAL_ID}>Local filesystem</option>
          {mounts.map((m: Mount) => (
            <option key={m.id} value={String(m.id)}>{m.name}</option>
          ))}
        </select>

        {/* Path text input */}
        <input
          className="fi mono"
          style={{ flex: 1, fontSize: 13 }}
          placeholder={value.mountId != null ? '/path/within/mount (optional)' : '/local/path'}
          value={value.path}
          onChange={e => onChange({ ...value, path: e.target.value })}
        />

        {/* Browse button */}
        <Button variant="secondary" style={{ flexShrink: 0 }} onClick={() => setBrowsing(true)}>
          <Folder size={14}/> Browse
        </Button>
      </div>
      {pathError(value.path, value.mountId !== null) && (
        <div style={{ color: 'var(--coral)', fontSize: 12, marginTop: 4 }}>
          {pathError(value.path, value.mountId !== null)}
        </div>
      )}

      {browsing && (
        <DirectoryBrowser
          mountId={value.mountId}
          mount={selectedMount}
          onSelect={path => onChange({ ...value, path })}
          onClose={() => setBrowsing(false)}
          initialPath={value.path}
        />
      )}
    </div>
  )
}
