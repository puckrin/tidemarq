import { createContext, useContext, useRef, useCallback, useState, useEffect } from 'react'
import { useWsEvents } from '../hooks/useWsEvents'
import type { WsEvent } from '../api/types'

export interface FileActivity {
  relPath: string
  action: string
  ts: number
}

export interface JobProgressState {
  filesDone: number
  filesTotal: number
  filesSkipped: number
  bytesDone: number
  rateKBs: number
  etaSecs: number
  currentFile: string
  currentAction: string
  recentFiles: FileActivity[]
  lastEvent: string
}

const DEFAULT_STATE: JobProgressState = {
  filesDone: 0,
  filesTotal: 0,
  filesSkipped: 0,
  bytesDone: 0,
  rateKBs: 0,
  etaSecs: 0,
  currentFile: '',
  currentAction: '',
  recentFiles: [],
  lastEvent: '',
}

interface JobProgressContextValue {
  getProgress: (jobId: number) => JobProgressState
  subscribe: (cb: () => void) => () => void
}

const JobProgressContext = createContext<JobProgressContextValue>({
  getProgress: () => DEFAULT_STATE,
  subscribe: () => () => {},
})

export function JobProgressProvider({ children }: { children: React.ReactNode }) {
  const storeRef = useRef<Map<number, JobProgressState>>(new Map())
  const listenersRef = useRef<Set<() => void>>(new Set())

  const notify = useCallback(() => {
    listenersRef.current.forEach(cb => cb())
  }, [])

  const subscribe = useCallback((cb: () => void) => {
    listenersRef.current.add(cb)
    return () => { listenersRef.current.delete(cb) }
  }, [])

  const getProgress = useCallback((jobId: number): JobProgressState => {
    return storeRef.current.get(jobId) ?? DEFAULT_STATE
  }, [])

  useWsEvents(useCallback((e: WsEvent) => {
    const existing = storeRef.current.get(e.job_id) ?? { ...DEFAULT_STATE }
    let updated: JobProgressState

    if (e.event === 'started') {
      // New run — reset all progress for this job.
      updated = { ...DEFAULT_STATE, lastEvent: 'started' }
    } else if (e.event === 'completed' || e.event === 'error' || e.event === 'paused') {
      updated = { ...existing, lastEvent: e.event, currentFile: '', currentAction: '' }
    } else if (e.event === 'progress') {
      // Add completed file actions to the recent-files list.
      // Events that only tick the progress counter have no current_file set, so the
      // guard on e.current_file naturally excludes them without needing string checks.
      let newRecentFiles = existing.recentFiles
      if (e.current_file && (e.file_action === 'copied' || e.file_action === 'skipped' || e.file_action === 'removing')) {
        const entry: FileActivity = { relPath: e.current_file, action: e.file_action, ts: Date.now() }
        // Dedup: skip if the most recent entry is identical (guards against any
        // double WS delivery that may occur during reconnect race conditions).
        const last = existing.recentFiles[0]
        if (!last || last.relPath !== entry.relPath || last.action !== entry.action) {
          newRecentFiles = [entry, ...existing.recentFiles].slice(0, 50)
        }
      }
      updated = {
        filesDone:    e.files_done    ?? existing.filesDone,
        filesTotal:   e.files_total   ?? existing.filesTotal,
        filesSkipped: e.files_skipped ?? existing.filesSkipped,
        bytesDone:    e.bytes_done    ?? existing.bytesDone,
        rateKBs:      e.rate_kbs      ?? existing.rateKBs,
        etaSecs:      e.eta_secs      ?? existing.etaSecs,
        // currentFile / currentAction only track in-progress state:
        // 'scanning' (evaluating) and 'copying' (bytes moving).
        // Completion events ('copied', 'skipped', 'removing') only feed the
        // activity list — they do not disturb the indicator so it stays visible
        // showing the last known in-progress file until the next one starts.
        currentFile:   (e.file_action === 'scanning' || e.file_action === 'copying' || e.file_action === 'removing')
          ? (e.current_file ?? existing.currentFile)
          : existing.currentFile,
        currentAction: (e.file_action === 'scanning' || e.file_action === 'copying' || e.file_action === 'removing')
          ? e.file_action
          : existing.currentAction,
        recentFiles:  newRecentFiles,
        lastEvent:    'progress',
      }
    } else {
      return
    }

    storeRef.current.set(e.job_id, updated)
    notify()
  }, [notify]))

  return (
    <JobProgressContext.Provider value={{ getProgress, subscribe }}>
      {children}
    </JobProgressContext.Provider>
  )
}

/** Returns live progress state for a specific job. Re-renders on every WS update. */
export function useJobProgress(jobId: number): JobProgressState {
  const { getProgress, subscribe } = useContext(JobProgressContext)
  const [, forceUpdate] = useState(0)

  useEffect(() => {
    return subscribe(() => forceUpdate(n => n + 1))
  }, [subscribe])

  return getProgress(jobId)
}
