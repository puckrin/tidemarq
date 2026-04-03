export interface Job {
  id: number
  name: string
  source_path: string
  destination_path: string
  mode: 'one-way-backup' | 'one-way-mirror' | 'two-way'
  status: 'idle' | 'running' | 'paused' | 'error' | 'disabled'
  bandwidth_limit_kb: number
  conflict_strategy: 'ask-user' | 'newest-wins' | 'largest-wins' | 'source-wins' | 'destination-wins'
  cron_schedule: string
  watch_enabled: boolean
  last_run_at: string | null
  last_error: string | null
  created_at: string
  updated_at: string
}

export interface Conflict {
  id: number
  job_id: number
  rel_path: string
  src_sha256: string
  dest_sha256: string
  src_mod_time: string
  dest_mod_time: string
  src_size: number
  dest_size: number
  strategy: string
  status: 'pending' | 'resolved' | 'auto-resolved'
  resolution: string | null
  resolved_at: string | null
  created_at: string
}

export interface FileVersion {
  id: number
  job_id: number
  rel_path: string
  version_num: number
  stored_path: string
  sha256: string
  size_bytes: number
  mod_time: string
  created_at: string
}

export interface QuarantineEntry {
  id: number
  job_id: number
  rel_path: string
  quarantine_path: string
  sha256: string
  size_bytes: number
  deleted_at: string
  expires_at: string
}

export interface User {
  id: number
  username: string
  role: 'admin' | 'operator' | 'viewer'
  created_at: string
  updated_at: string
}

export interface HealthResponse {
  version: string
  database: string
  uptime: string
}

export interface WsEvent {
  job_id: number
  event: 'started' | 'progress' | 'paused' | 'completed' | 'error'
  files_done?: number
  files_total?: number
  bytes_done?: number
  rate_kbs?: number
  eta_secs?: number
  message?: string
}

export interface ApiError {
  error: string
  code: string
}
