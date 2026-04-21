export interface AppSettings {
  versions_to_keep: number
  quarantine_retention_days: number
  audit_log_retention_days: number
  updated_at: string
}

export interface Job {
  id: number
  name: string
  source_path: string
  destination_path: string
  source_mount_id: number | null
  dest_mount_id: number | null
  mode: 'one-way-backup' | 'one-way-mirror' | 'two-way'
  status: 'idle' | 'running' | 'paused' | 'error' | 'disabled'
  bandwidth_limit_kb: number
  conflict_strategy: 'ask-user' | 'newest-wins' | 'largest-wins' | 'source-wins' | 'destination-wins'
  cron_schedule: string
  watch_enabled: boolean
  full_checksum: boolean
  hash_algo: 'sha256' | 'blake3'
  use_delta: boolean
  delta_block_size: number
  delta_min_bytes: number
  last_run_at: string | null
  last_error: string | null
  created_at: string
  updated_at: string
}

export interface BrowseEntry {
  name: string
  is_dir: boolean
  size?: number
}

export interface BrowseResponse {
  path: string
  entries: BrowseEntry[]
}

export interface Conflict {
  id: number
  job_id: number
  rel_path: string
  src_content_hash: string
  dest_content_hash: string
  src_hash_algo: string
  dest_hash_algo: string
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
  content_hash: string
  hash_algo: string
  size_bytes: number
  mod_time: string
  created_at: string
}

export interface QuarantineEntry {
  id: number
  job_id: number
  rel_path: string
  quarantine_path: string
  content_hash: string
  hash_algo: string
  size_bytes: number
  deleted_at: string
  expires_at: string
  /** "active" while in quarantine; "restored" or "deleted" after action taken. */
  status: 'active' | 'restored' | 'deleted'
  removed_at: string | null
}

export interface User {
  id: number
  username: string
  role: 'admin' | 'operator' | 'viewer'
  created_at: string
  updated_at: string
}

export interface Mount {
  id: number
  name: string
  type: 'sftp' | 'smb'
  host: string
  port: number
  username: string
  smb_share: string
  smb_domain: string
  sftp_host_key: string
  has_password: boolean
  has_ssh_key: boolean
  created_at: string
  updated_at: string
}

export interface MountInput {
  name: string
  type: 'sftp' | 'smb'
  host: string
  port: number
  username: string
  password?: string
  ssh_key?: string
  smb_share?: string
  smb_domain?: string
  sftp_host_key?: string
}

export interface AuditEntry {
  id: number
  job_id?: number
  job_name: string
  actor: string
  event: string
  message: string
  detail: string
  created_at: string
}

export interface HealthResponse {
  version: string
  database: string
  uptime: string
}

export interface WsEvent {
  job_id: number
  event: 'started' | 'progress' | 'paused' | 'completed' | 'error' | 'conflict_resolved'
  files_done?: number
  files_total?: number
  files_skipped?: number
  bytes_done?: number
  rate_kbs?: number
  eta_secs?: number
  current_file?: string
  file_action?: string  // "scanning" | "copying" | "copied" | "skipped" | "removing" | "present"
  message?: string
}

export interface ApiError {
  error: string
  code: string
}
