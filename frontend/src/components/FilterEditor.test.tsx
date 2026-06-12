import { render, screen, fireEvent } from '@testing-library/react'
import { FilterEditor } from './FilterEditor'
import type { FilterRuleset } from '../api/types'

const empty: FilterRuleset = { exclude_hidden: false, rules: [] }

function renderWith(value: FilterRuleset = empty) {
  const onChange = vi.fn()
  const utils = render(<FilterEditor value={value} onChange={onChange} />)
  return { ...utils, onChange }
}

describe('FilterEditor', () => {
  it('renders empty-state hint when there are no rules', () => {
    renderWith()
    expect(screen.getByText(/No rules/i)).toBeInTheDocument()
  })

  it('toggling exclude_hidden emits an onChange with the new value', () => {
    const { onChange } = renderWith()
    fireEvent.click(screen.getByLabelText(/Exclude hidden files/i, { selector: 'input' }))
    expect(onChange).toHaveBeenCalledWith({ exclude_hidden: true, rules: [] })
  })

  it('Add rule emits an onChange with one default glob rule', () => {
    const { onChange } = renderWith()
    fireEvent.click(screen.getByText(/Add rule/i))
    expect(onChange).toHaveBeenCalledTimes(1)
    const next = onChange.mock.calls[0]![0]
    expect(next.rules).toHaveLength(1)
    expect(next.rules[0].type).toBe('glob')
    expect(next.rules[0].action).toBe('exclude')
  })

  it('renders the right fields per rule type', () => {
    const ruleset: FilterRuleset = {
      exclude_hidden: false,
      rules: [
        { type: 'glob', action: 'exclude', pattern: '**/*.log' },
        { type: 'size', action: 'exclude', size_above_bytes: 1024 },
        { type: 'modified', action: 'exclude', modified_before_days_ago: 30 },
      ],
    }
    renderWith(ruleset)
    expect(screen.getByLabelText('Glob pattern')).toHaveValue('**/*.log')
    expect(screen.getByLabelText('Size above (bytes)')).toHaveValue(1024)
    expect(screen.getByLabelText('Modified before (days)')).toHaveValue(30)
  })

  // Changing rule type must reset the previous type's fields, or the server
  // will reject the resulting rule (e.g. a "size" rule that still carries a
  // stale `pattern` from when it was a glob is structurally invalid).
  it('changing rule type discards stale fields from the previous type', () => {
    const ruleset: FilterRuleset = {
      exclude_hidden: false,
      rules: [{ type: 'glob', action: 'exclude', pattern: '**/*.log' }],
    }
    const { onChange } = renderWith(ruleset)
    fireEvent.change(screen.getByLabelText('Rule type'), { target: { value: 'size' } })
    expect(onChange).toHaveBeenCalledTimes(1)
    const next = onChange.mock.calls[0]![0]
    expect(next.rules[0]).toEqual({ type: 'size', action: 'exclude' })
    expect(next.rules[0].pattern).toBeUndefined()
  })

  it('Remove drops the rule from the list', () => {
    const ruleset: FilterRuleset = {
      exclude_hidden: false,
      rules: [
        { type: 'glob', action: 'exclude', pattern: 'a' },
        { type: 'glob', action: 'exclude', pattern: 'b' },
      ],
    }
    const { onChange } = renderWith(ruleset)
    const removeButtons = screen.getAllByLabelText('Remove rule')
    fireEvent.click(removeButtons[0]!)
    expect(onChange).toHaveBeenCalledTimes(1)
    const next = onChange.mock.calls[0]![0]
    expect(next.rules).toHaveLength(1)
    expect(next.rules[0].pattern).toBe('b')
  })

  it('editing a field emits onChange with the patched rule', () => {
    const ruleset: FilterRuleset = {
      exclude_hidden: false,
      rules: [{ type: 'glob', action: 'exclude', pattern: 'a' }],
    }
    const { onChange } = renderWith(ruleset)
    fireEvent.change(screen.getByLabelText('Glob pattern'), { target: { value: '**/*.tmp' } })
    expect(onChange).toHaveBeenCalledWith({
      exclude_hidden: false,
      rules: [{ type: 'glob', action: 'exclude', pattern: '**/*.tmp' }],
    })
  })

  it('clearing a number field stores undefined, not 0', () => {
    // 0 would be a falsy-meaningful value (matches "no bound") for the server,
    // but an empty input should reset to "no constraint", not "constraint = 0".
    const ruleset: FilterRuleset = {
      exclude_hidden: false,
      rules: [{ type: 'size', action: 'exclude', size_above_bytes: 1024 }],
    }
    const { onChange } = renderWith(ruleset)
    fireEvent.change(screen.getByLabelText('Size above (bytes)'), { target: { value: '' } })
    const next = onChange.mock.calls[0]![0]
    expect(next.rules[0].size_above_bytes).toBeUndefined()
  })
})
