import type { Job, Conflict, FileVersion, QuarantineEntry, User, HealthResponse, Mount, MountInput, AuditEntry, BrowseResponse, AppSettings } from './types'

const BASE = '/api/v1'

export class ApiError extends Error {
  constructor(public code: string, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const token = localStorage.getItem('token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(init.headers as Record<string, string>),
  }
  if (token) headers['Authorization'] = `Bearer ${token}`

  const res = await fetch(path, { ...init, headers })

  if (res.status === 401) {
    localStorage.removeItem('token')
    window.dispatchEvent(new Event('auth:expired'))
    throw new ApiError('unauthorized', 'Session expired')
  }

  if (!res.ok) {
    let code = 'unknown'
    let message = res.statusText
    try {
      const body = await res.json()
      code = body.code ?? code
      message = body.error ?? message
    } catch { /* ignore */ }
    throw new ApiError(code, message)
  }

  if (res.status === 204) return undefined as T
  return res.json()
}

// Auth
export const login = (username: string, password: string) =>
  request<{ token: string }>(`${BASE}/auth/login`, {
    method: 'POST',
    body: JSON.stringify({ username, password }),
  })

export const getWsToken = () =>
  request<{ token: string }>(`${BASE}/auth/ws-token`)

// Health
export const getHealth = () => request<HealthResponse>('/health')

// Jobs
export const listJobs = () => request<Job[]>(`${BASE}/jobs`)
export const getJob = (id: number) => request<Job>(`${BASE}/jobs/${id}`)
export const createJob = (data: Partial<Job>) =>
  request<Job>(`${BASE}/jobs`, { method: 'POST', body: JSON.stringify(data) })
export const updateJob = (id: number, data: Partial<Job>) =>
  request<Job>(`${BASE}/jobs/${id}`, { method: 'PUT', body: JSON.stringify(data) })
export const deleteJob = (id: number) =>
  request<void>(`${BASE}/jobs/${id}`, { method: 'DELETE' })
export const runJob = (id: number) =>
  request<void>(`${BASE}/jobs/${id}/run`, { method: 'POST' })
export const pauseJob = (id: number) =>
  request<void>(`${BASE}/jobs/${id}/pause`, { method: 'POST' })
export const resumeJob = (id: number) =>
  request<void>(`${BASE}/jobs/${id}/resume`, { method: 'POST' })

// Conflicts
export const listConflicts = (jobId?: number) =>
  request<Conflict[]>(`${BASE}/conflicts${jobId ? `?job_id=${jobId}` : ''}`)
export const resolveConflict = (id: number, action: string) =>
  request<void>(`${BASE}/conflicts/${id}/resolve`, {
    method: 'POST',
    body: JSON.stringify({ action }),
  })
export const clearResolvedConflicts = (jobId?: number) =>
  request<void>(`${BASE}/conflicts/clear-resolved${jobId != null ? `?job_id=${jobId}` : ''}`, { method: 'POST' })

// Versions
export const listVersions = (jobId: number, path: string) =>
  request<FileVersion[]>(`${BASE}/versions?job_id=${jobId}&path=${encodeURIComponent(path)}`)
export const restoreVersion = (id: number) =>
  request<void>(`${BASE}/versions/${id}/restore`, { method: 'POST' })

// Quarantine
export const listQuarantine = (jobId?: number) =>
  request<QuarantineEntry[]>(`${BASE}/quarantine${jobId ? `?job_id=${jobId}` : ''}`)
export const listRemovedQuarantine = (jobId?: number) =>
  request<QuarantineEntry[]>(`${BASE}/quarantine/removed${jobId != null ? `?job_id=${jobId}` : ''}`)
export const restoreQuarantine = (id: number) =>
  request<void>(`${BASE}/quarantine/${id}/restore`, { method: 'POST' })
export const deleteQuarantineEntry = (id: number) =>
  request<void>(`${BASE}/quarantine/${id}`, { method: 'DELETE' })
export const clearRemovedQuarantine = (jobId?: number) =>
  request<void>(`${BASE}/quarantine/clear-removed${jobId != null ? `?job_id=${jobId}` : ''}`, { method: 'POST' })

// Mounts
export const listMounts = () => request<Mount[]>(`${BASE}/mounts`)
export const getMount = (id: number) => request<Mount>(`${BASE}/mounts/${id}`)
export const createMount = (data: MountInput) =>
  request<Mount>(`${BASE}/mounts`, { method: 'POST', body: JSON.stringify(data) })
export const updateMount = (id: number, data: MountInput) =>
  request<Mount>(`${BASE}/mounts/${id}`, { method: 'PUT', body: JSON.stringify(data) })
export const deleteMount = (id: number) =>
  request<void>(`${BASE}/mounts/${id}`, { method: 'DELETE' })
export const testMount = (id: number) =>
  request<{ ok: boolean; error?: string }>(`${BASE}/mounts/${id}/test`, { method: 'POST' })

// Settings
export const getSettings = () =>
  request<AppSettings>(`${BASE}/settings`)
export const updateSettings = (data: Pick<AppSettings, 'versions_to_keep' | 'quarantine_retention_days'>) =>
  request<AppSettings>(`${BASE}/settings`, { method: 'PUT', body: JSON.stringify(data) })

// Audit log
export const listAuditLog = (params?: { job_id?: number; event?: string; limit?: number; offset?: number }) => {
  const q = new URLSearchParams()
  if (params?.job_id != null) q.set('job_id', String(params.job_id))
  if (params?.event) q.set('event', params.event)
  if (params?.limit) q.set('limit', String(params.limit))
  if (params?.offset) q.set('offset', String(params.offset))
  const qs = q.toString()
  return request<AuditEntry[]>(`${BASE}/audit${qs ? `?${qs}` : ''}`)
}

// Browse (directory listing)
export const browseDir = (path: string, mountId?: number): Promise<BrowseResponse> => {
  const q = new URLSearchParams()
  q.set('path', path)
  if (mountId != null) q.set('mount_id', String(mountId))
  return request<BrowseResponse>(`${BASE}/browse?${q.toString()}`)
}

// Users
export const listUsers = () => request<User[]>(`${BASE}/users`)
export const createUser = (data: { username: string; password: string; role: string }) =>
  request<User>(`${BASE}/users`, { method: 'POST', body: JSON.stringify(data) })
export const updateUser = (id: number, data: Partial<User & { password?: string }>) =>
  request<User>(`${BASE}/users/${id}`, { method: 'PUT', body: JSON.stringify(data) })
export const deleteUser = (id: number) =>
  request<void>(`${BASE}/users/${id}`, { method: 'DELETE' })
