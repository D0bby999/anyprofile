// SPDX-License-Identifier: AGPL-3.0-or-later
import { useId } from 'react'
import type { service } from '../api'
import { health } from '../lib/health'

const chip: Record<string, string> = { danger: 'danger', warn: 'warn', info: 'accent', ok: 'ok', none: '' }

/** One pill per profile; hover or keyboard focus lists every reason. */
export function HealthPill({ row, now }: { row: service.ProfileRow; now?: Date }) {
  const h = health(row, now)
  const id = useId()
  if (!h.label) return null
  return (
    <span className="health" tabIndex={0} aria-describedby={id}>
      <span className={`chip ${chip[h.level]}`}><span className="dot" /><span className="label">{h.label}</span></span>
      <span className="healthpop" role="tooltip" id={id}>
        {h.reasons.map((x) => <span key={x}>{x}</span>)}
      </span>
    </span>
  )
}
