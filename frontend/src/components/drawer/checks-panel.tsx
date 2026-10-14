// SPDX-License-Identifier: AGPL-3.0-or-later
import { useState } from 'react'
import { t } from '../../i18n'
import { Icon } from '../icons'

export type CheckResult = { id: string; ok: boolean; level: string; got?: string; want?: string }

/** The consistency checks of the last launch: a one-line summary that opens into each condition. */
export function ChecksPanel({ checks }: { checks: CheckResult[] }) {
  const [open, setOpen] = useState(false)
  if (checks.length === 0) {
    return (
      <section className="dgroup" aria-label={t.checks.title}>
        <h3>{t.checks.title}</h3>
        <p className="hint">{t.checks.never}</p>
      </section>
    )
  }
  const bad = checks.filter((c) => !c.ok)
  const worst = bad.some((c) => c.level === 'danger') ? 'danger' : bad.length ? 'warn' : 'ok'
  return (
    <section className="dgroup" aria-label={t.checks.title}>
      <h3>{t.checks.title}</h3>
      <button type="button" className="disclosure checksummary" aria-expanded={open} onClick={() => setOpen(!open)}>
        <Icon name="chevron" className="icon sm" />
        <span className={`dotmark ${worst}`} aria-hidden="true" />
        {bad.length ? t.checks.someFailed(bad.length, checks.length) : t.checks.allPassed(checks.length)}
      </button>
      {open && (
        <ul className="checklist">
          {checks.map((c) => (
            <li key={c.id} className={c.ok ? 'ok' : c.level}>
              <Icon name={c.ok ? 'check' : 'alert'} className="icon sm" />
              <div>
                <b>{t.checks.name[c.id] ?? c.id}</b>
                {!c.ok && <p>{t.checks.text[c.id] ?? c.id}</p>}
                {!c.ok && (c.got || c.want) && <small className="mono">{t.checks.got}: {c.got || '–'} · {t.checks.want}: {c.want}</small>}
              </div>
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
