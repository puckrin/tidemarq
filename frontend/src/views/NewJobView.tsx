import { useState, useEffect } from 'react'
import { Check, ArrowLeft, ArrowRight } from 'lucide-react'
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query'
import { createJob, updateJob, getJob } from '../api/client'
import { Button } from '../components/Button'
import { useToast } from '../components/Toast'
import type { Job } from '../api/types'
import type { View } from '../components/Sidebar'

interface Props {
  onNav: (v: View, id?: number) => void
  editJobId?: number   // set when editing an existing job
}

type Step = 1 | 2 | 3 | 4 | 5

interface FormState {
  name: string
  source_path: string
  destination_path: string
  mode: Job['mode']
  conflict_strategy: Job['conflict_strategy']
  watch_enabled: boolean
  cron_schedule: string
  bandwidth_limit_kb: number
}

const INIT: FormState = {
  name: '',
  source_path: '',
  destination_path: '',
  mode: 'one-way-backup',
  conflict_strategy: 'ask-user',
  watch_enabled: true,
  cron_schedule: '',
  bandwidth_limit_kb: 0,
}

function jobToForm(j: Job): FormState {
  return {
    name:               j.name,
    source_path:        j.source_path,
    destination_path:   j.destination_path,
    mode:               j.mode,
    conflict_strategy:  j.conflict_strategy,
    watch_enabled:      j.watch_enabled,
    cron_schedule:      j.cron_schedule ?? '',
    bandwidth_limit_kb: j.bandwidth_limit_kb,
  }
}

const MODE_OPTIONS: { value: Job['mode']; title: string; desc: string }[] = [
  { value: 'one-way-backup', title: 'One-way backup',  desc: 'Source changes copy to destination. Deletions on source are not replicated — files remain on destination.' },
  { value: 'one-way-mirror', title: 'One-way mirror',  desc: 'Destination becomes an exact mirror of source. Deletions on source are replicated to destination.' },
  { value: 'two-way',        title: 'Two-way',         desc: 'Changes on either side propagate to the other. Conflicts are detected and queued for resolution.' },
]

const STRATEGY_OPTIONS: { value: Job['conflict_strategy']; label: string }[] = [
  { value: 'newest-wins',      label: 'Newest wins — keep the more recently modified file' },
  { value: 'largest-wins',     label: 'Largest wins — keep the larger file' },
  { value: 'source-wins',      label: 'Source wins — source always takes precedence' },
  { value: 'destination-wins', label: 'Destination wins — destination always takes precedence' },
  { value: 'ask-user',         label: 'Ask user — queue conflict for manual resolution' },
]

function StepIndicator({ step, current }: { step: number; current: Step }) {
  const done   = step < current
  const active = step === current
  const labels = ['Source & Name', 'Destination', 'Mode', 'Schedule & Bandwidth', 'Review']
  return (
    <div className={`step${active ? ' active' : done ? ' done' : ''}`}>
      <div className="step-num">{done ? <Check size={12}/> : step}</div>
      <span className="step-lbl">{labels[step - 1]}</span>
    </div>
  )
}

