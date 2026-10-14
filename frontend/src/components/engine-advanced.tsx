// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useId, useState } from 'react'
import { Api, main } from '../api'
import { t } from '../i18n'
import { Icon } from './icons'

type Props = {
  /** "" means the default engine. */
  value: string
  onChange: (id: string) => void
}

/** A collapsed "Advanced" area with the engine choice; closed, the default engine is used. */
export function EngineAdvanced({ value, onChange }: Props) {
  const [open, setOpen] = useState(false)
  const [engines, setEngines] = useState<main.EngineInfo[]>([])
  const id = useId()
  useEffect(() => { Api.Engines().then((e) => setEngines(e ?? [])).catch(() => {}) }, [])
  const current = value || engines.find((e) => e.default)?.engine || ''

  return (
    <div className="advanced">
      <button type="button" className="disclosure" aria-expanded={open} aria-controls={id}
        onClick={() => { if (open) onChange(''); setOpen(!open) }}>
        <Icon name="chevron" className="icon sm" />{t.engines.advanced}
      </button>
      {open && (
        <label className="field" id={id}>
          <span className="label">{t.engines.engineLabel}</span>
          <select className="input select" value={current} onChange={(e) => onChange(e.target.value)}>
            {engines.map((e) => (
              <option key={e.engine} value={e.engine} disabled={!e.installed}>
                {(t.engines.names[e.engine] ?? e.engine) + (e.installed ? '' : ` ${t.engines.notInstalledSuffix}`)}
              </option>
            ))}
          </select>
          <span className="hint">{t.engines.engineHint}</span>
        </label>
      )}
    </div>
  )
}
