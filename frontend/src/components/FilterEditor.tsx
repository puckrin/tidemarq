import { Plus, Trash2 } from 'lucide-react'
import { Button } from './Button'
import type { FilterRuleset, FilterRule, FilterRuleType, FilterAction } from '../api/types'

interface Props {
  value: FilterRuleset
  onChange: (next: FilterRuleset) => void
}

const RULE_TYPES: { value: FilterRuleType; label: string }[] = [
  { value: 'glob',      label: 'Glob pattern' },
  { value: 'extension', label: 'Extension' },
  { value: 'size',      label: 'Size' },
  { value: 'modified',  label: 'Modified date' },
]

function emptyRule(type: FilterRuleType): FilterRule {
  return { type, action: 'exclude' }
}

export function FilterEditor({ value, onChange }: Props) {
  const update = (rules: FilterRule[], excludeHidden = value.exclude_hidden) =>
    onChange({ exclude_hidden: excludeHidden, rules })

  const setRule = (i: number, patch: Partial<FilterRule>) => {
    const existing = value.rules[i]
    if (!existing) return
    const next = value.rules.slice()
    next[i] = { ...existing, ...patch }
    update(next)
  }

  // Whitelist fields per rule type so a stale `pattern` from a previous type
  // doesn't follow the user across to a size rule (and fail server validation).
  const setRuleType = (i: number, type: FilterRuleType) => {
    const cur = value.rules[i]
    if (!cur) return
    update(value.rules.map((r, j) => j === i ? { type, action: cur.action } : r))
  }

  const addRule = () => update([...value.rules, emptyRule('glob')])
  const removeRule = (i: number) => update(value.rules.filter((_, j) => j !== i))

  return (
    <div>
      <label className="flex gap8 items-center mb16" style={{ cursor: 'pointer' }}>
        <input
          type="checkbox"
          checked={value.exclude_hidden}
          onChange={e => update(value.rules, e.target.checked)}
        />
        <span className="text2">Exclude hidden files (dot-files)</span>
      </label>

      <div className="text3 mb8">
        Rules are evaluated top to bottom; the first match wins. Files matching no rule are included by default.
      </div>

      {value.rules.length === 0 && (
        <div
          className="text3 fs12"
          style={{
            padding: '12px 14px',
            border: '1px dashed var(--input-border)',
            borderRadius: 'var(--radius)',
            marginBottom: 12,
          }}
        >
          No rules — every file in the source tree will be considered.
        </div>
      )}

      <div className="flex" style={{ flexDirection: 'column', gap: 8, marginBottom: 12 }}>
        {value.rules.map((r, i) => (
          <FilterRuleRow
            key={i}
            rule={r}
            onTypeChange={t => setRuleType(i, t)}
            onActionChange={a => setRule(i, { action: a })}
            onPatchChange={patch => setRule(i, patch)}
            onRemove={() => removeRule(i)}
          />
        ))}
      </div>

      <Button variant="secondary" size="sm" onClick={addRule}>
        <Plus size={12}/> Add rule
      </Button>
    </div>
  )
}

function FilterRuleRow({
  rule, onTypeChange, onActionChange, onPatchChange, onRemove,
}: {
  rule: FilterRule
  onTypeChange: (t: FilterRuleType) => void
  onActionChange: (a: FilterAction) => void
  onPatchChange: (p: Partial<FilterRule>) => void
  onRemove: () => void
}) {
  return (
    <div
      className="flex gap8 items-center"
      style={{
        padding: '10px 12px',
        border: '1px solid var(--input-border)',
        borderRadius: 'var(--radius)',
        background: 'var(--input-bg)',
        flexWrap: 'wrap',
      }}
    >
      <select
        className="fi"
        style={{ width: 130 }}
        value={rule.action}
        onChange={e => onActionChange(e.target.value as FilterAction)}
        aria-label="Action"
      >
        <option value="exclude">Exclude</option>
        <option value="include">Include</option>
      </select>

      <select
        className="fi"
        style={{ width: 150 }}
        value={rule.type}
        onChange={e => onTypeChange(e.target.value as FilterRuleType)}
        aria-label="Rule type"
      >
        {RULE_TYPES.map(rt => (
          <option key={rt.value} value={rt.value}>{rt.label}</option>
        ))}
      </select>

      <RuleFields rule={rule} onPatchChange={onPatchChange} />

      <Button
        variant="ghost"
        size="sm"
        onClick={onRemove}
        style={{ marginLeft: 'auto' }}
        aria-label="Remove rule"
      >
        <Trash2 size={14}/>
      </Button>
    </div>
  )
}

function RuleFields({
  rule, onPatchChange,
}: {
  rule: FilterRule
  onPatchChange: (p: Partial<FilterRule>) => void
}) {
  switch (rule.type) {
    case 'glob':
      return (
        <input
          className="fi mono"
          style={{ flex: 1, minWidth: 180 }}
          placeholder="e.g. **/node_modules/**"
          value={rule.pattern ?? ''}
          onChange={e => onPatchChange({ pattern: e.target.value })}
          aria-label="Glob pattern"
        />
      )
    case 'extension':
      return (
        <input
          className="fi mono"
          style={{ flex: 1, minWidth: 120 }}
          placeholder=".log or log"
          value={rule.pattern ?? ''}
          onChange={e => onPatchChange({ pattern: e.target.value })}
          aria-label="Extension"
        />
      )
    case 'size':
      return (
        <>
          <input
            className="fi"
            type="number"
            min={0}
            style={{ width: 140 }}
            placeholder="Larger than (bytes)"
            value={rule.size_above_bytes ?? ''}
            onChange={e => onPatchChange({ size_above_bytes: e.target.value === '' ? undefined : Number(e.target.value) })}
            aria-label="Size above (bytes)"
          />
          <input
            className="fi"
            type="number"
            min={0}
            style={{ width: 140 }}
            placeholder="Smaller than (bytes)"
            value={rule.size_below_bytes ?? ''}
            onChange={e => onPatchChange({ size_below_bytes: e.target.value === '' ? undefined : Number(e.target.value) })}
            aria-label="Size below (bytes)"
          />
        </>
      )
    case 'modified':
      return (
        <>
          <input
            className="fi"
            type="number"
            min={0}
            style={{ width: 160 }}
            placeholder="Older than (days)"
            value={rule.modified_before_days_ago ?? ''}
            onChange={e => onPatchChange({ modified_before_days_ago: e.target.value === '' ? undefined : Number(e.target.value) })}
            aria-label="Modified before (days)"
          />
          <input
            className="fi"
            type="number"
            min={0}
            style={{ width: 160 }}
            placeholder="Within last (days)"
            value={rule.modified_within_days ?? ''}
            onChange={e => onPatchChange({ modified_within_days: e.target.value === '' ? undefined : Number(e.target.value) })}
            aria-label="Modified within (days)"
          />
        </>
      )
  }
}