export function NewJobView({ onNav, editJobId }: Props) {
  const qc    = useQueryClient()
  const toast = useToast()
  const isEdit = editJobId != null

  const [step, setStep]       = useState<Step>(1)
  const [form, setForm]       = useState<FormState>(INIT)
  const [ready, setReady]     = useState(!isEdit)   // false until existing job loaded

  // Load existing job when editing
  const { data: existingJob } = useQuery({
    queryKey: ['job', editJobId],
    queryFn:  () => getJob(editJobId!),
    enabled:  isEdit,
    staleTime: Infinity,
  })

  useEffect(() => {
    if (existingJob && !ready) {
      setForm(jobToForm(existingJob))
      setReady(true)
    }
  }, [existingJob, ready])

  const set = <K extends keyof FormState>(k: K, v: FormState[K]) =>
    setForm(f => ({ ...f, [k]: v }))

  const save = useMutation({
    mutationFn: () => isEdit
      ? updateJob(editJobId!, {
          name:               form.name,
          source_path:        form.source_path,
          destination_path:   form.destination_path,
          mode:               form.mode,
          conflict_strategy:  form.conflict_strategy,
          watch_enabled:      form.watch_enabled,
          cron_schedule:      form.cron_schedule,
          bandwidth_limit_kb: form.bandwidth_limit_kb,
        })
      : createJob({
          name:               form.name,
          source_path:        form.source_path,
          destination_path:   form.destination_path,
          mode:               form.mode,
          conflict_strategy:  form.conflict_strategy,
          watch_enabled:      form.watch_enabled,
          cron_schedule:      form.cron_schedule,
          bandwidth_limit_kb: form.bandwidth_limit_kb,
        }),
    onSuccess: (job) => {
      qc.invalidateQueries({ queryKey: ['jobs'] })
      qc.invalidateQueries({ queryKey: ['job', job.id] })
      toast(isEdit ? `Job "${job.name}" updated.` : `Job "${job.name}" created.`, 'ok')
      onNav('job-detail', job.id)
    },
    onError: () => toast(isEdit ? 'Failed to update job.' : 'Failed to create job.', 'err'),
  })

  if (isEdit && !ready) {
    return <div className="text3" style={{ padding: 24 }}>Loading…</div>
  }

  const stepRow = (
    <div className="steps">
      {[1, 2, 3, 4, 5].map((s, i) => (
        <React.Fragment key={s}>
          <StepIndicator step={s} current={step} />
          {i < 4 && <div className={`step-line${s < step ? ' done' : ''}`} />}
        </React.Fragment>
      ))}
    </div>
  )

  const navRow = (
    <div className="flex gap8" style={{ justifyContent: 'flex-end', marginTop: 16 }}>
      <Button variant="ghost" onClick={() => isEdit ? onNav('job-detail', editJobId) : onNav('jobs')}>
        Cancel
      </Button>
      {step > 1 && (
        <Button variant="secondary" onClick={() => setStep(s => (s - 1) as Step)}>
          <ArrowLeft size={14}/> Back
        </Button>
      )}
      {step < 5
        ? <Button variant="primary" onClick={() => setStep(s => (s + 1) as Step)}>
            Next <ArrowRight size={14}/>
          </Button>
        : <Button variant="primary" onClick={() => save.mutate()} disabled={save.isPending}>
            {save.isPending
              ? (isEdit ? 'Saving…' : 'Creating…')
              : <><Check size={14}/> {isEdit ? 'Save changes' : 'Create job'}</>}
          </Button>
      }
    </div>
  )

  return (
    <div>
      <div className="bc">
        <a onClick={() => onNav('jobs')}>Sync Jobs</a>
        <span className="bc-sep">/</span>
        {isEdit && (
          <>
            <a onClick={() => onNav('job-detail', editJobId)}>{form.name || 'Job'}</a>
            <span className="bc-sep">/</span>
          </>
        )}
        <span>{isEdit ? 'Edit' : 'New Job'}</span>
      </div>
      <div className="page-title mb24">{isEdit ? 'Edit Sync Job' : 'Create Sync Job'}</div>

      {stepRow}

      <div style={{ maxWidth: 720 }}>

        {/* Step 1 — Name + Source */}
        {step === 1 && (
          <div className="card mb16">
            <div className="card-title mb16">Step 1 — Name &amp; Source</div>
            <div className="fg">
              <label className="fl">Job name</label>
              <input
                className="fi"
                placeholder="e.g. Documents → NAS Backup"
                value={form.name}
                onChange={e => set('name', e.target.value)}
              />
            </div>
            <div className="fg" style={{ marginBottom: 0 }}>
              <label className="fl">Source path</label>
              <input
                className="fi mono"
                style={{ fontSize: 13 }}
                placeholder="/home/user/Documents"
                value={form.source_path}
                onChange={e => set('source_path', e.target.value)}
              />
            </div>
          </div>
        )}

        {/* Step 2 — Destination */}
        {step === 2 && (
          <>
            {form.source_path && (
              <div className="card mb16">
                <div className="card-title mb16">
                  Step 1 — Source{' '}
                  <span className="badge b-synced" style={{ marginLeft: 8 }}>
                    <Check size={10}/> Selected
                  </span>
                </div>
                <div className="mono fs12 text2" style={{ padding: '10px 12px', background: 'var(--input-bg)', borderRadius: 'var(--radius)', border: '1px solid var(--input-border)' }}>
                  {form.source_path}
                </div>
              </div>
            )}
            <div className="card mb16">
              <div className="card-title mb16">Step 2 — Destination</div>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">Destination path</label>
                <input
                  className="fi mono"
                  style={{ fontSize: 13 }}
                  placeholder="/mnt/backup or \\server\share"
                  value={form.destination_path}
                  onChange={e => set('destination_path', e.target.value)}
                />
              </div>
            </div>
          </>
        )}

        {/* Step 3 — Mode */}
        {step === 3 && (
          <div className="card mb16">
            <div className="card-title mb16">Step 3 — Sync Mode</div>
            <div className="mode-cards">
              {MODE_OPTIONS.map(m => (
                <div
                  key={m.value}
                  className={`mode-card${form.mode === m.value ? ' sel' : ''}`}
                  onClick={() => set('mode', m.value)}
                >
                  <div className="mc-title">{m.title}</div>
                  <div className="mc-desc">{m.desc}</div>
                </div>
              ))}
            </div>
            <div className="fg" style={{ marginBottom: 0, marginTop: 8 }}>
              <label className="fl">Conflict strategy</label>
              <select
                className="fs"
                value={form.conflict_strategy}
                onChange={e => set('conflict_strategy', e.target.value as Job['conflict_strategy'])}
              >
                {STRATEGY_OPTIONS.map(o => (
                  <option key={o.value} value={o.value}>{o.label}</option>
                ))}
              </select>
              <div className="fhint">How to handle files modified on both sides simultaneously.</div>
            </div>
          </div>
        )}

        {/* Step 4 — Schedule & Bandwidth */}
        {step === 4 && (
          <div className="card mb16">
            <div className="card-title mb16">Step 4 — Schedule &amp; Bandwidth</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div className="flex gap12" style={{ alignItems: 'flex-start' }}>
                <label className="toggle" style={{ marginTop: 3 }}>
                  <input
                    type="checkbox"
                    checked={form.watch_enabled}
                    onChange={e => set('watch_enabled', e.target.checked)}
                  />
                  <span className="tog-sl"/>
                </label>
                <div>
                  <div style={{ fontSize: 13, fontWeight: 500 }}>Filesystem watch</div>
                  <div className="fs12 text2">Trigger sync immediately when files change on the source</div>
                </div>
              </div>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">Cron schedule (optional)</label>
                <input
                  className="fi mono"
                  style={{ maxWidth: 200, fontSize: 13 }}
                  placeholder="0 2 * * *"
                  value={form.cron_schedule}
                  onChange={e => set('cron_schedule', e.target.value)}
                />
                <div className="fhint">
                  Leave blank to disable scheduled runs. Example: <code>0 2 * * *</code> = daily at 02:00
                </div>
              </div>
              <div className="fg" style={{ marginBottom: 0 }}>
                <label className="fl">Bandwidth limit (KB/s, 0 = unlimited)</label>
                <input
                  className="fi"
                  style={{ maxWidth: 150 }}
                  type="number"
                  min={0}
                  value={form.bandwidth_limit_kb}
                  onChange={e => set('bandwidth_limit_kb', Number(e.target.value))}
                />
              </div>
            </div>
          </div>
        )}

        {/* Step 5 — Review */}
        {step === 5 && (
          <div className="card mb16">
            <div className="card-title mb16">Step 5 — Review</div>
            {save.isError && (
              <div style={{ color: 'var(--coral-light)', marginBottom: 16, fontSize: 13 }}>
                {isEdit ? 'Failed to save changes.' : 'Failed to create job.'} Please check your settings and try again.
              </div>
            )}
            <div style={{ display: 'flex', flexDirection: 'column', gap: 12, fontSize: 13 }}>
              {([
                ['Name',             form.name || '—'],
                ['Source',           <span className="mono fs12">{form.source_path || '—'}</span>],
                ['Destination',      <span className="mono fs12">{form.destination_path || '—'}</span>],
                ['Mode',             form.mode.replace(/-/g, ' ')],
                ['Conflict strategy',form.conflict_strategy.replace(/-/g, ' ')],
                ['FS watch',         form.watch_enabled ? 'Enabled' : 'Disabled'],
                ['Cron schedule',    form.cron_schedule || 'None'],
                ['Bandwidth limit',  form.bandwidth_limit_kb > 0 ? `${form.bandwidth_limit_kb} KB/s` : 'Unlimited'],
              ] as [string, React.ReactNode][]).map(([label, val]) => (
                <div key={label} className="flex gap8">
                  <span className="text3 fw5" style={{ minWidth: 160 }}>{label}</span>
                  <span>{val}</span>
                </div>
              ))}
            </div>
          </div>
        )}

        {navRow}
      </div>
    </div>
  )
}

import React from 'react'
