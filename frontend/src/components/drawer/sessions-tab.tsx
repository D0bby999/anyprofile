// SPDX-License-Identifier: AGPL-3.0-or-later
import { useEffect, useState } from 'react'
import { Api, onDataChanged, store } from '../../api'
import { t } from '../../i18n'
import { relativeTime, sinceSpan } from '../../lib/format'
import { countryName, flagText } from '../../lib/flags'

/** Notes for one session, compared with the next older session that measured an exit. */
function notesFor(list: store.Session[], i: number): { text: string; tone: string }[] {
  const s = list[i]
  if (s.exitError) return [{ text: flagText({ id: 'exit_failed', level: 'warn', params: {} }).label, tone: 'warn' }]
  const prev = list.slice(i + 1).find((x) => x.exitIp && x.proxyId === s.proxyId)
  if (!s.exitIp || !prev) return []
  if (prev.exitCountry !== s.exitCountry) return [{ text: t.flags.country_changed as string, tone: 'danger' }]
  if (prev.exitIp !== s.exitIp) return [{ text: t.flags.ip_changed as string, tone: '' }]
  return []
}

/** The profile's launches, newest first. */
export function SessionsTab({ profileId }: { profileId: string }) {
  const [list, setList] = useState<store.Session[] | null>(null)
  useEffect(() => {
    const load = () => Api.Sessions(profileId).then((s) => setList(s ?? [])).catch(() => setList([]))
    load()
    return onDataChanged(['profiles'], load)
  }, [profileId])

  if (list === null) return <p className="hint">{t.common.loading}</p>
  if (list.length === 0) return <p className="sessions-empty">{t.sessions.empty}</p>
  return (
    <table className="table sessions">
      <thead>
        <tr><th>{t.sessions.opened}</th><th>{t.sessions.length}</th><th>{t.sessions.exitIp}</th><th>{t.sessions.notes}</th></tr>
      </thead>
      <tbody>
        {list.map((s, i) => (
          <tr key={s.id}>
            <td title={new Date(s.startedAt).toLocaleString()}>{relativeTime(s.startedAt)}</td>
            <td className="nowrap">{s.stoppedAt ? spanBetween(s.startedAt, s.stoppedAt) : t.sessions.running}</td>
            <td><span className="mono selectable">{s.exitIp || '–'}</span>{s.exitCountry && <small title={countryName(s.exitCountry)}> {s.exitCountry}</small>}</td>
            <td>{notesFor(list, i).map((n) => <span key={n.text} className={`chip ${n.tone}`}>{n.text}</span>)}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}

function spanBetween(from: string, to: string): string {
  return sinceSpan(from, new Date(to))
}
