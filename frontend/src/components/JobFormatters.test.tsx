import { render, screen } from '@testing-library/react'
import { StatusBadge, ModePill } from './JobFormatters'
import type { Job } from '../api/types'

describe('StatusBadge', () => {
  const cases: { status: Job['status']; label: string }[] = [
    { status: 'running',  label: 'Running'  },
    { status: 'idle',     label: 'Synced'   },
    { status: 'paused',   label: 'Stopped'  },
    { status: 'error',    label: 'Error'    },
    { status: 'disabled', label: 'Disabled' },
  ]

  cases.forEach(({ status, label }) => {
    it(`renders "${label}" for status="${status}"`, () => {
      render(<StatusBadge status={status} />)
      expect(screen.getByText(label)).toBeInTheDocument()
    })
  })
})

describe('ModePill', () => {
  const cases: { mode: Job['mode']; label: string }[] = [
    { mode: 'one-way-backup', label: 'One-way backup' },
    { mode: 'one-way-mirror', label: 'One-way mirror' },
    { mode: 'two-way',        label: 'Two-way'        },
  ]

  cases.forEach(({ mode, label }) => {
    it(`renders "${label}" for mode="${mode}"`, () => {
      render(<ModePill mode={mode} />)
      expect(screen.getByText(label)).toBeInTheDocument()
    })
  })
})
